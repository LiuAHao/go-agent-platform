package httptransport

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"go-agent-platform/internal/app"
	"go-agent-platform/internal/domain/auth"
	"go-agent-platform/internal/platform/mcp"
	wstransport "go-agent-platform/internal/transport/ws"
)

type Server struct {
	app      *app.Application
	http     *http.Server
	wsServer *wstransport.Server
}

func NewHandler(application *app.Application) http.Handler {
	wsServer := wstransport.New(application.Events)
	mux := http.NewServeMux()
	server := &Server{
		app:      application,
		wsServer: wsServer,
	}
	server.registerRoutes(mux)
	return mux
}

func New(addr string, application *app.Application) *Server {
	wsServer := wstransport.New(application.Events)
	mux := http.NewServeMux()
	s := &Server{
		app:      application,
		wsServer: wsServer,
		http: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
	s.registerRoutes(mux)
	return s
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/ws", s.wsServer.Handle)
	mux.HandleFunc("/api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("/api/v1/me", s.withAuth(s.handleMe))
	mux.HandleFunc("/api/v1/workspaces", s.withAuth(s.handleWorkspaces))
	mux.HandleFunc("/api/v1/agents", s.withAuth(s.handleAgents))
	mux.HandleFunc("/api/v1/agents/", s.withAuth(s.handleAgentActions))
	mux.HandleFunc("/api/v1/tools", s.withAuth(s.handleTools))
	mux.HandleFunc("/api/v1/tools/", s.withAuth(s.handleToolActions))
	mux.HandleFunc("/api/v1/skills", s.withAuth(s.handleSkills))
	mux.HandleFunc("/api/v1/skills/", s.withAuth(s.handleSkillActions))
	mux.HandleFunc("/api/v1/models", s.withAuth(s.handleModels))
	mux.HandleFunc("/api/v1/models/", s.withAuth(s.handleModelActions))
	mux.HandleFunc("/api/v1/sessions", s.withAuth(s.handleSessions))
	mux.HandleFunc("/api/v1/sessions/", s.withAuth(s.handleSessionActions))
	mux.HandleFunc("/api/v1/tasks", s.withAuth(s.handleTasks))
	mux.HandleFunc("/api/v1/tasks/", s.withAuth(s.handleTaskActions))
	mux.HandleFunc("/api/v1/approvals", s.withAuth(s.handleApprovals))
	mux.HandleFunc("/api/v1/approvals/", s.withAuth(s.handleApprovalActions))
	mux.HandleFunc("/api/v1/schedules", s.withAuth(s.handleSchedules))
	mux.HandleFunc("/api/v1/audit-events", s.withAuth(s.handleAuditEvents))

	// 存储管理接口
	mux.HandleFunc("/api/v1/storage/stats", s.withAuth(s.handleStorageStats))
	mux.HandleFunc("/api/v1/storage/clean", s.withAuth(s.handleStorageClean))
	mux.HandleFunc("/api/v1/storage/retention", s.withAuth(s.handleStorageRetention))

	// 备份管理接口
	mux.HandleFunc("/api/v1/backup/settings", s.withAuth(s.handleBackupSettings))
	mux.HandleFunc("/api/v1/backup/trigger", s.withAuth(s.handleBackupTrigger))
	mux.HandleFunc("/api/v1/backup/restore", s.withAuth(s.handleBackupRestore))
	mux.HandleFunc("/api/v1/backup/status", s.withAuth(s.handleBackupStatus))

	// 消息管理接口
	mux.HandleFunc("/api/v1/messages", s.withAuth(s.handleMessages))

	// MCP 管理接口
	mux.HandleFunc("/api/v1/mcp/servers", s.withAuth(s.handleMCPServers))
	mux.HandleFunc("/api/v1/mcp/servers/", s.withAuth(s.handleMCPServerActions))
	mux.HandleFunc("/api/v1/mcp/tools", s.withAuth(s.handleMCPTools))
}

func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	resp, err := s.app.Login(req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMe(w http.ResponseWriter, _ *http.Request, user auth.User) {
	writeJSON(w, http.StatusOK, map[string]any{
		"user": user,
	})
}

func (s *Server) handleWorkspaces(w http.ResponseWriter, r *http.Request, user auth.User) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"items": s.app.ListWorkspaces(user.ID)})
	case http.MethodPost:
		var req struct{ Name string `json:"name"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		writeJSON(w, http.StatusCreated, s.app.CreateWorkspace(user.ID, req.Name))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request, user auth.User) {
	switch r.Method {
	case http.MethodGet:
		workspaceID, err := s.app.ResolveWorkspaceID(user.ID, r.URL.Query().Get("workspace_id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": s.app.ListAgents(workspaceID)})
	case http.MethodPost:
		var req app.CreateAgentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		item, err := s.app.CreateAgent(user.ID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAgentActions(w http.ResponseWriter, r *http.Request, user auth.User) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}
	agentID := parts[0]
	switch {
	case len(parts) == 2 && parts[1] == "versions" && r.Method == http.MethodPost:
		item, err := s.app.CreateVersion(user.ID, agentID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
	case len(parts) == 2 && parts[1] == "publish" && r.Method == http.MethodPost:
		var req struct {
			VersionID string `json:"version_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VersionID == "" {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		if err := s.app.PublishVersion(user.ID, agentID, req.VersionID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "published"})
	case len(parts) == 2 && parts[1] == "sessions" && r.Method == http.MethodDelete:
		result, err := s.app.DeleteAgentSessions(user.ID, agentID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeError(w, http.StatusNotFound, "route not found")
	}
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request, user auth.User) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.app.ListToolCatalog(user.ID))
	case http.MethodPost:
		var req app.CreateToolRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		item, err := s.app.CreateTool(user.ID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleToolActions(w http.ResponseWriter, r *http.Request, user auth.User) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/tools/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) == 2 && parts[1] == "install" {
		switch r.Method {
		case http.MethodPost:
			if err := s.app.InstallTool(user.ID, parts[0]); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "installed"})
		case http.MethodDelete:
			if err := s.app.UninstallTool(user.ID, parts[0]); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "uninstalled"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	toolID := parts[0]
	if toolID == "" {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var req app.UpdateToolRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		item, err := s.app.UpdateTool(user.ID, toolID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := s.app.DeleteTool(user.ID, toolID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request, user auth.User) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.app.ListSkillCatalog(user.ID))
	case http.MethodPost:
		var req app.CreateSkillRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		item, err := s.app.CreateSkill(user.ID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleSkillActions(w http.ResponseWriter, r *http.Request, user auth.User) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/skills/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) == 2 && parts[1] == "install" {
		switch r.Method {
		case http.MethodPost:
			if err := s.app.InstallSkill(user.ID, parts[0]); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "installed"})
		case http.MethodDelete:
			if err := s.app.UninstallSkill(user.ID, parts[0]); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "uninstalled"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	skillID := parts[0]
	if skillID == "" {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var req app.UpdateSkillRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		item, err := s.app.UpdateSkill(user.ID, skillID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := s.app.DeleteSkill(user.ID, r.URL.Query().Get("workspace_id"), skillID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request, user auth.User) {
	switch r.Method {
	case http.MethodGet:
		workspaceID, err := s.app.ResolveWorkspaceID(user.ID, r.URL.Query().Get("workspace_id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": s.app.ListModels(workspaceID)})
	case http.MethodPost:
		var req app.CreateModelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		item, err := s.app.CreateModel(user.ID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleModelActions(w http.ResponseWriter, r *http.Request, user auth.User) {
	modelID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/models/"), "/")
	if modelID == "" {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var req app.UpdateModelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		item, err := s.app.UpdateModel(user.ID, modelID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := s.app.DeleteModel(user.ID, r.URL.Query().Get("workspace_id"), modelID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request, user auth.User) {
	switch r.Method {
	case http.MethodGet:
		agentID := strings.TrimSpace(r.URL.Query().Get("agent_id"))
		if agentID == "" {
			writeError(w, http.StatusBadRequest, "agent_id is required")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": s.app.ListSessionsByAgent(user.ID, agentID)})
	case http.MethodPost:
		var req app.CreateSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		item, err := s.app.CreateSession(user.ID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleSessionActions(w http.ResponseWriter, r *http.Request, user auth.User) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// 处理 /api/v1/sessions/{id}/messages
	if len(parts) == 2 && parts[1] == "messages" && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"items": s.app.ListMessages(parts[0])})
		return
	}

	// 处理 /api/v1/sessions/{id}/tasks
	if len(parts) == 2 && parts[1] == "tasks" && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"items": s.app.ListTasksBySession(parts[0])})
		return
	}

	// 处理 /api/v1/sessions/{id} DELETE
	if len(parts) == 1 && parts[0] != "" && r.Method == http.MethodDelete {
		if err := s.app.DeleteSession(user.ID, parts[0]); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		return
	}

	writeError(w, http.StatusNotFound, "route not found")
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req app.ExecuteTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	item, err := s.app.ExecuteTask(user.ID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handleTaskActions(w http.ResponseWriter, r *http.Request, user auth.User) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 1 && r.Method == http.MethodGet {
		item, err := s.app.GetTask(parts[0])
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"task": item, "steps": s.app.GetTaskSteps(parts[0])})
		return
	}
	if len(parts) == 2 && parts[1] == "cancel" && r.Method == http.MethodPost {
		item, err := s.app.CancelTask(user.ID, parts[0])
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	writeError(w, http.StatusNotFound, "route not found")
}

func (s *Server) handleApprovals(w http.ResponseWriter, r *http.Request, _ auth.User) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.app.ListApprovals(r.URL.Query().Get("workspace_id"))})
}

func (s *Server) handleApprovalActions(w http.ResponseWriter, r *http.Request, user auth.User) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/approvals/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || r.Method != http.MethodPost {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}
	var (
		item authzResponse
		err  error
	)
	switch parts[1] {
	case "approve":
		item.approval, err = s.app.Approve(user.ID, parts[0], true)
	case "reject":
		item.approval, err = s.app.Approve(user.ID, parts[0], false)
	default:
		writeError(w, http.StatusNotFound, "route not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item.approval)
}

type authzResponse struct {
	approval any
}

func (s *Server) handleSchedules(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req app.CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	item, err := s.app.CreateSchedule(user.ID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handleAuditEvents(w http.ResponseWriter, r *http.Request, _ auth.User) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.app.ListAuditEvents(r.URL.Query().Get("workspace_id"))})
}

func (s *Server) withAuth(next func(http.ResponseWriter, *http.Request, auth.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			writeJSON(w, http.StatusOK, map[string]any{})
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
		user, err := s.app.Authenticate(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		next(w, r, user)
	}
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]any{"error": message, "timestamp": time.Now().UTC()})
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
}

// 存储管理处理函数

func (s *Server) handleStorageStats(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats, err := s.app.GetStorageStats(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleStorageClean(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	result, err := s.app.ExecuteCleanup(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleStorageRetention(w http.ResponseWriter, r *http.Request, user auth.User) {
	switch r.Method {
	case http.MethodGet:
		policy, err := s.app.GetRetentionPolicy(user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, policy)
	case http.MethodPut:
		var policy app.RetentionPolicy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		if err := s.app.UpdateRetentionPolicy(user.ID, policy); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// 备份管理处理函数

func (s *Server) handleBackupSettings(w http.ResponseWriter, r *http.Request, user auth.User) {
	switch r.Method {
	case http.MethodGet:
		settings, err := s.app.GetBackupSettings(user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, settings)
	case http.MethodPut:
		var settings app.BackupSettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		if err := s.app.UpdateBackupSettings(user.ID, settings); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleBackupTrigger(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := s.app.TriggerBackup(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "backup triggered"})
}

func (s *Server) handleBackupRestore(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := s.app.RestoreBackup(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "backup restored"})
}

func (s *Server) handleBackupStatus(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	status, err := s.app.GetBackupStatus(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// 消息管理处理函数

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	result, err := s.app.ClearAllMessages(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// MCP 管理处理函数

func (s *Server) handleMCPServers(w http.ResponseWriter, r *http.Request, user auth.User) {
	switch r.Method {
	case http.MethodGet:
		// 列出所有 MCP 服务器
		servers := s.app.MCPManager.ListServers()
		writeJSON(w, http.StatusOK, map[string]any{"items": servers})
	case http.MethodPost:
		// 启动新的 MCP 服务器
		var req struct {
			ID      string            `json:"id"`
			Name    string            `json:"name"`
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}

		config := &mcp.ServerConfig{
			ID:      req.ID,
			Name:    req.Name,
			Command: req.Command,
			Args:    req.Args,
			Env:     req.Env,
		}

		if err := s.app.StartMCPServer(config); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"status": "started", "id": req.ID})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleMCPServerActions(w http.ResponseWriter, r *http.Request, user auth.User) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/mcp/servers/")
	serverID := strings.Trim(path, "/")

	if serverID == "" {
		writeError(w, http.StatusBadRequest, "server id is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 获取服务器工具列表
		tools, err := s.app.MCPManager.GetServerTools(serverID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": tools})
	case http.MethodDelete:
		// 停止服务器
		if err := s.app.MCPManager.StopServer(serverID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleMCPTools(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 获取所有 MCP 工具
	allTools := s.app.MCPManager.GetAllTools()
	writeJSON(w, http.StatusOK, map[string]any{"tools": allTools})
}
