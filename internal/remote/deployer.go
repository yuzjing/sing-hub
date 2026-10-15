// internal/remote/deployer.go
package remote

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
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

// PingTarget 执行双栈感知的 TCP 探活
func PingTarget(endpoint string, port int, timeout time.Duration) (int64, error) {
	if port <= 0 {
		port = 80
	}
	// 关键：自动为 IPv6 包裹中括号 [2001:...]:port，规避解析失败！
	host := endpoint
	if strings.Contains(endpoint, ":") && !strings.HasPrefix(endpoint, "[") {
		// 检查是否已经自带端口
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

// PingAllNodes 一键对所有开启了 enable_ping 的节点并发探活 (协程池并发，秒级完成)
func PingAllNodes(devices []model.Device, timeout time.Duration) []NodePingReport {
	var wg sync.WaitGroup
	reports := make([]NodePingReport, 0, len(devices))
	var mu sync.Mutex

	for _, d := range devices {
		if !d.EnablePing || len(d.Endpoints) == 0 {
			continue
		}
		endpoint := d.Endpoints[0] // 默认探测首选端点
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

// DeployConfig 具备多鉴权模式与 IPv6 支持的四步预检推送
func DeployConfig(d model.Device, configJSON []byte, timeout time.Duration) (string, error) {
	if len(d.Endpoints) == 0 || d.SSHUser == "" {
		return "", fmt.Errorf("device [%s] missing endpoints or ssh_user", d.Name)
	}

	endpoint := d.Endpoints[0]
	port := d.SSHPort
	if port <= 0 {
		port = 22
	}

	// 解析认证方式 (私钥物理路径 / 私钥文本 / 密码)
	var authMethods []ssh.AuthMethod
	switch d.AuthType {
	case "key_path":
		if d.KeyPath != "" {
			keyBytes, err := os.ReadFile(d.KeyPath)
			if err != nil {
				slog.Error("failed to read ssh key from host path", "path", d.KeyPath, "err", err)
				return "", fmt.Errorf("read ssh key file failed: %w", err)
			}
			signer, err := ssh.ParsePrivateKey(keyBytes)
			if err != nil {
				return "", fmt.Errorf("parse private key failed: %w", err)
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
	slog.Info("dialing ssh for deployment", "device_id", d.ID, "addr", addr, "auth_type", d.AuthType)

	client, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		slog.Error("ssh dial failed", "device_id", d.ID, "addr", addr, "err", err)
		return "", fmt.Errorf("ssh dial failed to %s: %w", addr, err)
	}
	defer client.Close()

	remotePath := d.RemotePath
	if remotePath == "" {
		remotePath = "~/.config/sing-box/config.json"
	}
	tempPath := remotePath + ".new"

	// 1. 写入临时文件
	session1, err := client.NewSession()
	if err != nil {
		return "", err
	}
	writeCmd := fmt.Sprintf("mkdir -p $(dirname %s) && cat << 'EOF' > %s\n%s\nEOF", tempPath, tempPath, string(configJSON))
	if out, err := session1.CombinedOutput(writeCmd); err != nil {
		session1.Close()
		slog.Error("failed to write temp file", "err", err, "out", string(out))
		return "", fmt.Errorf("write temp failed: %s (%w)", string(out), err)
	}
	session1.Close()

	// 2. 容器预检 (Pre-flight check)
	session2, err := client.NewSession()
	if err != nil {
		return "", err
	}
	checkCmd := fmt.Sprintf("podman exec sing-box sing-box check -c %s 2>&1 || sing-box check -c %s 2>&1", tempPath, tempPath)
	out, err := session2.CombinedOutput(checkCmd)
	session2.Close()
	if err != nil {
		slog.Error("pre-flight check rejected config", "device_id", d.ID, "out", string(out))
		cleanSession, _ := client.NewSession()
		_, _ = cleanSession.CombinedOutput(fmt.Sprintf("rm -f %s", tempPath))
		cleanSession.Close()
		return "", fmt.Errorf("pre-flight validation error! Old config kept.\n%s", string(out))
	}

	// 3. 原子重命名并重载
	session3, err := client.NewSession()
	if err != nil {
		return "", err
	}
	swapCmd := fmt.Sprintf("mv -f %s %s && (systemctl --user reload-or-restart sing-box || systemctl reload-or-restart sing-box)", tempPath, remotePath)
	if out, err := session3.CombinedOutput(swapCmd); err != nil {
		session3.Close()
		return "", fmt.Errorf("swap and reload error: %s (%w)", string(out), err)
	}
	session3.Close()

	// 4. 读回 SHA-256 校验
	session4, err := client.NewSession()
	if err != nil {
		return "", err
	}
	shaOut, err := session4.CombinedOutput(fmt.Sprintf("sha256sum %s | awk '{print $1}'", remotePath))
	session4.Close()
	if err != nil {
		return "", fmt.Errorf("verify checksum error: %w", err)
	}

	remoteSHA := strings.TrimSpace(string(shaOut))
	localSHA := fmt.Sprintf("%x", sha256.Sum256(configJSON))
	if remoteSHA != localSHA {
		slog.Error("checksum drift detected", "expected", localSHA[:8], "actual", remoteSHA[:8])
		return "", fmt.Errorf("drift! Local %s != Remote %s", localSHA[:8], remoteSHA[:8])
	}

	slog.Info("deploy verified successfully", "device_id", d.ID, "hash", remoteSHA[:8])
	return remoteSHA[:8], nil
}
