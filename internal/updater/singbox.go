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
	mu        sync.RWMutex // 读写锁：高频读状态零等待
	isRunning bool
	lastError string
	lastCheck time.Time

	// 👑 关键架构优化：内存版本缓存，杜绝重复 fork 进程唤醒 CPU
	cachedStableVer string
	cachedAlphaVer  string
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

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func New(s *store.Store, binDir string) *Updater {
	abs, err := filepath.Abs(binDir)
	if err != nil {
		abs = binDir
	}
	u := &Updater{
		store:  s,
		binDir: abs,
	}

	// 👑 启动时如果发现 DB 无记录，才物理扫描一次补齐历史状态（仅执行一次）
	ctx := context.Background()
	if u.store.GetSetting(ctx, "singbox_local_stable_ver", "") == "" {
		ver := detectLocalBinVersion(filepath.Join(abs, "sing-box-stable"))
		_ = u.store.SetSetting(ctx, "singbox_local_stable_ver", ver)
	}
	if u.store.GetSetting(ctx, "singbox_local_alpha_ver", "") == "" {
		ver := detectLocalBinVersion(filepath.Join(abs, "sing-box-alpha"))
		_ = u.store.SetSetting(ctx, "singbox_local_alpha_ver", ver)
	}

	return u
}

// 刷新内存缓存（调用时需持有锁或在单协程初始化阶段）
func (u *Updater) refreshLocalVersionsLocked() {
	stableBin := filepath.Join(u.binDir, "sing-box-stable")
	alphaBin := filepath.Join(u.binDir, "sing-box-alpha")
	u.cachedStableVer = detectLocalBinVersion(stableBin)
	u.cachedAlphaVer = detectLocalBinVersion(alphaBin)
}

func detectLocalBinVersion(binPath string) string {
	info, err := os.Stat(binPath)
	if err != nil {
		return "未安装"
	}
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

	lastCheckStr := "-"
	if !u.lastCheck.IsZero() {
		lastCheckStr = u.lastCheck.Format("2006-01-02 15:04:05")
	}

	// 👑 彻底剔除 detectLocalBinVersion！直接从 DB 读入，微秒级响应，0 进程，放行 CPU C6 休眠
	return UpdaterStatus{
		IsRunning:           u.isRunning,
		BinDir:              u.binDir,
		LocalStableVersion:  u.store.GetSetting(ctx, "singbox_local_stable_ver", "未安装"),
		LocalAlphaVersion:   u.store.GetSetting(ctx, "singbox_local_alpha_ver", "未安装"),
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
		// 👑 更新完成后，刷新一次内存版本
		u.refreshLocalVersionsLocked()
		u.mu.Unlock()
	}()
}

func (u *Updater) getHTTPClient(ctx context.Context) (*http.Client, error) {
	client := &http.Client{Timeout: 600 * time.Second}
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

// 👑 未来免疫型资产挑选器：按评分选出最适合当前平台的纯静态二进制
func pickBestAsset(assets []releaseAsset) (string, error) {
	expectedUniversalSuffix := fmt.Sprintf("-linux-%s.tar.gz", runtime.GOARCH)

	// 1. 黄金标准：严格匹配官方通用纯静态包 (无任何 glibc / musl / v3 后缀)
	for _, a := range assets {
		if strings.HasSuffix(a.Name, expectedUniversalSuffix) {
			slog.Info("matched official universal static asset", "name", a.Name)
			return a.BrowserDownloadURL, nil
		}
	}

	// 2. 兜底回退：如果未来官方改名，排除一切已知有毒变种 (glibc, musl, v3, legacy)
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, "linux-"+runtime.GOARCH) && strings.HasSuffix(name, ".tar.gz") {
			if !strings.Contains(name, "glibc") && !strings.Contains(name, "musl") &&
				!strings.Contains(name, "v3") && !strings.Contains(name, "legacy") {
				slog.Warn("fallback to clean asset", "name", a.Name)
				return a.BrowserDownloadURL, nil
			}
		}
	}

	return "", fmt.Errorf("未在 Release 资产中找到适用于 linux-%s 的安全静态构建包", runtime.GOARCH)
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

	var targetTag string
	var assets []releaseAsset

	// 👑 统一提取版本号与资产列表，消灭两套逻辑的差异
	if channel == "stable" {
		var rel struct {
			TagName string         `json:"tag_name"`
			Assets  []releaseAsset `json:"assets"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
			return fmt.Errorf("解析 Stable JSON 失败: %w", err)
		}
		targetTag = rel.TagName
		assets = rel.Assets
	} else {
		var rels []struct {
			Prerelease bool           `json:"prerelease"`
			TagName    string         `json:"tag_name"`
			Assets     []releaseAsset `json:"assets"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
			return fmt.Errorf("解析 Alpha JSON 失败: %w", err)
		}
		for _, r := range rels {
			if r.Prerelease {
				targetTag = r.TagName
				assets = r.Assets
				break
			}
		}
	}

	if targetTag != "" {
		_ = u.store.SetSetting(ctx, "singbox_remote_"+channel+"_ver", targetTag)
	}

	// 👑 执行统一的资产挑选（Stable 和 Alpha 享受完全相同的安全过滤）
	downloadURL, err := pickBestAsset(assets)
	if err != nil {
		return err
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
		// 👑 超时放宽至 10 分钟，杜绝 90MB 大包写入中断
		directClient := &http.Client{Timeout: 600 * time.Second}
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
			tmpPath := fmt.Sprintf("%s.tmp.%d", targetPath, time.Now().UnixNano())
			var success bool
			defer func() {
				if !success {
					_ = os.Remove(tmpPath)
				}
			}()

			f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("创建临时二进制文件失败: %w", err)
			}

			written, err := io.Copy(f, tr)
			_ = f.Close()
			if err != nil {
				return fmt.Errorf("写入本地二进制中断 (已写 %d 字节): %w", written, err)
			}

			ver := detectLocalBinVersion(tmpPath)
			if strings.Contains(ver, "异常") || strings.Contains(ver, "损坏") || ver == "未安装" {
				return fmt.Errorf("二进制校验失败 (%s)，已自动回滚销毁", ver)
			}

			if err := os.Rename(tmpPath, targetPath); err != nil {
				return fmt.Errorf("原子替换二进制失败: %w", err)
			}

			_ = u.store.SetSetting(ctx, "singbox_local_"+channel+"_ver", ver)
			success = true
			found = true
			slog.Info("sing-box updated and persisted to DB", "channel", channel, "version", ver)
			break
		}
	}

	if !found {
		return errors.New("压缩包内未找到名为 'sing-box' 的可执行文件")
	}
	return nil
}
