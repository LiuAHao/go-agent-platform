package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"go-agent-platform/internal/config"
	"go-agent-platform/internal/platform/mysql"

	"github.com/rs/cors"
)

func main() {
	cfg := config.Load()

	// 初始化 MySQL 存储
	store, err := mysql.New(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
	}
	defer store.Close()

	// 确保种子数据
	if err := store.EnsureSeedData(cfg); err != nil {
		log.Printf("Warning: Failed to seed data: %v", err)
	}

	// 创建路由
	mux := http.NewServeMux()

	// 注册路由
	registerRoutes(mux, store)

	// 配置 CORS
	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}).Handler(mux)

	// 获取监听地址
	addr := os.Getenv("ADMIN_HTTP_ADDR")
	if addr == "" {
		addr = ":8082"
	}

	fmt.Printf("Admin API server starting on %s\n", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func registerRoutes(mux *http.ServeMux, store *mysql.Store) {
	// 认证
	mux.HandleFunc("/admin/api/v1/auth/login", handleLogin(store))

	// 用户管理
	mux.HandleFunc("/admin/api/v1/users", handleUsers(store))
	mux.HandleFunc("/admin/api/v1/users/", handleUserActions(store))

	// Skill 管理
	mux.HandleFunc("/admin/api/v1/skills", handleSkills(store))
	mux.HandleFunc("/admin/api/v1/skills/", handleSkillActions(store))

	// MCP 工具管理
	mux.HandleFunc("/admin/api/v1/tools", handleTools(store))
	mux.HandleFunc("/admin/api/v1/tools/", handleToolActions(store))

	// 统计
	mux.HandleFunc("/admin/api/v1/stats/overview", handleStatsOverview(store))
	mux.HandleFunc("/admin/api/v1/stats/daily", handleStatsDaily(store))

	// 系统配置
	mux.HandleFunc("/admin/api/v1/system/config", handleSystemConfig(store))
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}

func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 简化的认证检查
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			writeError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		// TODO: 验证 token
		next(w, r)
	}
}
