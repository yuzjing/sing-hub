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
	"sing-hub/internal/updater"
	"sing-hub/internal/versioning"
)

type Server struct {
	store     *store.Store
	assembler *assembler.Engine
	gitRepo   *versioning.GitRepo
	updater   *updater.Updater
	dbPath    string
	logLevel  *slog.LevelVar
	mux       *http.ServeMux
}

func New(s *store.Store, a *assembler.Engine, g *versioning.GitRepo, u *updater.Updater, dbPath string, level *slog.LevelVar) *Server {
	srv := &Server{
		store:     s,
		assembler: a,
		gitRepo:   g,
		updater:   u,
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

	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("CRITICAL: panic recovered in http handler",
				"error", fmt.Sprintf("%v", rec),
				"path", r.URL.Path,
				"method", r.Method,
			)
			http.Error(w, fmt.Sprintf("Internal Server Error: %v", rec), http.StatusInternalServerError)
		}

		if !strings.HasPrefix(r.URL.Path, "/_app") && !strings.HasPrefix(r.URL.Path, "/favicon") {
			lvl := slog.LevelInfo
			if rw.statusCode >= 500 {
				lvl = slog.LevelError
			} else if rw.statusCode >= 400 {
				if strings.HasSuffix(r.URL.Path, "/preview") {
					lvl = slog.LevelDebug
				} else {
					lvl = slog.LevelWarn
				}
			}
			slog.Log(r.Context(), lvl, "http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.statusCode,
				"duration_ms", time.Since(start).Milliseconds(),
				"client_ip", r.RemoteAddr,
			)
		}
	}()

	s.mux.ServeHTTP(rw, r)
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
	s.mux.HandleFunc("GET /sub/{token}", s.handleSubscribe)

	s.mux.HandleFunc("POST /api/components/preview", s.handlePreviewComponent)

	s.mux.HandleFunc("POST /api/profiles/{id}/compile", s.handleCompileProfile)
	s.mux.HandleFunc("GET /api/profiles/{id}/compile", s.handleCompileProfile)
	s.mux.HandleFunc("GET /api/profiles/{id}/history", s.handleProfileHistory)
	s.mux.HandleFunc("GET /api/profiles/{id}/diff", s.handleProfileDiff)
	s.mux.HandleFunc("POST /api/profiles/{id}/rollback", s.handleProfileRollback)

	s.mux.HandleFunc("GET /api/components", s.handleListComponents)
	s.mux.HandleFunc("POST /api/components", s.handleSaveComponent)
	s.mux.HandleFunc("DELETE /api/components/{id}", s.handleDeleteComponent)
	s.mux.HandleFunc("POST /api/components/batch-delete", s.handleBatchDeleteComponents)

	s.mux.HandleFunc("GET /api/modules", s.handleListModules)
	s.mux.HandleFunc("POST /api/modules", s.handleSaveModule)
	s.mux.HandleFunc("DELETE /api/modules/{id}", s.handleDeleteModule)
	s.mux.HandleFunc("POST /api/modules/batch-delete", s.handleBatchDeleteModules)

	s.mux.HandleFunc("GET /api/profiles", s.handleListProfiles)
	s.mux.HandleFunc("GET /api/profiles/{id}", s.handleGetProfile)
	s.mux.HandleFunc("POST /api/profiles", s.handleSaveProfile)
	s.mux.HandleFunc("DELETE /api/profiles/{id}", s.handleDeleteProfile)
	s.mux.HandleFunc("POST /api/profiles/batch-delete", s.handleBatchDeleteProfiles)

	s.mux.HandleFunc("GET /api/devices", s.handleListDevices)
	s.mux.HandleFunc("POST /api/devices", s.handleSaveDevice)
	s.mux.HandleFunc("DELETE /api/devices/{id}", s.handleDeleteDevice)
	s.mux.HandleFunc("POST /api/devices/batch-delete", s.handleBatchDeleteDevices)
	s.mux.HandleFunc("POST /api/devices/{id}/ping", s.handlePingDevice)
	s.mux.HandleFunc("POST /api/devices/ping-all", s.handlePingAllDevices)
	s.mux.HandleFunc("POST /api/devices/{id}/deploy", s.handleDeployDevice)

	s.mux.HandleFunc("GET /api/system/updater", s.handleGetUpdaterStatus)
	s.mux.HandleFunc("POST /api/system/updater/trigger", s.handleTriggerUpdate)
	s.mux.HandleFunc("GET /api/system/updater/trigger", s.handleTriggerUpdate)

	s.mux.HandleFunc("GET /api/system/backups", s.handleListServerBackups)
	s.mux.HandleFunc("POST /api/system/backup/create", s.handleCreateServerBackup)
	s.mux.HandleFunc("POST /api/system/restore/path", s.handleRestoreFromPath)

	s.mux.HandleFunc("GET /api/system/log-level", s.handleGetLogLevel)
	s.mux.HandleFunc("POST /api/system/log-level", s.handleSetLogLevel)

	s.mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	s.mux.HandleFunc("POST /api/settings", s.handleSaveSettings)
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

	data, commitHash, _, err := s.assembler.Assemble(r.Context(), profile.ID)
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
	data, commitHash, report, err := s.assembler.Assemble(r.Context(), id)
	if err != nil {
		slog.Error("compile profile failed", "profile_id", id, "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	slog.Info("compile profile success",
		"profile_id", id,
		"bytes", len(data),
		"commit", commitHash,
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"json":        string(data),
		"commit_hash": commitHash,
		"validation":  report,
	})
}

