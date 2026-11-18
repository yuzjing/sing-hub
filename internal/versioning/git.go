// internal/versioning/git.go
package versioning

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"sing-hub/internal/model"
)

type GitRepo struct {
	repoDir string
	mu      sync.Mutex
	pushMu  sync.Mutex
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

	return g, nil
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

// CommitSnapshot 保存纯文本 JSON，并顺手将当前数据库的 SQL Dump 一起提交入 Git
func (g *GitRepo) CommitSnapshot(profileID string, jsonContent []byte, sqlDump string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	profilesDir := filepath.Join(g.repoDir, "profiles")
	dbDir := filepath.Join(g.repoDir, "database")
	_ = os.MkdirAll(profilesDir, 0755)
	_ = os.MkdirAll(dbDir, 0755)

	// 1. 写入成品配置 JSON
	jsonPath := filepath.Join(profilesDir, profileID+".json")
	if err := os.WriteFile(jsonPath, jsonContent, 0644); err != nil {
		return "", err
	}

	// 2. 写入纯文本 SQL 备份
	if sqlDump != "" {
		sqlPath := filepath.Join(dbDir, "sing-hub.sql")
		_ = os.WriteFile(sqlPath, []byte(sqlDump), 0644)
	}

	_ = g.run("add", ".")

	status, _ := g.runOutput("status", "--porcelain")
	if status == "" {
		return g.runOutput("rev-parse", "--short", "HEAD")
	}

	msg := fmt.Sprintf("chore(audit): profile [%s] updated at %s", profileID, time.Now().Format("2006-01-02 15:04:05"))
	if err := g.run("commit", "-m", msg); err != nil {
		return "", err
	}

	return g.runOutput("rev-parse", "--short", "HEAD")
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
	_, headErr := g.runOutput("rev-parse", "--verify", "HEAD")
	g.mu.Unlock()
	if headErr != nil {
		return fmt.Errorf("本地尚无快照提交，请先在 Studio 编译任意方案后再推送")
	}

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

	args = append(args, "push", remoteURL, "HEAD:"+branch)

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
		return fmt.Errorf("git push failed: %s (%w)", msg, err)
	}
	return nil
}
