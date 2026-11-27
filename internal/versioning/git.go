// internal/versioning/git.go
package versioning

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"sing-hub/internal/model"
)

const maxAutoBackups = 10

type BackupEntry struct {
	FileName  string `json:"file_name"`
	Path      string `json:"path"`
	Kind      string `json:"kind"` // "auto" | "manual"
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

type GitStatus struct {
	Initialized    bool   `json:"initialized"`
	LocalHead      string `json:"local_head"`
	LocalCommitMsg string `json:"local_commit_msg"`
	LocalCommitAt  string `json:"local_commit_at"`
	RemoteHead     string `json:"remote_head"`
	LastPushedHead string `json:"last_pushed_head"`
	LastPushAt     string `json:"last_push_at"`
	LastPushError  string `json:"last_push_error"`
	AheadCount     int    `json:"ahead_count"`
	Dirty          bool   `json:"dirty"`
	Synced         bool   `json:"synced"`
}

type GitRepo struct {
	repoDir       string
	mu            sync.Mutex
	pushMu        sync.Mutex
	statusMu      sync.RWMutex
	lastPushAt    time.Time
	lastPushError string
}

func NewGitRepo(baseDir string) (*GitRepo, error) {
	repoDir := filepath.Join(baseDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return nil, err
	}

	g := &GitRepo{repoDir: repoDir}
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		if err := g.run("init"); err != nil {
			return nil, fmt.Errorf("git init failed: %w", err)
		}
		_ = g.run("config", "user.name", "Sing-Hub Engine")
		_ = g.run("config", "user.email", "hub@local")
	}

	// 自动迁移旧版 baseDir/backups/*.sql 到 repo/backups/，实现单库统一纳管与推送
	legacyBackupsDir := filepath.Join(baseDir, "backups")
	repoBackupsDir := g.BackupsDir()
	_ = os.MkdirAll(repoBackupsDir, 0755)
	if entries, err := os.ReadDir(legacyBackupsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
				continue
			}
			src := filepath.Join(legacyBackupsDir, e.Name())
			dst := filepath.Join(repoBackupsDir, e.Name())
			if _, statErr := os.Stat(dst); os.IsNotExist(statErr) {
				if data, rErr := os.ReadFile(src); rErr == nil {
					if wErr := os.WriteFile(dst, data, 0644); wErr == nil {
						_ = os.Remove(src)
					}
				}
			}
		}
	}

	return g, nil
}

func (g *GitRepo) BackupsDir() string {
	return filepath.Join(g.repoDir, "backups")
}

func (g *GitRepo) run(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s (%w)", strings.Join(args, " "), string(out), err)
	}
	return nil
}

func (g *GitRepo) runOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s (%w)", strings.Join(args, " "), string(out), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *GitRepo) pruneProfilesLocked(activeProfileIDs []string) {
	profilesDir := filepath.Join(g.repoDir, "profiles")
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		return
	}
	activeSet := make(map[string]struct{}, len(activeProfileIDs))
	for _, id := range activeProfileIDs {
		activeSet[id] = struct{}{}
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		if _, ok := activeSet[id]; !ok {
			_ = os.Remove(filepath.Join(profilesDir, e.Name()))
		}
	}
}

func (g *GitRepo) maybeWriteAutoBackupLocked(sqlDump string) {
	if strings.TrimSpace(sqlDump) == "" {
		return
	}
	backupsDir := g.BackupsDir()
	_ = os.MkdirAll(backupsDir, 0755)

	// 检查最新 auto_*.sql 是否内容一致，一致则不重复生成
	entries, err := os.ReadDir(backupsDir)
	if err == nil {
		var autoFiles []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasPrefix(e.Name(), "auto_") && strings.HasSuffix(e.Name(), ".sql") {
				autoFiles = append(autoFiles, e.Name())
			}
		}
		sort.Strings(autoFiles)
		if len(autoFiles) > 0 {
			latestPath := filepath.Join(backupsDir, autoFiles[len(autoFiles)-1])
			if latestBytes, rErr := os.ReadFile(latestPath); rErr == nil {
				if sha256.Sum256(latestBytes) == sha256.Sum256([]byte(sqlDump)) {
					return
				}
			}
		}
	}

	fileName := fmt.Sprintf("auto_%s.sql", time.Now().Format("20060102_150405"))
	_ = os.WriteFile(filepath.Join(backupsDir, fileName), []byte(sqlDump), 0644)

	// 滚动淘汰，仅保留最近 maxAutoBackups (10) 份自动快照
	entries, err = os.ReadDir(backupsDir)
	if err != nil {
		return
	}
	var autoFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "auto_") && strings.HasSuffix(e.Name(), ".sql") {
			autoFiles = append(autoFiles, e.Name())
		}
	}
	sort.Strings(autoFiles)
	if len(autoFiles) > maxAutoBackups {
		for _, oldName := range autoFiles[:len(autoFiles)-maxAutoBackups] {
			_ = os.Remove(filepath.Join(backupsDir, oldName))
		}
	}
}