func (s *Server) handlePreviewComponent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Template    string `json:"template"`
		PayloadTmpl string `json:"payload_tmpl"`
		Params      string `json:"params"`
		ParamsJSON  string `json:"params_json"`
		NodeID      string `json:"node_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	tmplStr := req.Template
	if strings.TrimSpace(tmplStr) == "" {
		tmplStr = req.PayloadTmpl
	}
	paramsStr := req.Params
	if strings.TrimSpace(paramsStr) == "" {
		paramsStr = req.ParamsJSON
	}

	res, err := s.assembler.PreviewComponent(r.Context(), tmplStr, paramsStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"json": res})
}

func (s *Server) handleProfileHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.gitRepo == nil {
		writeJSON(w, http.StatusOK, []model.CommitRecord{})
		return
	}
	list, err := s.gitRepo.ListHistory(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleProfileDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	commit := r.URL.Query().Get("commit")
	if commit == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "commit query param required"})
		return
	}
	if s.gitRepo == nil {
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
	var req struct {
		Commit string `json:"commit"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if s.gitRepo == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "git repo not initialized"})
		return
	}
	content, err := s.gitRepo.Rollback(id, req.Commit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"json": string(content)})
}

func (s *Server) handlePingDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dev, err := s.store.GetDevice(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
		return
	}

	endpoint := dev.SSHHost
	if len(dev.Endpoints) > 0 {
		endpoint = dev.Endpoints[0]
	}
	port := dev.SSHPort
	if port <= 0 {
		port = 22
	}

	ms, err := remote.PingTarget(endpoint, port, 3*time.Second)
	report := remote.NodePingReport{
		DeviceID:  dev.ID,
		Endpoint:  endpoint,
		Online:    err == nil,
		LatencyMS: ms,
	}
	if err != nil {
		report.Error = err.Error()
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handlePingAllDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.store.ListDevices(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	reports := remote.PingAllNodes(devices, 3*time.Second)
	writeJSON(w, http.StatusOK, reports)
}

func (s *Server) handleDeployDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dev, err := s.store.GetDevice(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
		return
	}
	if !dev.EnablePush {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "device push disabled"})
		return
	}
	if strings.TrimSpace(dev.TargetProfileID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target profile not assigned"})
		return
	}

	configBytes, _, _, err := s.assembler.Assemble(r.Context(), dev.TargetProfileID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("assemble error: %v", err)})
		return
	}

	timeoutSec := 10
	timeoutStr := s.store.GetSetting(r.Context(), "ssh_timeout", "10")
	var t int
	if _, err := fmt.Sscanf(timeoutStr, "%d", &t); err == nil && t > 0 {
		timeoutSec = t
	}

	result, err := remote.DeployConfig(*dev, configBytes, time.Duration(timeoutSec)*time.Second)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "logs": result.Logs})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGetUpdaterStatus(w http.ResponseWriter, r *http.Request) {
	if s.updater == nil {
		writeJSON(w, http.StatusOK, map[string]any{"is_running": false})
		return
	}
	status := s.updater.GetStatus(r.Context())
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleTriggerUpdate(w http.ResponseWriter, r *http.Request) {
	if s.updater != nil {
		s.updater.TriggerUpdate(r.Context())
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "triggered"})
}

