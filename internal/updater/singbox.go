// internal/updater/singbox.go
package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"sing-hub/internal/store"

	"golang.org/x/net/proxy"
)

type Updater struct {
	store     *store.Store
	binDir    string
	mu        sync.Mutex
	isRunning bool
	lastError string
	lastCheck time.Time
}

type UpdaterStatus struct {
	IsRunning           bool   `json:"is_running"`
	BinDir              string `json:"bin_dir"`
	LocalStableVersion  string `json:"local_stable_version"`
	LocalAlphaVersion   string `json:"local_alpha_version"`
	RemoteStableVersion string `json:"remote_stable_version"`
	RemoteAlphaVersion  string `json:"remote_alpha_version"`
	LastCheck           string `json:"last_check"`
	LastError           string `json:"last_error"`
}

func New(s *store.Store, binDir string) *Updater {
	abs, err := filepath.Abs(binDir)
	if err != nil {
		abs = binDir
	}
	return &Updater{
		store:  s,
		binDir: abs,
	}
}

func (u *Updater) GetBinDir() string {
	return u.binDir
}

func detectLocalBinVersion(binPath string) string {
	if _, err := os.Stat(binPath); err != nil {
		return "未安装"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, binPath, "version").Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 0 {
			fields := strings.Fields(lines[0])
			for i, f := range fields {
				if f == "version" && i+1 < len(fields) {
					return "v" + strings.TrimPrefix(fields[i+1], "v")
				}
			}
			return lines[0]
		}
	}
	return "已就绪"
}

func (u *Updater) GetStatus(ctx context.Context) UpdaterStatus {
	u.mu.Lock()
	defer u.mu.Unlock()

	stableBin := filepath.Join(u.binDir, "sing-box-stable")
	alphaBin := filepath.Join(u.binDir, "sing-box-alpha")

	return UpdaterStatus{
		IsRunning:           u.isRunning,
		BinDir:              u.binDir,
		LocalStableVersion:  detectLocalBinVersion(stableBin),
		LocalAlphaVersion:   detectLocalBinVersion(alphaBin),
		RemoteStableVersion: u.store.GetSetting(ctx, "singbox_remote_stable_ver", "-"),
		RemoteAlphaVersion:  u.store.GetSetting(ctx, "singbox_remote_alpha_ver", "-"),
		LastCheck:           u.lastCheck.Format("2006-01-02 15:04:05"),
		LastError:           u.lastError,
	}
}

func (u *Updater) TriggerUpdate(ctx context.Context) {
	u.mu.Lock()
	if u.isRunning {
		u.mu.Unlock()
		return
	}
	u.isRunning = true
	u.lastError = "准备拉取中..."
	u.mu.Unlock()

	go func() {
		err := u.runUpdateFlow()
		u.mu.Lock()
		u.isRunning = false
		u.lastCheck = time.Now()
		if err != nil {
			u.lastError = err.Error()
			slog.Error("singbox update failed", "error", err)
		} else {
			u.lastError = ""
		}
		u.mu.Unlock()
	}()
}

func (u *Updater) StartPeriodicTask() {
	go func() {
		time.Sleep(3 * time.Second)
		u.TriggerUpdate(context.Background())

		for {
			time.Sleep(1 * time.Hour)
			ctx := context.Background()
			interval := u.store.GetSetting(ctx, "singbox_update_interval", "24")
			if interval == "0" {
				continue
			}
			if time.Since(u.lastCheck).Hours() >= 23.5 {
				u.TriggerUpdate(ctx)
			}
		}
	}()
}