func (g *GitRepo) commitAllLocked(msg string) (string, bool, error) {
	if err := g.run("add", "-A"); err != nil {
		return "", false, err
	}
	status, _ := g.runOutput("status", "--porcelain")
	if status == "" {
		hash, _ := g.runOutput("rev-parse", "--short", "HEAD")
		return hash, false, nil
	}
	if err := g.run("commit", "-m", msg); err != nil {
		return "", false, err
	}
	hash, err := g.runOutput("rev-parse", "--short", "HEAD")
	return hash, true, err
}

// CommitProfileAndSync 编译方案后更新配置快照、清理已删方案、按需保留自动 SQL 快照并提交
func (g *GitRepo) CommitProfileAndSync(profileID string, jsonContent []byte, activeProfileIDs []string, sqlDump string, createAutoBackup bool) (string, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	profilesDir := filepath.Join(g.repoDir, "profiles")
	dbDir := filepath.Join(g.repoDir, "database")
	_ = os.MkdirAll(profilesDir, 0755)
	_ = os.MkdirAll(dbDir, 0755)

	jsonPath := filepath.Join(profilesDir, profileID+".json")
	if err := os.WriteFile(jsonPath, jsonContent, 0644); err != nil {
		return "", false, err
	}

	g.pruneProfilesLocked(activeProfileIDs)

	if sqlDump != "" {
		sqlPath := filepath.Join(dbDir, "sing-hub.sql")
		existingSQL, rErr := os.ReadFile(sqlPath)
		dbChanged := rErr != nil || !bytes.Equal(existingSQL, []byte(sqlDump))
		if err := os.WriteFile(sqlPath, []byte(sqlDump), 0644); err != nil {
			return "", false, err
		}
		if dbChanged && createAutoBackup {
			g.maybeWriteAutoBackupLocked(sqlDump)
		}
	}

	msg := fmt.Sprintf("chore(audit): profile [%s] updated at %s", profileID, time.Now().Format("2006-01-02 15:04:05"))
	return g.commitAllLocked(msg)
}

// SyncDatabaseState 全量对账数据库与已删方案（用于增删改节点/积木/母版/方案时同步）
func (g *GitRepo) SyncDatabaseState(activeProfileIDs []string, sqlDump string, createAutoBackup bool) (string, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	dbDir := filepath.Join(g.repoDir, "database")
	_ = os.MkdirAll(dbDir, 0755)

	g.pruneProfilesLocked(activeProfileIDs)

	if sqlDump != "" {
		sqlPath := filepath.Join(dbDir, "sing-hub.sql")
		existingSQL, rErr := os.ReadFile(sqlPath)
		dbChanged := rErr != nil || !bytes.Equal(existingSQL, []byte(sqlDump))
		if err := os.WriteFile(sqlPath, []byte(sqlDump), 0644); err != nil {
			return "", false, err
		}
		if dbChanged && createAutoBackup {
			g.maybeWriteAutoBackupLocked(sqlDump)
		}
	}

	msg := fmt.Sprintf("chore(sync): database state synced at %s", time.Now().Format("2006-01-02 15:04:05"))
	return g.commitAllLocked(msg)
}

func (g *GitRepo) ListBackups() ([]BackupEntry, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	backupsDir := g.BackupsDir()
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupEntry{}, nil
		}
		return nil, err
	}

	list := make([]BackupEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		kind := "manual"
		if strings.HasPrefix(entry.Name(), "auto_") {
			kind = "auto"
		}
		list = append(list, BackupEntry{
			FileName:  entry.Name(),
			Path:      filepath.Join(backupsDir, entry.Name()),
			Kind:      kind,
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime().Format(time.RFC3339),
		})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt > list[j].CreatedAt
	})
	return list, nil
}

