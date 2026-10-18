// internal/updater/singbox.go
package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"sing-hub/internal/store"
)

type Updater struct {
	store     *store.Store
	mu        sync.Mutex
	isRunning bool
	lastError string
	lastCheck time.Time
}

type UpdaterStatus struct {
	IsRunning     bool   `json:"is_running"`
	StableVersion string `json:"stable_version"`
	AlphaVersion  string `json:"alpha_version"`
	LastCheck     string `json:"last_check"`
	LastError     string `json:"last_error"`
}

func New(s *store.Store) *Updater {
	return &Updater{store: s}
}

func (u *Updater) GetStatus(ctx context.Context) UpdaterStatus {
	u.mu.Lock()
	defer u.mu.Unlock()
	return UpdaterStatus{
		IsRunning:     u.isRunning,
		StableVersion: u.store.GetSetting(ctx, "singbox_stable_ver", "未安装"),
		AlphaVersion:  u.store.GetSetting(ctx, "singbox_alpha_ver", "未安装"),
		LastCheck:     u.lastCheck.Format("2006-01-02 15:04:05"),
		LastError:     u.lastError,
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
		}
		u.mu.Unlock()
	}()
}

func (u *Updater) StartPeriodicTask() {
	go func() {
		// 启动时延迟 10 秒触发一次初步检测
		time.Sleep(10 * time.Second)
		u.TriggerUpdate(context.Background())

		for {
			time.Sleep(1 * time.Hour)
			ctx := context.Background()
			interval := u.store.GetSetting(ctx, "singbox_update_interval", "24")
			if interval == "0" {
				continue // 仅手动更新
			}
			// 检查是否到达周期
			if time.Since(u.lastCheck).Hours() >= 23.5 {
				u.TriggerUpdate(ctx)
			}
		}
	}()
}

func (u *Updater) runUpdateFlow() error {
	ctx := context.Background()
	channel := u.store.GetSetting(ctx, "singbox_update_channel", "all")
	if channel == "disabled" {
		return nil
	}

	binDir := os.Getenv("SINGBOX_BIN_DIR")
	if binDir == "" {
		dataDir := os.Getenv("DATA_DIR")
		if dataDir == "" {
			dataDir = "./data"
		}
		binDir = filepath.Join(dataDir, "bin")
	}
	_ = os.MkdirAll(binDir, 0755)

	token := u.store.GetSetting(ctx, "git_token", "")

	if channel == "stable" || channel == "all" {
		if err := u.downloadChannel(ctx, "stable", binDir, token); err != nil {
			return err
		}
	}

	if channel == "alpha" || channel == "all" {
		if err := u.downloadChannel(ctx, "alpha", binDir, token); err != nil {
			return err
		}
	}

	return nil
}

func (u *Updater) downloadChannel(ctx context.Context, channel string, binDir string, token string) error {
	apiURL := "https://api.github.com/repos/SagerNet/sing-box/releases/latest"
	if channel == "alpha" {
		apiURL = "https://api.github.com/repos/SagerNet/sing-box/releases"
	}

	req, _ := http.NewRequest("GET", apiURL, nil)
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求 release API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API 拒绝访问, status: %d", resp.StatusCode)
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
			return err
		}
		targetTag = rel.TagName
		for _, a := range rel.Assets {
			if strings.Contains(a.Name, "linux-"+runtime.GOARCH) && strings.HasSuffix(a.Name, ".tar.gz") {
				downloadURL = a.BrowserDownloadURL
				break
			}
		}
	} else {
		// alpha 版需要从列表中找到第一个 prerelease
		var rels []struct {
			Prerelease bool   `json:"prerelease"`
			TagName    string `json:"tag_name"`
			Assets     []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			} `json:"assets"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
			return err
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

	if downloadURL == "" {
		return fmt.Errorf("未找到对应 %s 架构的发布产物", runtime.GOARCH)
	}

	// 检查当前版本是否已是最新
	currentVerKey := "singbox_stable_ver"
	targetBinName := "sing-box-stable"
	if channel == "alpha" {
		currentVerKey = "singbox_alpha_ver"
		targetBinName = "sing-box-alpha"
	}
	currentVer := u.store.GetSetting(ctx, currentVerKey, "")
	if currentVer == targetTag {
		slog.Info("sing-box is already up to date", "channel", channel, "version", targetTag)
		return nil
	}

	slog.Info("downloading sing-box update", "channel", channel, "version", targetTag)

	// 下载并解压
	dlReq, _ := http.NewRequest("GET", downloadURL, nil)
	dlResp, err := client.Do(dlReq)
	if err != nil {
		return fmt.Errorf("下载产物失败: %w", err)
	}
	defer dlResp.Body.Close()

	gzr, err := gzip.NewReader(dlResp.Body)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// 寻找压缩包内的 sing-box 二进制
		if !header.FileInfo().IsDir() && strings.HasSuffix(header.Name, "sing-box") {
			targetPath := filepath.Join(binDir, targetBinName)
			f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
			slog.Info("sing-box updated successfully", "channel", channel, "version", targetTag)
			_ = u.store.SetSetting(ctx, currentVerKey, targetTag)
			break
		}
	}

	return nil
}