func (u *Updater) getHTTPClient(ctx context.Context) (*http.Client, error) {
	client := &http.Client{Timeout: 120 * time.Second}
	proxyStr := u.store.GetSetting(ctx, "github_proxy", "")

	if proxyStr != "" {
		proxyURL, err := url.Parse(proxyStr)
		if err != nil {
			return nil, fmt.Errorf("代理地址解析失败: %w", err)
		}

		transport := &http.Transport{}
		if proxyURL.Scheme == "socks5" || proxyURL.Scheme == "socks5h" {
			dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
			if err != nil {
				return nil, fmt.Errorf("SOCKS5 代理拨号器创建失败: %w", err)
			}
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		} else {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
		client.Transport = transport
	}

	return client, nil
}

func (u *Updater) runUpdateFlow() error {
	ctx := context.Background()
	channel := u.store.GetSetting(ctx, "singbox_update_channel", "all")
	if channel == "disabled" {
		return nil
	}

	if err := os.MkdirAll(u.binDir, 0755); err != nil {
		return fmt.Errorf("无法创建下载目录 %s: %w", u.binDir, err)
	}

	token := u.store.GetSetting(ctx, "git_token", "")
	mirror := strings.TrimRight(u.store.GetSetting(ctx, "github_mirror", ""), "/")

	proxyClient, err := u.getHTTPClient(ctx)
	if err != nil {
		return err
	}

	var errs []string
	if channel == "stable" || channel == "all" {
		if err := u.downloadChannel(ctx, "stable", u.binDir, token, mirror, proxyClient); err != nil {
			errs = append(errs, "Stable: "+err.Error())
		}
	}
	if channel == "alpha" || channel == "all" {
		if err := u.downloadChannel(ctx, "alpha", u.binDir, token, mirror, proxyClient); err != nil {
			errs = append(errs, "Alpha: "+err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, " | "))
	}
	return nil
}

func (u *Updater) downloadChannel(ctx context.Context, channel string, binDir string, token string, mirror string, proxyClient *http.Client) error {
	apiURL := "https://api.github.com/repos/SagerNet/sing-box/releases/latest"
	if channel == "alpha" {
		apiURL = "https://api.github.com/repos/SagerNet/sing-box/releases"
	}

	// 1. 控制流（API）：强制使用配置的代理 Client
	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := proxyClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求 GitHub API 失败 (请检查代理或直连): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API 拒绝 (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var targetTag, downloadURL string
	if channel == "stable" {
		var rel struct {
			TagName string `json:"tag_name"`
			Assets  []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			} `json:"assets"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
			return fmt.Errorf("解析 Stable JSON 失败: %w", err)
		}
		targetTag = rel.TagName
		for _, a := range rel.Assets {
			if strings.Contains(a.Name, "linux-"+runtime.GOARCH) && strings.HasSuffix(a.Name, ".tar.gz") {
				downloadURL = a.BrowserDownloadURL
				break
			}
		}
	} else {
		var rels []struct {
			Prerelease bool   `json:"prerelease"`
			TagName    string `json:"tag_name"`
			Assets     []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			} `json:"assets"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
			return fmt.Errorf("解析 Alpha JSON 失败: %w", err)
		}
		for _, r := range rels {
			if r.Prerelease {
				targetTag = r.TagName
				for _, a := range r.Assets {
					if strings.Contains(a.Name, "linux-"+runtime.GOARCH) && strings.HasSuffix(a.Name, ".tar.gz") {
						downloadURL = a.BrowserDownloadURL
						break
					}
				}
				break
			}
		}
	}

	if targetTag != "" {
		_ = u.store.SetSetting(ctx, "singbox_remote_"+channel+"_ver", targetTag)
	}

	if downloadURL == "" {
		return fmt.Errorf("未找到适用于 linux-%s 的包", runtime.GOARCH)
	}

	targetBinName := "sing-box-stable"
	if channel == "alpha" {
		targetBinName = "sing-box-alpha"
	}
	targetPath := filepath.Join(binDir, targetBinName)

	if detectLocalBinVersion(targetPath) == targetTag {
		slog.Info("sing-box is already up to date", "channel", channel, "version", targetTag)
		return nil
	}

	// 2. 数据流（Asset）：智能级联策略
	// 策略：镜像前缀走直连高速下载；若未配或失败，则自动回退至代理 Client 下载
	var dlResp *http.Response
	if mirror != "" {
		mirroredURL := mirror + "/" + downloadURL
		slog.Info("trying mirror download for sing-box", "channel", channel, "url", mirroredURL)
		directClient := &http.Client{Timeout: 90 * time.Second}
		dlReq, _ := http.NewRequestWithContext(ctx, "GET", mirroredURL, nil)
		res, dErr := directClient.Do(dlReq)
		if dErr == nil && res.StatusCode == 200 {
			dlResp = res
		} else {
			if res != nil {
				res.Body.Close()
			}
			slog.Warn("mirror download failed, fallback to proxy", "mirror_err", dErr)
		}
	}

	if dlResp == nil {
		slog.Info("downloading sing-box via proxy client", "channel", channel, "url", downloadURL)
		dlReq, _ := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
		res, err := proxyClient.Do(dlReq)
		if err != nil {
			return fmt.Errorf("下载二进制包失败: %w", err)
		}
		if res.StatusCode != 200 {
			res.Body.Close()
			return fmt.Errorf("下载请求被拒绝: HTTP %d", res.StatusCode)
		}
		dlResp = res
	}
	defer dlResp.Body.Close()

	gzr, err := gzip.NewReader(dlResp.Body)
	if err != nil {
		return fmt.Errorf("解压 gzip 失败: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	found := false
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("解包 tar 失败: %w", err)
		}
		if !header.FileInfo().IsDir() && strings.HasSuffix(header.Name, "sing-box") {
			f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("创建二进制文件失败: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("写入本地二进制失败: %w", err)
			}
			f.Close()
			found = true
			slog.Info("sing-box updated successfully", "channel", channel, "version", targetTag, "path", targetPath)
			break
		}
	}

	if !found {
		return errors.New("压缩包内未找到名为 'sing-box' 的二进制可执行文件")
	}
	return nil
}