func (g *GitRepo) CreateManualBackup(sqlDump string) (BackupEntry, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	backupsDir := g.BackupsDir()
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		return BackupEntry{}, err
	}
	fileName := fmt.Sprintf("manual_%s.sql", time.Now().Format("20060102_150405"))
	targetPath := filepath.Join(backupsDir, fileName)
	if err := os.WriteFile(targetPath, []byte(sqlDump), 0644); err != nil {
		return BackupEntry{}, err
	}
	_, _, _ = g.commitAllLocked(fmt.Sprintf("chore(backup): manual snapshot %s", fileName))

	info, _ := os.Stat(targetPath)
	var size int64
	if info != nil {
		size = info.Size()
	}
	return BackupEntry{
		FileName:  fileName,
		Path:      targetPath,
		Kind:      "manual",
		SizeBytes: size,
		CreatedAt: time.Now().Format(time.RFC3339),
	}, nil
}

func (g *GitRepo) DeleteBackup(nameOrPath string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	base := filepath.Base(strings.TrimSpace(nameOrPath))
	if base == "" || base == "." || base == ".." || !strings.HasSuffix(base, ".sql") {
		return fmt.Errorf("非法的快照文件名")
	}
	target := filepath.Join(g.BackupsDir(), base)
	if err := os.Remove(target); err != nil {
		return err
	}
	_, _, _ = g.commitAllLocked(fmt.Sprintf("chore(backup): delete snapshot %s", base))
	return nil
}

func (g *GitRepo) ReadBackup(nameOrPath string) ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	base := filepath.Base(strings.TrimSpace(nameOrPath))
	if base == "" || base == "." || base == ".." || !strings.HasSuffix(base, ".sql") {
		return nil, fmt.Errorf("非法的快照文件名")
	}
	return os.ReadFile(filepath.Join(g.BackupsDir(), base))
}

func (g *GitRepo) buildGitAuthArgsAndEnv(remoteURL, token, proxyURL string) ([]string, []string) {
	var args []string
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	token = strings.TrimSpace(token)
	if token != "" && (strings.HasPrefix(remoteURL, "http://") || strings.HasPrefix(remoteURL, "https://")) {
		gitUser, gitPass, hasColon := strings.Cut(token, ":")
		if !hasColon {
			gitUser = "oauth2"
			gitPass = token
		}
		env = append(env, "SINGHUB_GIT_USER="+gitUser, "SINGHUB_GIT_PASS="+gitPass)
		args = append(args, "-c", `credential.helper=!f() { printf "username=%s\npassword=%s\n" "$SINGHUB_GIT_USER" "$SINGHUB_GIT_PASS"; }; f`)
	}

	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL != "" && (strings.HasPrefix(remoteURL, "http://") || strings.HasPrefix(remoteURL, "https://")) {
		args = append(args, "-c", "http.proxy="+proxyURL)
	}
	return args, env
}

// PushRemote 通用远端推送（不修改 .git/config，支持 GitHub / Gitea / Forgejo / GitLab / SSH）
func (g *GitRepo) PushRemote(remoteURL, branch, token, proxyURL string) error {
	g.pushMu.Lock()
	defer g.pushMu.Unlock()

	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return fmt.Errorf("未配置远端 Git 仓库地址")
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}

	g.mu.Lock()
	headSHA, headErr := g.runOutput("rev-parse", "HEAD")
	g.mu.Unlock()
	if headErr != nil {
		return fmt.Errorf("本地尚无快照提交，请先在 Studio 编译或创建快照后再推送")
	}

	args, env := g.buildGitAuthArgsAndEnv(remoteURL, token, proxyURL)
	args = append(args, "push", remoteURL, headSHA+":refs/heads/"+branch)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoDir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if token != "" {
			msg = strings.ReplaceAll(msg, token, "***")
		}
		g.statusMu.Lock()
		g.lastPushError = msg
		g.statusMu.Unlock()
		return fmt.Errorf("git push failed: %s (%w)", msg, err)
	}

	g.mu.Lock()
	_ = g.run("update-ref", "refs/singhub/last_pushed", headSHA)
	g.mu.Unlock()

	g.statusMu.Lock()
	g.lastPushAt = time.Now()
	g.lastPushError = ""
	g.statusMu.Unlock()
	return nil
}

