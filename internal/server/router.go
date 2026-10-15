// internal/server/router.go
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sing-hub/internal/assembler"
	"sing-hub/internal/model"
	"sing-hub/internal/remote"
	"sing-hub/internal/store"
	"sing-hub/internal/versioning"
)

type Server struct {
	store     *store.Store
	assembler *assembler.Engine
	gitRepo   *versioning.GitRepo
	dbPath    string         // 补齐数据库物理路径
	logLevel  *slog.LevelVar // 动态日志级别控制器
	mux       *http.ServeMux
}

func New(s *store.Store, a *assembler.Engine, g *versioning.GitRepo, dbPath string, level *slog.LevelVar) *Server {
	srv := &Server{
		store:     s,
		assembler: a,
		gitRepo:   g,
		dbPath:    dbPath,
		logLevel:  level,
		mux:       http.NewServeMux(),
	}
	srv.routes()
	return srv
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rw := &responseWriterInterceptor{ResponseWriter: w, statusCode: http.StatusOK}
	s.mux.ServeHTTP(rw, r)

	if !strings.HasPrefix(r.URL.Path, "/_app") && !strings.HasPrefix(r.URL.Path, "/favicon") {
		lvl := slog.LevelInfo
		if rw.statusCode >= 500 {
			lvl = slog.LevelError
		} else if rw.statusCode >= 400 {
			lvl = slog.LevelWarn
		}
		slog.Log(r.Context(), lvl, "http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", r.RemoteAddr,
		)
	}
}

type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterInterceptor) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (s *Server) routes() {
	// 客户端订阅端点
	s.mux.HandleFunc("GET /sub/{token}", s.handleSubscribe)

	// 工坊实时单组件预览沙箱
	s.mux.HandleFunc("POST /api/components/preview", s.handlePreviewComponent)

	// 大盘装配、编译、历史与回滚
	s.mux.HandleFunc("GET /api/profiles/{id}/compile", s.handleCompileProfile)
	s.mux.HandleFunc("GET /api/profiles/{id}/history", s.handleProfileHistory)
	s.mux.HandleFunc("GET /api/profiles/{id}/diff", s.handleProfileDiff)
	s.mux.HandleFunc("POST /api/profiles/{id}/rollback", s.handleProfileRollback)

	// 组件资产 CRUD 与批量删除
	s.mux.HandleFunc("GET /api/components", s.handleListComponents)
	s.mux.HandleFunc("POST /api/components", s.handleSaveComponent)
	s.mux.HandleFunc("DELETE /api/components/{id}", s.handleDeleteComponent)
	s.mux.HandleFunc("POST /api/components/batch-delete", s.handleBatchDeleteComponents)

	// 方案配方 CRUD 与批量删除
	s.mux.HandleFunc("GET /api/profiles", s.handleListProfiles)
	s.mux.HandleFunc("GET /api/profiles/{id}", s.handleGetProfile)
	s.mux.HandleFunc("POST /api/profiles", s.handleSaveProfile)
	s.mux.HandleFunc("DELETE /api/profiles/{id}", s.handleDeleteProfile)
	s.mux.HandleFunc("POST /api/profiles/batch-delete", s.handleBatchDeleteProfiles)

	// 节点实例 CRUD、批量删除、探活与推送
	s.mux.HandleFunc("GET /api/devices", s.handleListDevices)
	s.mux.HandleFunc("POST /api/devices", s.handleSaveDevice)
	s.mux.HandleFunc("DELETE /api/devices/{id}", s.handleDeleteDevice)
	s.mux.HandleFunc("POST /api/devices/batch-delete", s.handleBatchDeleteDevices)
	s.mux.HandleFunc("POST /api/devices/{id}/ping", s.handlePingDevice)
	s.mux.HandleFunc("POST /api/devices/ping-all", s.handlePingAllDevices)
	s.mux.HandleFunc("POST /api/devices/{id}/deploy", s.handleDeployDevice)

	// 服务端路径备份、生成与热恢复
	s.mux.HandleFunc("GET /api/system/backups", s.handleListServerBackups)
	s.mux.HandleFunc("POST /api/system/backup/create", s.handleCreateServerBackup)
	s.mux.HandleFunc("POST /api/system/restore/path", s.handleRestoreFromPath)

	// 动态日志级别与设置
	s.mux.HandleFunc("GET /api/system/log-level", s.handleGetLogLevel)
	s.mux.HandleFunc("POST /api/system/log-level", s.handleSetLogLevel)
	s.mux.HandleFunc("GET /api/system/settings", s.handleGetSettings)
	s.mux.HandleFunc("POST /api/system/settings", s.handleSaveSettings)

	s.RegisterStatic()
}

