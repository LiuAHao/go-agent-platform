package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-agent-platform/internal/domain/auth"
	"go-agent-platform/internal/domain/shared"
	"go-agent-platform/internal/platform/mysql"
)

// 登录处理
func handleLogin(store *mysql.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request")
			return
		}

		user, err := store.FindUserByEmail(req.Email)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}

		if user.PasswordHash != shared.HashPassword(req.Password) {
			writeError(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}

		// 生成 token
		token := shared.NewID("token")
		if err := store.SaveSessionToken(auth.SessionToken{
			Token:     token,
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to create session")
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"token": token,
			"user": map[string]interface{}{
				"id":    user.ID,
				"email": user.Email,
				"name":  user.Name,
				"role":  "admin",
			},
		})
	}
}

// 用户列表
func handleUsers(store *mysql.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 简化实现
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"items": []interface{}{},
			"total": 0,
		})
	}
}

// 用户操作
func handleUserActions(store *mysql.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/admin/api/v1/users/")
		if path == "" {
			writeError(w, http.StatusBadRequest, "User ID required")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// Skill 列表
func handleSkills(store *mysql.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}

		skills, err := store.ListPlatformSkills()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to list skills")
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"items": skills,
			"total": len(skills),
		})
	}
}

// Skill 操作
func handleSkillActions(store *mysql.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/admin/api/v1/skills/")
		if path == "" {
			writeError(w, http.StatusBadRequest, "Skill ID required")
			return
		}

		switch r.Method {
		case http.MethodGet:
			skill, err := store.FindSkillByID(path)
			if err != nil {
				writeError(w, http.StatusNotFound, "Skill not found")
				return
			}
			writeJSON(w, http.StatusOK, skill)
		case http.MethodDelete:
			if err := store.DeleteSkill("", path); err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to delete skill")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}

// MCP 工具列表
func handleTools(store *mysql.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tools, err := store.ListPlatformTools()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to list tools")
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"items": tools,
			"total": len(tools),
		})
	}
}

// MCP 工具操作
func handleToolActions(store *mysql.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/admin/api/v1/tools/")
		if path == "" {
			writeError(w, http.StatusBadRequest, "Tool ID required")
			return
		}

		switch r.Method {
		case http.MethodGet:
			tool, err := store.FindToolByID(path)
			if err != nil {
				writeError(w, http.StatusNotFound, "Tool not found")
				return
			}
			writeJSON(w, http.StatusOK, tool)
		case http.MethodDelete:
			if err := store.DeleteTool("", path); err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to delete tool")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}

// 统计概览
func handleStatsOverview(store *mysql.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 简化实现
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"total_users":    0,
			"total_skills":   0,
			"total_tools":    0,
			"total_sessions": 0,
			"total_messages": 0,
		})
	}
}

// 每日统计
func handleStatsDaily(store *mysql.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 简化实现
		writeJSON(w, http.StatusOK, []interface{}{})
	}
}

// 系统配置
func handleSystemConfig(store *mysql.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"site_name":        "Go Agent Platform",
			"site_description": "Local-first Agent Studio",
		})
	}
}