// GetStatus 获取本地与远端同步状态（checkRemote=false 时 0 网络请求，毫秒级响应）
func (g *GitRepo) GetStatus(remoteURL, branch, token, proxyURL string, checkRemote bool) GitStatus {
	var st GitStatus
	st.Initialized = true

	g.mu.Lock()
	if out, err := g.runOutput("log", "-1", "--pretty=format:%h|%ct|%s"); err == nil && out != "" {
		parts := strings.SplitN(out, "|", 3)
		if len(parts) >= 1 {
			st.LocalHead = parts[0]
		}
		if len(parts) >= 2 {
			if sec, pErr := strconv.ParseInt(parts[1], 10, 64); pErr == nil {
				st.LocalCommitAt = time.Unix(sec, 0).Format("2006-01-02 15:04:05")
			}
		}
		if len(parts) >= 3 {
			st.LocalCommitMsg = parts[2]
		}
	}

	if statusOut, err := g.runOutput("status", "--porcelain"); err == nil && statusOut != "" {
		st.Dirty = true
	}

	if lastPushed, err := g.runOutput("rev-parse", "--short", "refs/singhub/last_pushed"); err == nil {
		st.LastPushedHead = lastPushed
		if cntStr, cErr := g.runOutput("rev-list", "--count", "refs/singhub/last_pushed..HEAD"); cErr == nil {
			st.AheadCount, _ = strconv.Atoi(cntStr)
		}
	} else if st.LocalHead != "" {
		if cntStr, cErr := g.runOutput("rev-list", "--count", "HEAD"); cErr == nil {
			st.AheadCount, _ = strconv.Atoi(cntStr)
		}
	}
	g.mu.Unlock()

	g.statusMu.RLock()
	if !g.lastPushAt.IsZero() {
		st.LastPushAt = g.lastPushAt.Format("2006-01-02 15:04:05")
	}
	st.LastPushError = g.lastPushError
	g.statusMu.RUnlock()

	remoteURL = strings.TrimSpace(remoteURL)
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}

	if checkRemote && remoteURL != "" {
		args, env := g.buildGitAuthArgsAndEnv(remoteURL, token, proxyURL)
		args = append(args, "ls-remote", remoteURL, "refs/heads/"+branch)
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = g.repoDir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		cancel()
		if err == nil {
			fields := strings.Fields(string(out))
			if len(fields) > 0 && len(fields[0]) >= 7 {
				st.RemoteHead = fields[0][:7]
			}
		} else {
			msg := strings.TrimSpace(string(out))
			if token != "" {
				msg = strings.ReplaceAll(msg, token, "***")
			}
			if msg != "" {
				st.LastPushError = msg
			}
		}
	}

	if st.LocalHead != "" && !st.Dirty {
		if st.RemoteHead != "" {
			st.Synced = st.RemoteHead == st.LocalHead
		} else if st.LastPushedHead != "" {
			st.Synced = st.LastPushedHead == st.LocalHead && st.AheadCount == 0
		}
	}

	return st
}

func (g *GitRepo) ListHistory(profileID string) ([]model.CommitRecord, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	relPath := filepath.Join("profiles", profileID+".json")
	out, err := g.runOutput("log", "--follow", "--pretty=format:%h|%an|%at|%s", "-n", "20", "--", relPath)
	if err != nil {
		return []model.CommitRecord{}, nil
	}

	lines := strings.Split(out, "\n")
	records := make([]model.CommitRecord, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) >= 4 {
			sec, _ := strconv.ParseInt(parts[2], 10, 64)
			records = append(records, model.CommitRecord{
				Hash:      parts[0],
				Author:    parts[1],
				CreatedAt: time.Unix(sec, 0),
				Message:   parts[3],
			})
		}
	}
	return records, nil
}

func (g *GitRepo) GetDiff(profileID string, commitHash string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	relPath := filepath.Join("profiles", profileID+".json")
	return g.runOutput("diff", commitHash, "--", relPath)
}

func (g *GitRepo) Rollback(profileID string, commitHash string) ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	relPath := filepath.Join("profiles", profileID+".json")
	if err := g.run("checkout", commitHash, "--", relPath); err != nil {
		return nil, err
	}

	filePath := filepath.Join(g.repoDir, relPath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	_ = g.run("commit", "-m", fmt.Sprintf("revert(audit): rollback %s to %s", profileID, commitHash))
	return data, nil
}

func (g *GitRepo) CommitComponentTemplate(compID string, compName string, tmplContent string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	compsDir := filepath.Join(g.repoDir, "components")
	_ = os.MkdirAll(compsDir, 0755)

	filePath := filepath.Join(compsDir, compID+".tmpl")
	if err := os.WriteFile(filePath, []byte(tmplContent), 0644); err != nil {
		return "", err
	}

	relPath := filepath.Join("components", compID+".tmpl")
	if err := g.run("add", relPath); err != nil {
		return "", err
	}

	status, _ := g.runOutput("status", "--porcelain", relPath)
	if status == "" {
		return g.runOutput("rev-parse", "--short", "HEAD")
	}

	msg := fmt.Sprintf("feat(template): update component [%s] (%s)", compName, compID)
	if err := g.run("commit", "-m", msg); err != nil {
		return "", err
	}

	return g.runOutput("rev-parse", "--short", "HEAD")
}