func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	profile, err := s.store.GetProfileByToken(r.Context(), token)
	if err != nil {
		http.Error(w, "profile not found", http.StatusNotFound)
		return
	}
	if !profile.Capabilities.EnableSubscribe {
		http.Error(w, "subscription disabled", http.StatusForbidden)
		return
	}

	data, commitHash, err := s.assembler.Assemble(r.Context(), profile.ID)
	if err != nil {
		slog.Error("assemble subscribe error", "profile", profile.ID, "err", err)
		http.Error(w, "assemble error", http.StatusInternalServerError)
		return
	}

	etag := fmt.Sprintf(`"%s"`, commitHash)
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleCompileProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data, commitHash, err := s.assembler.Assemble(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("X-Commit-Hash", commitHash)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handlePreviewComponent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PayloadTmpl string `json:"payload_tmpl"`
		ParamsJSON  string `json:"params_json"`
		NodeID      string `json:"node_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	preview, err := s.assembler.PreviewComponent(r.Context(), req.PayloadTmpl, req.ParamsJSON, req.NodeID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(preview))
}

func (s *Server) handleProfileHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.gitRepo == nil {
		writeJSON(w, http.StatusOK, []model.CommitRecord{})
		return
	}
	history, err := s.gitRepo.ListHistory(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (s *Server) handleProfileDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	commit := r.URL.Query().Get("commit")
	if s.gitRepo == nil || commit == "" {
		writeJSON(w, http.StatusOK, map[string]string{"diff": ""})
		return
	}
	diff, err := s.gitRepo.GetDiff(id, commit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": diff})
}

func (s *Server) handleProfileRollback(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	commit := r.URL.Query().Get("commit")
	if s.gitRepo == nil || commit == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing commit"})
		return
	}
	data, err := s.gitRepo.Rollback(id, commit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handlePingDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	devices, _ := s.store.ListDevices(r.Context())
	var target *model.Device
	for _, d := range devices {
		if d.ID == id {
			target = &d
			break
		}
	}
	if target == nil || len(target.Endpoints) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node has no endpoints"})
		return
	}
	ms, err := remote.PingTarget(target.Endpoints[0], target.SSHPort, 2*time.Second)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"online": false, "latency_ms": 0, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"online": true, "latency_ms": ms})
}

func (s *Server) handlePingAllDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.store.ListDevices(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	reports := remote.PingAllNodes(devices, 2*time.Second)
	writeJSON(w, http.StatusOK, reports)
}

func (s *Server) handleDeployDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	devices, _ := s.store.ListDevices(r.Context())
	var target *model.Device
	for _, d := range devices {
		if d.ID == id {
			target = &d
			break
		}
	}
	if target == nil || target.TargetProfileID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no profile assigned"})
		return
	}

	configJSON, _, err := s.assembler.Assemble(r.Context(), target.TargetProfileID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "assemble error: " + err.Error()})
		return
	}

	hash, err := remote.DeployConfig(*target, configJSON, 5*time.Second)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "success", "verified_hash": hash})
}

// 扫描服务端本地备份目录
func (s *Server) handleListServerBackups(w http.ResponseWriter, r *http.Request) {
	backupDir := filepath.Join(filepath.Dir(s.dbPath), "backups")
	_ = os.MkdirAll(backupDir, 0755)

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type BackupItem struct {
		FileName  string `json:"file_name"`
		Path      string `json:"path"`
		SizeBytes int64  `json:"size_bytes"`
		CreatedAt string `json:"created_at"`
	}

	list := make([]BackupItem, 0)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			info, err := entry.Info()
			if err == nil {
				list = append(list, BackupItem{
					FileName:  entry.Name(),
					Path:      filepath.Join(backupDir, entry.Name()),
					SizeBytes: info.Size(),
					CreatedAt: info.ModTime().Format("2006-01-02 15:04:05"),
				})
			}
		}
	}
	writeJSON(w, http.StatusOK, list)
}

// 服务端自动生成快照并落盘，顺带提交 Git 记录
func (s *Server) handleCreateServerBackup(w http.ResponseWriter, r *http.Request) {
	sqlDump, err := s.store.DumpSQL(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "dump sql error: " + err.Error()})
		return
	}

	backupDir := filepath.Join(filepath.Dir(s.dbPath), "backups")
	_ = os.MkdirAll(backupDir, 0755)

	fileName := fmt.Sprintf("sing-hub-%s.sql", time.Now().Format("20060102-150405"))
	targetPath := filepath.Join(backupDir, fileName)

	if err := os.WriteFile(targetPath, []byte(sqlDump), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "write file error: " + err.Error()})
		return
	}

	var commitHash string
	if s.gitRepo != nil {
		commitHash, _ = s.gitRepo.CommitSnapshot("system-backup", []byte("{}"), sqlDump)
	}

	slog.Info("server-side backup snapshot created", "path", targetPath, "commit", commitHash)
	writeJSON(w, http.StatusOK, map[string]string{
		"file_name":   fileName,
		"path":        targetPath,
		"commit_hash": commitHash,
	})
}

// 指定服务端绝对路径一键原地热恢复
func (s *Server) handleRestoreFromPath(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := readJSON(r, &req); err != nil || req.Path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing server file path"})
		return
	}

	content, err := os.ReadFile(req.Path)
	if err != nil {
		slog.Error("failed to read restore file", "path", req.Path, "err", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read file failed: " + err.Error()})
		return
	}

	if err := s.store.RestoreSQL(r.Context(), string(content)); err != nil {
		slog.Error("restore sql execution failed", "path", req.Path, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "execute restore failed: " + err.Error()})
		return
	}

	slog.Info("database successfully restored from path", "path", req.Path)
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored", "path": req.Path})
}

func (s *Server) handleGetLogLevel(w http.ResponseWriter, r *http.Request) {
	cur := "INFO"
	if s.logLevel != nil && s.logLevel.Level() == slog.LevelDebug {
		cur = "DEBUG"
	}
	writeJSON(w, http.StatusOK, map[string]string{"level": cur})
}

func (s *Server) handleSetLogLevel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Level string `json:"level"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if s.logLevel != nil {
		if strings.ToUpper(req.Level) == "DEBUG" {
			s.logLevel.Set(slog.LevelDebug)
		} else {
			s.logLevel.Set(slog.LevelInfo)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings := map[string]string{
		"git_remote_url": s.store.GetSetting(r.Context(), "git_remote_url", ""),
		"git_branch":     s.store.GetSetting(r.Context(), "git_branch", "main"),
		"git_token":      s.store.GetSetting(r.Context(), "git_token", ""),
		"ssh_timeout":    s.store.GetSetting(r.Context(), "ssh_timeout", "5"),
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	for k, v := range req {
		_ = s.store.SetSetting(r.Context(), k, v)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListComponents(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListComponents(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleSaveComponent(w http.ResponseWriter, r *http.Request) {
	var c model.Component
	if err := readJSON(r, &c); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.SaveComponent(r.Context(), c); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	if s.gitRepo != nil {
		_, _ = s.gitRepo.CommitComponentTemplate(c.ID, c.Name, c.PayloadTmpl)
	}
}

func (s *Server) handleDeleteComponent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteComponent(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleBatchDeleteComponents(w http.ResponseWriter, r *http.Request) {
	var req model.BatchDeleteReq
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.BatchDeleteComponents(r.Context(), req.IDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListProfiles(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.store.GetProfile(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleSaveProfile(w http.ResponseWriter, r *http.Request) {
	var p model.Profile
	if err := readJSON(r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.SaveProfile(r.Context(), p); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteProfile(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleBatchDeleteProfiles(w http.ResponseWriter, r *http.Request) {
	var req model.BatchDeleteReq
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.BatchDeleteProfiles(r.Context(), req.IDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListDevices(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleSaveDevice(w http.ResponseWriter, r *http.Request) {
	var d model.Device
	if err := readJSON(r, &d); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.SaveDevice(r.Context(), d); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteDevice(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleBatchDeleteDevices(w http.ResponseWriter, r *http.Request) {
	var req model.BatchDeleteReq
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.BatchDeleteDevices(r.Context(), req.IDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func readJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 10*1024*1024)).Decode(target)
}
