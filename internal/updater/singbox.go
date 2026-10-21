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
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"sing-hub/internal/store"

	"golang.org/x/net/proxy"
)

var verExtractRegex = regexp.MustCompile(`([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[a-zA-Z0-9.]+)?(?:-alpha\.[0-9]+)?)`)

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

// detectLocalBinVersion 绝对真实物理探测：杜绝任何“已就绪”等歧义词
func detectLocalBinVersion(binPath string) string {
	info, err := os.Stat(binPath)
	if err != nil {
		return "未安装"
	}
	// 校验文件非空且具备基础可执行权限
	if info.Size() == 0 {
		return "文件损坏 (0字节)"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2000*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Warn("failed to execute local sing-box binary", "path", binPath, "err", err, "out", string(out))
		return "执行异常 (文件损坏或缺少动态库)"
	}

	raw := strings.TrimSpace(string(out))
	if matches := verExtractRegex.FindStringSubmatch(raw); len(matches) > 1 {
		return "v" + strings.TrimPrefix(matches[1], "v")
	}

	if len(raw) > 0 {
		return strings.Split(raw, "\n")[0]
	}
	return "版本未知"
}

func (u *Updater) GetStatus(ctx context.Context) UpdaterStatus {
	u.mu.Lock()
	defer u.mu.Unlock()

	stableBin := filepath.Join(u.binDir, "sing-box-stable")
	alphaBin := filepath.Join(u.binDir, "sing-box-alpha")

	lastCheckStr := "-"
	if !u.lastCheck.IsZero() {
		lastCheckStr = u.lastCheck.Format("2006-01-02 15:04:05")
	}

	return UpdaterStatus{
		IsRunning:           u.isRunning,
		BinDir:              u.binDir,
		LocalStableVersion:  detectLocalBinVersion(stableBin),
		LocalAlphaVersion:   detectLocalBinVersion(alphaBin),
		RemoteStableVersion: u.store.GetSetting(ctx, "singbox_remote_stable_ver", "-"),
		RemoteAlphaVersion:  u.store.GetSetting(ctx, "singbox_remote_alpha_ver", "-"),
		LastCheck:           lastCheckStr,
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
	// 关键优化：清空上次错误，拉取中不要把临时提示塞进 lastError 导致前端误报异常！
	u.lastError = ""
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
				return nil, fmt.Errorf("SOCKS5 代理创建失败: %w", err)
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
		return fmt.Errorf("无法创建下载目录: %w", err)
	}

	token := u.store.GetSetting(ctx, "git_token", "")
	mirror := strings.TrimRight(u.store.GetSetting(ctx, "github_mirror", ""), "/")

	proxyClient, err := u.getHTTPClient(ctx)
	if err != nil {
		return err
	}

	var errs []string
	if channel == "stable" || channel == "all" {
		if err := u.downloadChannel(ctx, "stable", token, mirror, proxyClient); err != nil {
			errs = append(errs, "Stable: "+err.Error())
		}
	}
	if channel == "alpha" || channel == "all" {
		if err := u.downloadChannel(ctx, "alpha", token, mirror, proxyClient); err != nil {
			errs = append(errs, "Alpha: "+err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, " | "))
	}
	return nil
}

func (u *Updater) downloadChannel(ctx context.Context, channel string, token string, mirror string, proxyClient *http.Client) error {
	apiURL := "https://api.github.com/repos/SagerNet/sing-box/releases/latest"
	if channel == "alpha" {
		apiURL = "https://api.github.com/repos/SagerNet/sing-box/releases"
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := proxyClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API 访问被拒 (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
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
		return fmt.Errorf("未找到适用于 linux-%s 的安装包", runtime.GOARCH)
	}

	targetBinName := "sing-box-stable"
	if channel == "alpha" {
		targetBinName = "sing-box-alpha"
	}
	targetPath := filepath.Join(u.binDir, targetBinName)

	// 本地存在且运行正常，版本相同则直接跳过
	if detectLocalBinVersion(targetPath) == targetTag {
		slog.Info("sing-box is already up to date", "channel", channel, "version", targetTag)
		return nil
	}

	var dlResp *http.Response
	if mirror != "" {
		mirroredURL := mirror + "/" + downloadURL
		slog.Info("trying mirror download", "channel", channel, "url", mirroredURL)
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
			return fmt.Errorf("下载安装包失败: %w", err)
		}
		if res.StatusCode != 200 {
			res.Body.Close()
			return fmt.Errorf("下载被拒绝 (HTTP %d)", res.StatusCode)
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
		return errors.New("压缩包内未找到名为 'sing-box' 的可执行文件")
	}
	return nil
}