func (s *Server) handleListServerBackups(w http.ResponseWriter, r *http.Request) {
	backupDir := filepath.Join(filepath.Dir(s.dbPath), "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type BackupItem struct {
		FileName  string `json:"file_name"`
		Path      string `json:"path"`
		SizeBytes int64  `json:"size_bytes"`
		CreatedAt string `json:"created_at"`
	}

	var list []BackupItem
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		list = append(list, BackupItem{
			FileName:  entry.Name(),
			Path:      filepath.Join(backupDir, entry.Name()),
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateServerBackup(w http.ResponseWriter, r *http.Request) {
	backupDir := filepath.Join(filepath.Dir(s.dbPath), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot create backup dir: " + err.Error()})
		return
	}

	dumpSQL, err := s.store.DumpSQL(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "dump error: " + err.Error()})
		return
	}

	fileName := fmt.Sprintf("backup_%s.sql", time.Now().Format("20060102_150405"))
	targetPath := filepath.Join(backupDir, fileName)

	if err := os.WriteFile(targetPath, []byte(dumpSQL), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "write backup error: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"file_name": fileName, "path": targetPath})
}

func (s *Server) handleRestoreFromPath(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	sqlBytes, err := os.ReadFile(req.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot read backup file: " + err.Error()})
		return
	}

	if err := s.store.RestoreSQL(r.Context(), string(sqlBytes)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "restore error: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}

func (s *Server) handleGetLogLevel(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"level": s.logLevel.Level().String(),
	})
}

func (s *Server) handleSetLogLevel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Level string `json:"level"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	switch strings.ToUpper(req.Level) {
	case "DEBUG":
		s.logLevel.Set(slog.LevelDebug)
	case "INFO":
		s.logLevel.Set(slog.LevelInfo)
	case "WARN":
		s.logLevel.Set(slog.LevelWarn)
	case "ERROR":
		s.logLevel.Set(slog.LevelError)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid log level"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"level": s.logLevel.Level().String(),
	})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings := map[string]string{
		"git_remote_url":          s.store.GetSetting(r.Context(), "git_remote_url", ""),
		"git_branch":              s.store.GetSetting(r.Context(), "git_branch", "main"),
		"git_token":               s.store.GetSetting(r.Context(), "git_token", ""),
		"ssh_timeout":             s.store.GetSetting(r.Context(), "ssh_timeout", "10"),
		"singbox_update_channel":  s.store.GetSetting(r.Context(), "singbox_update_channel", "all"),
		"singbox_update_interval": s.store.GetSetting(r.Context(), "singbox_update_interval", "24"),
		"singbox_linter_enabled":  s.store.GetSetting(r.Context(), "singbox_linter_enabled", "true"),
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
}

func (s *Server) handleDeleteComponent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteComponent(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleListModules(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListModules(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleSaveModule(w http.ResponseWriter, r *http.Request) {
	var m model.Module
	if err := readJSON(r, &m); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.SaveModule(r.Context(), m); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteModule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteModule(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleBatchDeleteModules(w http.ResponseWriter, r *http.Request) {
	var req model.BatchDeleteReq
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.BatchDeleteModules(r.Context(), req.IDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func readJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 10<<20)).Decode(target)
}
