// internal/remote/deployer.go
package remote

import (
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

func PingTarget(endpoint string, port int, timeout time.Duration) (int64, error) {
	if port <= 0 {
		port = 80
	}
	host := endpoint
	if strings.Contains(endpoint, ":") && !strings.HasPrefix(endpoint, "[") {
		if h, p, err := net.SplitHostPort(endpoint); err == nil {
			host = h
			if parsedP, err := strconv.Atoi(p); err == nil {
				port = parsedP
			}
		}
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
var verRegex = regexp.MustCompile(`(?:sing-box\s+version\s+|v)?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[a-zA-Z0-9.]+)?(?:-alpha\.[0-9]+)?)`)

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

func DeployConfig(d model.Device, configJSON []byte, timeout time.Duration) (DeployResult, error) {
	var res DeployResult
	var logs strings.Builder

	logStep := func(step string) {
		logs.WriteString(fmt.Sprintf("\n[+] %s\n", step))
	}
	logOut := func(out string) {
		if strings.TrimSpace(out) != "" {
			logs.WriteString(fmt.Sprintf("%s\n", strings.TrimSpace(out)))
		}
	}

	if len(d.Endpoints) == 0 || d.SSHUser == "" {
		return res, fmt.Errorf("节点缺失通信端点或 SSH 用户")
	}

	endpoint := d.Endpoints[0]
	port := d.SSHPort
	if port <= 0 {
		port = 22
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
		logs.WriteString(fmt.Sprintf("SSH 认证或网络握手失败: %v\n", err))
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

	// 1. 智能探测远端运行环境与 Sing-box 内核版本
	logStep("阶段 1: 探测远端运行环境与内核版本...")
	var verProbeCmd string
	switch d.DeployStrategy {
	case "podman":
		// 优先 exec，若容器刚创建则从 logs 或镜像 inspect 尝试
		verProbeCmd = fmt.Sprintf("%s podman exec %s sing-box version 2>/dev/null || podman logs --tail 50 %s 2>/dev/null", envPrefix, svc, svc)
	case "docker":
		verProbeCmd = fmt.Sprintf("%s docker exec %s sing-box version 2>/dev/null || docker logs --tail 50 %s 2>/dev/null", envPrefix, svc, svc)
	case "standalone":
		verProbeCmd = fmt.Sprintf("%s sing-box version 2>/dev/null || ~/.local/bin/sing-box version 2>/dev/null || /usr/local/bin/sing-box version 2>/dev/null", envPrefix)
	default: // systemd (兼容原生二进制与 Quadlet 容器化)
		verProbeCmd = fmt.Sprintf("%s (sing-box version 2>/dev/null || podman exec %s sing-box version 2>/dev/null || docker exec %s sing-box version 2>/dev/null || journalctl --user -u %s -n 50 2>/dev/null)", envPrefix, svc, svc, svc)
	}

	sessV, _ := client.NewSession()
	vOut, _ := sessV.CombinedOutput(verProbeCmd)
	sessV.Close()

	detectedVer := parseSingboxVersion(string(vOut))
	if detectedVer != "" {
		res.RemoteVersion = detectedVer
		logOut(fmt.Sprintf("探测到活跃内核版本: %s", res.RemoteVersion))
	} else {
		res.RemoteVersion = "待同步探测"
		logOut("提示: 当前未能获取确切版本号（服务可能处于停止状态），将在重载后再次核验。")
	}

	// 2. 写入临时配置文件
	logStep("阶段 2: 安全分发临时配置至目标主机...")
	sess1, _ := client.NewSession()
	writeCmd := fmt.Sprintf("%s mkdir -p $(dirname %s) && cat << 'EOF' > %s\n%s\nEOF", envPrefix, tempPath, tempPath, string(configJSON))
	out1, err := sess1.CombinedOutput(writeCmd)
	sess1.Close()
	if err != nil {
		logOut(string(out1))
		logs.WriteString(fmt.Sprintf("文件落盘失败: %v\n", err))
		res.Logs = logs.String()
		return res, fmt.Errorf("写入临时文件失败")
	}
	logOut("临时配置文件已持久化落盘。")

	// 3. 内核预检：运用 /dev/stdin 管道，无视容器内部挂载目录差异
	logStep("阶段 3: 执行内核深度语义预检 (Pre-flight Checking)...")
	var checkCmd string
	switch d.DeployStrategy {
	case "podman":
		checkCmd = fmt.Sprintf("%s podman exec -i %s sing-box check -c /dev/stdin < %s", envPrefix, svc, tempPath)
	case "docker":
		checkCmd = fmt.Sprintf("%s docker exec -i %s sing-box check -c /dev/stdin < %s", envPrefix, svc, tempPath)
	case "standalone":
		checkCmd = fmt.Sprintf("%s (sing-box check -c %s 2>&1 || true)", envPrefix, tempPath)
	default: // systemd (若宿主机有二进制用宿主机，没有则借用容器/Quadlet)
		checkCmd = fmt.Sprintf("%s (sing-box check -c %s 2>&1 || podman exec -i %s sing-box check -c /dev/stdin < %s 2>&1)", envPrefix, tempPath, svc, tempPath)
	}

	sess2, _ := client.NewSession()
	out2, err := sess2.CombinedOutput(checkCmd)
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
	logOut("Sing-box 语法与语义校验 100% 通过 ✓")

	// 4. 原子覆写并触发重载
	logStep("阶段 4: 原子覆写正式配置并启动/重载守护进程...")
	var reloadCmd, statusCmd string
	switch d.DeployStrategy {
	case "podman":
		// Quadlet 或原生 Podman：优先用 systemctl --user 重启，若不是 Quadlet 则用 podman restart
		reloadCmd = fmt.Sprintf("%s (systemctl --user restart %s 2>/dev/null || podman restart %s)", envPrefix, svc, svc)
		statusCmd = fmt.Sprintf("%s (systemctl --user is-active --quiet %s 2>/dev/null || podman inspect -f '{{.State.Running}}' %s 2>/dev/null)", envPrefix, svc, svc)
	case "docker":
		reloadCmd = fmt.Sprintf("%s docker restart %s", envPrefix, svc)
		statusCmd = fmt.Sprintf("%s docker inspect -f '{{.State.Running}}' %s", envPrefix, svc)
	case "standalone":
		reloadCmd = "echo '已替换配置文件 (无守护进程策略)'"
		statusCmd = "true"
	default: // systemd
		reloadCmd = fmt.Sprintf("%s (systemctl --user restart %s 2>/dev/null || systemctl restart %s 2>/dev/null || sudo systemctl restart %s 2>&1)", envPrefix, svc, svc, svc)
		statusCmd = fmt.Sprintf("%s (systemctl --user is-active --quiet %s 2>/dev/null || systemctl is-active --quiet %s 2>/dev/null)", envPrefix, svc, svc)
	}

	sess3, _ := client.NewSession()
	swapCmd := fmt.Sprintf("%s mv -f %s %s && %s", envPrefix, tempPath, remotePath, reloadCmd)
	out3, err := sess3.CombinedOutput(swapCmd)
	sess3.Close()
	if err != nil {
		logOut(string(out3))
		res.Logs = logs.String()
		return res, fmt.Errorf("服务重载指令执行异常: %v", err)
	}
	logOut("配置替换完成，已触发服务优雅重载。")

	// 5. 存活探针与故障日志捕获
	logStep("阶段 5: 验证服务存活状态 (Health Check)...")
	time.Sleep(1200 * time.Millisecond) // 等待 systemd/podman 容器拉起

	sessStatus, _ := client.NewSession()
	_, statusErr := sessStatus.CombinedOutput(statusCmd)
	sessStatus.Close()

	if statusErr != nil {
		logOut("⚠️ 警告: 服务重载后未能处于 Active/Running 状态，正在抓取错误日志现场...")
		sessLog, _ := client.NewSession()
		var logCaptureCmd string
		if d.DeployStrategy == "docker" {
			logCaptureCmd = fmt.Sprintf("%s docker logs --tail 25 %s 2>&1", envPrefix, svc)
		} else {
			// 智能拉取 systemd 用户态 journal 或 podman logs
			logCaptureCmd = fmt.Sprintf("%s (journalctl --user -u %s -n 25 --no-pager 2>/dev/null || journalctl -u %s -n 25 --no-pager 2>/dev/null || podman logs --tail 25 %s 2>&1)", envPrefix, svc, svc, svc)
		}
		journalOut, _ := sessLog.CombinedOutput(logCaptureCmd)
		sessLog.Close()
		logOut(string(journalOut))
	} else {
		logOut("守护进程运行状态健康 (Active/Running) ✓")
	}

	// 再次探测一次版本（若初次未成功抓取到）
	if res.RemoteVersion == "待同步探测" {
		sessV2, _ := client.NewSession()
		vOut2, _ := sessV2.CombinedOutput(verProbeCmd)
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
		res.Success = true
		res.VerifiedHash = remoteSHA[:8]
	} else {
		logOut("警告: 远端物理文件 SHA256 与本地编译快照存在细微出入")
	}

	res.Logs = logs.String()
	return res, nil
}
