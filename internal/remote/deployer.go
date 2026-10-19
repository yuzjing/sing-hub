// internal/remote/deployer.go
package remote

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
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

// DeployResult 推送结果数据结构
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

// DeployConfig 执行详尽 SSH 部署与校验日志收集
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
		return res, fmt.Errorf("节点缺失端点或 SSH 用户")
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
			if err != nil {
				return res, fmt.Errorf("解析私钥失败: %w", err)
			}
			authMethods = append(authMethods, ssh.PublicKeys(signer))
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
	logStep(fmt.Sprintf("Connecting to %s@%s...", d.SSHUser, addr))

	client, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		logs.WriteString(fmt.Sprintf("Error: %v\n", err))
		res.Logs = logs.String()
		return res, fmt.Errorf("SSH 连接失败: %w", err)
	}
	defer client.Close()
	logOut("Connected successfully.")

	// 根据用户选择的部署策略，精准构造命令
	svc := d.ServiceName
	if svc == "" {
		svc = "sing-box"
	}

	var verCmd, checkCmd, reloadCmd string
	remotePath := d.RemotePath
	if remotePath == "" {
		remotePath = "~/.config/sing-box/config.json"
	}
	tempPath := remotePath + ".new"

	switch d.DeployStrategy {
	case "podman":
		logStep(fmt.Sprintf("Strategy: Podman Container [%s]", svc))
		verCmd = fmt.Sprintf("podman exec %s sing-box version 2>/dev/null", svc)
		checkCmd = fmt.Sprintf("podman exec %s sing-box check -c %s 2>&1", svc, tempPath)
		reloadCmd = fmt.Sprintf("podman restart %s 2>&1", svc)
	case "docker":
		logStep(fmt.Sprintf("Strategy: Docker Container [%s]", svc))
		verCmd = fmt.Sprintf("docker exec %s sing-box version 2>/dev/null", svc)
		checkCmd = fmt.Sprintf("docker exec %s sing-box check -c %s 2>&1", svc, tempPath)
		reloadCmd = fmt.Sprintf("docker restart %s 2>&1", svc)
	case "standalone":
		logStep("Strategy: Standalone Binary (No Service Restart)")
		verCmd = "sing-box version 2>/dev/null"
		checkCmd = fmt.Sprintf("sing-box check -c %s 2>&1", tempPath)
		reloadCmd = "echo 'Standalone mode: Service restart skipped.'"
	default:
		logStep(fmt.Sprintf("Strategy: Systemd Service [%s]", svc))
		verCmd = "sing-box version 2>/dev/null"
		checkCmd = fmt.Sprintf("sing-box check -c %s 2>&1", tempPath)
		reloadCmd = fmt.Sprintf("systemctl --user reload-or-restart %s 2>/dev/null || sudo systemctl reload-or-restart %s 2>&1", svc, svc)
	}

	logStep("Detecting remote environment...")
	sessionV, _ := client.NewSession()
	vOut, _ := sessionV.CombinedOutput(verCmd)
	sessionV.Close()
	if strings.TrimSpace(string(vOut)) != "" {
		res.RemoteVersion = strings.Split(strings.TrimSpace(string(vOut)), "\n")[0]
		logOut(res.RemoteVersion)
	} else {
		res.RemoteVersion = "Unknown / Command failed"
		logOut("Warning: Could not detect sing-box version using the specified strategy.")
	}

	logStep("Writing configuration to remote temp path...")
	session1, _ := client.NewSession()
	writeCmd := fmt.Sprintf("mkdir -p $(dirname %s) && cat << 'EOF' > %s\n%s\nEOF", tempPath, tempPath, string(configJSON))
	out1, err := session1.CombinedOutput(writeCmd)
	session1.Close()
	if err != nil {
		logOut(string(out1))
		logs.WriteString(fmt.Sprintf("Error: %v\n", err))
		res.Logs = logs.String()
		return res, fmt.Errorf("写入临时文件失败")
	}

	logStep("Running pre-flight check...")
	session2, _ := client.NewSession()
	out2, err := session2.CombinedOutput(checkCmd)
	session2.Close()
	if err != nil {
		logOut(string(out2))
		logs.WriteString("\n[!] Pre-flight check failed. Aborting deployment.")
		res.Logs = logs.String()
		cleanSession, _ := client.NewSession()
		_, _ = cleanSession.CombinedOutput(fmt.Sprintf("rm -f %s", tempPath))
		cleanSession.Close()
		return res, fmt.Errorf("远端预检失败")
	}
	logOut("Check passed.")

	logStep("Swapping configuration and reloading service...")
	session3, _ := client.NewSession()
	swapCmd := fmt.Sprintf("mv -f %s %s && %s", tempPath, remotePath, reloadCmd)
	out3, err := session3.CombinedOutput(swapCmd)
	session3.Close()
	if err != nil {
		logOut(string(out3))
		res.Logs = logs.String()
		return res, fmt.Errorf("重载服务失败: %v", err)
	}
	logOut("Reload/Restart successful.")

	logStep("Verifying SHA256 checksum...")
	session4, _ := client.NewSession()
	shaOut, err := session4.CombinedOutput(fmt.Sprintf("sha256sum %s | awk '{print $1}'", remotePath))
	session4.Close()

	remoteSHA := strings.TrimSpace(string(shaOut))
	localSHA := fmt.Sprintf("%x", sha256.Sum256(configJSON))

	if err == nil && remoteSHA == localSHA {
		logOut(fmt.Sprintf("Checksum Match: %s", remoteSHA[:8]))
		res.Success = true
		res.VerifiedHash = remoteSHA[:8]
	} else {
		logOut("Warning: Checksum verification failed or drift detected.")
	}

	res.Logs = logs.String()
	return res, nil
}
