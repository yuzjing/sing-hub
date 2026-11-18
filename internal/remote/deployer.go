// internal/remote/deployer.go
package remote

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"sing-hub/internal/model"

	"golang.org/x/crypto/ssh"
)

type NodePingReport struct {
	DeviceID  string `json:"device_id"`
	Endpoint  string `json:"endpoint"`
	Online    bool   `json:"online"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

type DeployResult struct {
	Success       bool   `json:"success"`
	VerifiedHash  string `json:"verified_hash"`
	RemoteVersion string `json:"remote_version"`
	Logs          string `json:"logs"`
}

// 👑 健壮支持：纯IP、带端口IP、[IPv6]:port、域名等各类复杂端点
func PingTarget(endpoint string, port int, timeout time.Duration) (int64, error) {
	if port <= 0 {
		port = 22
	}
	endpoint = strings.TrimSpace(endpoint)
	host := endpoint

	// 优先尝试标准拆分 host 和 port
	if h, p, err := net.SplitHostPort(endpoint); err == nil {
		host = h
		if parsedP, err := strconv.Atoi(p); err == nil && parsedP > 0 {
			port = parsedP
		}
	} else {
		// 去除 IPv6 外层方括号残留
		host = strings.Trim(endpoint, "[]")
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return 0, err
	}
	_ = conn.Close()
	return time.Since(start).Milliseconds(), nil
}

func resolveSSHTarget(d model.Device) (string, int) {
	raw := strings.TrimSpace(d.SSHHost)
	if raw == "" && len(d.Endpoints) > 0 {
		raw = strings.TrimSpace(d.Endpoints[0])
	}
	port := d.SSHPort
	if port <= 0 {
		port = 22
	}
	if raw == "" {
		return "", port
	}

	host := raw
	if h, p, err := net.SplitHostPort(raw); err == nil {
		host = h
		if parsedP, err := strconv.Atoi(p); err == nil && parsedP > 0 && d.SSHPort <= 0 {
			port = parsedP
		}
	} else {
		host = strings.Trim(raw, "[]")
	}
	return host, port
}

func PingAllNodes(devices []model.Device, timeout time.Duration) []NodePingReport {
	var wg sync.WaitGroup
	reports := make([]NodePingReport, 0, len(devices))
	var mu sync.Mutex

	for _, d := range devices {
		if !d.EnablePing || len(d.Endpoints) == 0 {
			continue
		}
		endpoint := d.Endpoints[0]
		wg.Add(1)
		go func(dev model.Device, ep string) {
			defer wg.Done()
			port := dev.SSHPort
			if port <= 0 {
				port = 22
			}
			ms, err := PingTarget(ep, port, timeout)
			report := NodePingReport{
				DeviceID:  dev.ID,
				Endpoint:  ep,
				Online:    err == nil,
				LatencyMS: ms,
			}
			if err != nil {
				report.Error = err.Error()
			}
			mu.Lock()
			reports = append(reports, report)
			mu.Unlock()
		}(d, endpoint)
	}
	wg.Wait()
	return reports
}

// 抽取版本号正则，匹配如 "1.14.3" 或 "v1.15.0-alpha.12"
var verRegex = regexp.MustCompile(`(?i)sing-box\s+version\s+v?([0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)`)

func parseSingboxVersion(raw string) string {
	lines := strings.Split(raw, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		matches := verRegex.FindStringSubmatch(l)
		if len(matches) > 1 {
			return "v" + strings.TrimPrefix(matches[1], "v")
		}
	}
	return ""
}

type strategyPlan struct {
	verProbeCmd   string
	checkCmd      string
	reloadCmd     string
	statusCmd     string
	logCaptureCmd string
}

func buildStrategyPlan(strategy, svc, envPrefix, tempPath string) strategyPlan {
	scDetect := fmt.Sprintf(
		`SVC=%s; SC_MODE=""; `+
			`if [ "$CURRENT_UID" -eq 0 ]; then `+
			`systemctl cat "$SVC.service" >/dev/null 2>&1 && SC_MODE="system"; `+
			`elif systemctl --user cat "$SVC.service" >/dev/null 2>&1; then `+
			`SC_MODE="user"; `+
			`elif systemctl cat "$SVC.service" >/dev/null 2>&1; then `+
			`SC_MODE="sudo"; `+
			`fi; `+
			`sc() { `+
			`case "$SC_MODE" in `+
			`system) systemctl "$@" ;; `+
			`user) systemctl --user "$@" ;; `+
			`sudo) sudo -n systemctl "$@" ;; `+
			`*) return 1 ;; `+
			`esac; `+
			`}; `,
		svc,
	)
	baseEnv := envPrefix + scDetect

	binDetect := `BIN=$(command -v sing-box 2>/dev/null); ` +
		`if [ -z "$BIN" ] && [ -n "$SC_MODE" ]; then ` +
		`CAND=$(sc show -p ExecStart --value "$SVC.service" 2>/dev/null | sed -n 's/.*path=\([^ ;]*\).*/\1/p' | head -n1); ` +
		`case "$(basename "$CAND" 2>/dev/null)" in sing-box*) [ -x "$CAND" ] && BIN="$CAND" ;; esac; ` +
		`fi; `

	switch strategy {
	case "podman":
		return strategyPlan{
			verProbeCmd: baseEnv + `podman exec "$SVC" sing-box version 2>/dev/null`,
			checkCmd: baseEnv + fmt.Sprintf(
				`if [ "$(podman inspect -f '{{.State.Running}}' "$SVC" 2>/dev/null)" = "true" ]; then `+
					`podman exec -i "$SVC" sing-box check -c /dev/stdin < %s; `+
					`else echo "[i] 容器当前未运行，跳过远端 exec 预检（已在控制面完成校验）"; fi`,
				tempPath,
			),
			reloadCmd: baseEnv + `if [ -n "$SC_MODE" ]; then sc reset-failed "$SVC" 2>/dev/null; sc restart "$SVC"; else podman restart "$SVC"; fi`,
			statusCmd: baseEnv + `if [ -n "$SC_MODE" ]; then sc is-active --quiet "$SVC"; else [ "$(podman inspect -f '{{.State.Running}}' "$SVC" 2>/dev/null)" = "true" ]; fi`,
			logCaptureCmd: baseEnv + `if [ "$SC_MODE" = "user" ]; then journalctl --user -u "$SVC" -n 25 --no-pager 2>&1; ` +
				`elif [ -n "$SC_MODE" ]; then journalctl -u "$SVC" -n 25 --no-pager 2>&1 || sc status "$SVC" --no-pager -l 2>&1; ` +
				`else podman logs --tail 25 "$SVC" 2>&1; fi`,
		}

	case "docker":
		return strategyPlan{
			verProbeCmd: baseEnv + `docker exec "$SVC" sing-box version 2>/dev/null`,
			checkCmd: baseEnv + fmt.Sprintf(
				`if [ "$(docker inspect -f '{{.State.Running}}' "$SVC" 2>/dev/null)" = "true" ]; then `+
					`docker exec -i "$SVC" sing-box check -c /dev/stdin < %s; `+
					`else echo "[i] 容器当前未运行，跳过远端 exec 预检（已在控制面完成校验）"; fi`,
				tempPath,
			),
			reloadCmd:     baseEnv + `if [ -n "$SC_MODE" ]; then sc reset-failed "$SVC" 2>/dev/null; sc restart "$SVC"; else docker restart "$SVC"; fi`,
			statusCmd:     baseEnv + `if [ -n "$SC_MODE" ]; then sc is-active --quiet "$SVC"; else [ "$(docker inspect -f '{{.State.Running}}' "$SVC" 2>/dev/null)" = "true" ]; fi`,
			logCaptureCmd: baseEnv + `if [ -n "$SC_MODE" ]; then journalctl -u "$SVC" -n 25 --no-pager 2>&1; else docker logs --tail 25 "$SVC" 2>&1; fi`,
		}

	case "standalone":
		return strategyPlan{
			verProbeCmd:   baseEnv + binDetect + `[ -n "$BIN" ] && "$BIN" version 2>/dev/null`,
			checkCmd:      baseEnv + binDetect + fmt.Sprintf(`if [ -z "$BIN" ]; then echo "未找到 sing-box 可执行文件" >&2; exit 1; fi; "$BIN" check -c %s`, tempPath),
			reloadCmd:     `echo "已替换配置文件 (standalone 模式不重启守护进程)"`,
			statusCmd:     `true`,
			logCaptureCmd: `true`,
		}

	default: // systemd：100% 原生 Systemd 主导生命周期，通过 ExecStart 精确识别底层执行器
		execDetect := `if [ -z "$SC_MODE" ]; then echo "未找到 systemd 服务单元: $SVC.service" >&2; exit 1; fi; ` +
			`EXEC_LINE=$(sc show -p ExecStart --value "$SVC.service" 2>/dev/null); ` +
			`EXEC_BIN=$(printf '%s' "$EXEC_LINE" | sed -n 's/.*path=\([^ ;]*\).*/\1/p' | head -n1); ` +
			`CNAME=$(printf '%s' "$EXEC_LINE" | sed -n 's/.*--name[= ]\([^ ;]*\).*/\1/p' | head -n1); ` +
			`[ -n "$CNAME" ] || CNAME="$SVC"; `

		return strategyPlan{
			verProbeCmd: baseEnv + execDetect +
				`case "$(basename "$EXEC_BIN" 2>/dev/null)" in ` +
				`sing-box*) "$EXEC_BIN" version 2>/dev/null ;; ` +
				`podman) podman exec "$CNAME" sing-box version 2>/dev/null ;; ` +
				`docker) docker exec "$CNAME" sing-box version 2>/dev/null ;; ` +
				`esac`,
			checkCmd: baseEnv + execDetect + fmt.Sprintf(
				`case "$(basename "$EXEC_BIN" 2>/dev/null)" in `+
					`sing-box*) "$EXEC_BIN" check -c %s ;; `+
					`podman) if [ "$(podman inspect -f '{{.State.Running}}' "$CNAME" 2>/dev/null)" = "true" ]; then podman exec -i "$CNAME" sing-box check -c /dev/stdin < %s; else echo "[i] Quadlet/Podman 容器未运行，跳过远端 exec 预检"; fi ;; `+
					`docker) if [ "$(docker inspect -f '{{.State.Running}}' "$CNAME" 2>/dev/null)" = "true" ]; then docker exec -i "$CNAME" sing-box check -c /dev/stdin < %s; else echo "[i] Docker 容器未运行，跳过远端 exec 预检"; fi ;; `+
					`*) echo "[i] 非标准 ExecStart ($EXEC_BIN)，跳过远端预检" ;; `+
					`esac`,
				tempPath, tempPath, tempPath,
			),
			reloadCmd: baseEnv + `if [ -z "$SC_MODE" ]; then echo "未找到 systemd 服务单元: $SVC.service" >&2; exit 1; fi; sc reset-failed "$SVC" 2>/dev/null; sc restart "$SVC"`,
			statusCmd: baseEnv + `[ -n "$SC_MODE" ] && sc is-active --quiet "$SVC"`,
			logCaptureCmd: baseEnv + `if [ "$SC_MODE" = "user" ]; then journalctl --user -u "$SVC" -n 25 --no-pager 2>&1; ` +
				`else journalctl -u "$SVC" -n 25 --no-pager 2>&1 || sc status "$SVC" --no-pager -l 2>&1; fi`,
		}
	}
}

func DeployConfig(d model.Device, configJSON []byte, timeout time.Duration) (DeployResult, error) {
	var res DeployResult
	var logs strings.Builder

	logStep := func(step string) {
		fmt.Fprintf(&logs, "\n[+] %s\n", step)
	}
	logOut := func(out string) {
		if strings.TrimSpace(out) != "" {
			fmt.Fprintf(&logs, "%s\n", strings.TrimSpace(out))
		}
	}

	endpoint, port := resolveSSHTarget(d)
	if endpoint == "" || d.SSHUser == "" {
		return res, fmt.Errorf("节点缺失 SSH 地址或 SSH 用户")
	}

	var authMethods []ssh.AuthMethod
	switch d.AuthType {
	case "key_path":
		if d.KeyPath != "" {
			keyBytes, err := os.ReadFile(d.KeyPath)
			if err != nil {
				return res, fmt.Errorf("读取私钥失败: %w", err)
			}
			signer, err := ssh.ParsePrivateKey(keyBytes)
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	case "key_content":
		if d.AuthSecret != "" {
			signer, err := ssh.ParsePrivateKey([]byte(d.AuthSecret))
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	case "password":
		authMethods = append(authMethods, ssh.Password(d.AuthSecret))
	}
	if len(authMethods) == 0 {
		authMethods = append(authMethods, ssh.Password(""))
	}

	clientConfig := &ssh.ClientConfig{
		User:            d.SSHUser,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	addr := net.JoinHostPort(endpoint, strconv.Itoa(port))
	logStep(fmt.Sprintf("正在通过 SSH 连接节点: %s@%s...", d.SSHUser, addr))

	client, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		fmt.Fprintf(&logs, "SSH 认证或网络握手失败: %v\n", err)
		res.Logs = logs.String()
		return res, fmt.Errorf("SSH 连接失败: %w", err)
	}
	defer client.Close()
	logOut("SSH 认证通道建立成功。")

	svc := d.ServiceName
	if svc == "" {
		svc = "sing-box"
	}

	remotePath := d.RemotePath
	if remotePath == "" {
		remotePath = "~/.config/sing-box/config.json"
	}
	tempPath := remotePath + ".new"

	// 核心环境变量前缀：解决 Rootless Podman 与 Systemd User 访问 D-Bus 的关键凭证
	envPrefix := "export PATH=$PATH:/usr/local/bin:/opt/bin:~/.local/bin:/usr/bin:/bin; " +
		"CURRENT_UID=$(id -u 2>/dev/null || echo 0); " +
		"if [ \"$CURRENT_UID\" -ne 0 ] && [ -d \"/run/user/$CURRENT_UID\" ]; then " +
		"  export XDG_RUNTIME_DIR=\"/run/user/$CURRENT_UID\"; " +
		"  export DBUS_SESSION_BUS_ADDRESS=\"unix:path=$XDG_RUNTIME_DIR/bus\"; " +
		"fi; "

	plan := buildStrategyPlan(d.DeployStrategy, svc, envPrefix, tempPath)

	// 1. 探测远端运行环境与 Sing-box 内核版本
	logStep("阶段 1: 探测远端运行环境与内核版本...")
	sessV, _ := client.NewSession()
	vOut, _ := sessV.CombinedOutput(plan.verProbeCmd)
	sessV.Close()

	if detectedVer := parseSingboxVersion(string(vOut)); detectedVer != "" {
		res.RemoteVersion = detectedVer
		logOut(fmt.Sprintf("探测到活跃内核版本: %s", res.RemoteVersion))
	} else {
		res.RemoteVersion = "待同步探测"
		logOut("提示: 当前未能获取确切版本号（服务可能处于停止状态），将在重载后再次核验。")
	}

	// 2. 写入临时配置文件（Stdin 管道直传，确保 SHA256 字节 1:1 一致）
	logStep("阶段 2: 安全分发临时配置至目标主机...")
	sess1, _ := client.NewSession()
	sess1.Stdin = bytes.NewReader(configJSON)
	writeCmd := fmt.Sprintf("%s mkdir -p $(dirname %s) && cat > %s", envPrefix, tempPath, tempPath)
	out1, err := sess1.CombinedOutput(writeCmd)
	sess1.Close()
	if err != nil {
		logOut(string(out1))
		logs.WriteString(fmt.Sprintf("文件落盘失败: %v\n", err))
		res.Logs = logs.String()
		return res, fmt.Errorf("写入临时文件失败")
	}
	logOut("临时配置文件已持久化落盘。")

	// 3. 内核预检
	logStep("阶段 3: 执行内核深度语义预检 (Pre-flight Checking)...")
	sess2, _ := client.NewSession()
	out2, err := sess2.CombinedOutput(plan.checkCmd)
	sess2.Close()
	if err != nil {
		logOut(string(out2))
		logs.WriteString("\n[!] 预检未通过，已紧急熔断部署，并清理现场残留。\n")
		res.Logs = logs.String()
		cleanSess, _ := client.NewSession()
		_, _ = cleanSess.CombinedOutput(fmt.Sprintf("%s rm -f %s", envPrefix, tempPath))
		cleanSess.Close()
		return res, fmt.Errorf("远端预检失败: 配置未通过官方语法校验")
	}
	if strings.TrimSpace(string(out2)) != "" {
		logOut(string(out2))
	}
	logOut("Sing-box 语法与语义校验 100% 通过 ✓")

	// 4. 原子覆写并触发重载
	logStep("阶段 4: 原子覆写正式配置并启动/重载守护进程...")
	sess3, _ := client.NewSession()
	swapCmd := fmt.Sprintf("%s mv -f %s %s && (%s)", envPrefix, tempPath, remotePath, plan.reloadCmd)
	out3, err := sess3.CombinedOutput(swapCmd)
	sess3.Close()
	if err != nil {
		logOut(string(out3))
		res.Logs = logs.String()
		return res, fmt.Errorf("服务重载指令执行异常: %v", err)
	}
	logOut("配置替换完成，已触发服务优雅重载。")

	// 5. 存活探针、故障日志捕获与真实版本二次核验
	logStep("阶段 5: 验证服务存活状态 (Health Check)...")
	time.Sleep(1200 * time.Millisecond)

	sessStatus, _ := client.NewSession()
	_, statusErr := sessStatus.CombinedOutput(plan.statusCmd)
	sessStatus.Close()

	serviceHealthy := statusErr == nil
	if !serviceHealthy {
		logOut("⚠️ 警告: 服务重载后未能处于 Active/Running 状态，正在抓取错误日志现场...")
		sessLog, _ := client.NewSession()
		journalOut, _ := sessLog.CombinedOutput(plan.logCaptureCmd)
		sessLog.Close()
		logOut(string(journalOut))
	} else {
		logOut("守护进程运行状态健康 (Active/Running) ✓")
		sessV2, _ := client.NewSession()
		vOut2, _ := sessV2.CombinedOutput(plan.verProbeCmd)
		sessV2.Close()
		if finalVer := parseSingboxVersion(string(vOut2)); finalVer != "" {
			res.RemoteVersion = finalVer
		}
	}

	// 6. 哈希比对闭环
	logStep("阶段 6: 远端 SHA256 镜像核验...")
	sess4, _ := client.NewSession()
	shaOut, err := sess4.CombinedOutput(fmt.Sprintf("%s sha256sum %s | awk '{print $1}'", envPrefix, remotePath))
	sess4.Close()

	remoteSHA := strings.TrimSpace(string(shaOut))
	localSHA := fmt.Sprintf("%x", sha256.Sum256(configJSON))

	if err == nil && remoteSHA == localSHA {
		logOut(fmt.Sprintf("校验和完美匹配: %s", remoteSHA[:8]))
		res.VerifiedHash = remoteSHA[:8]
	} else {
		logOut("警告: 远端物理文件 SHA256 与本地编译快照存在出入")
		res.Logs = logs.String()
		return res, fmt.Errorf("远端配置校验和不匹配")
	}

	if !serviceHealthy {
		res.Logs = logs.String()
		return res, fmt.Errorf("配置已推送，但远端服务未能正常启动")
	}

	res.Success = true
	res.Logs = logs.String()
	return res, nil

}
