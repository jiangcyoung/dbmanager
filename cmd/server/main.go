package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"dbmanager/internal/audit"
	"dbmanager/internal/auth"
	"dbmanager/internal/config"
	"dbmanager/internal/db"
	"dbmanager/internal/handler"
	"dbmanager/internal/middleware"
)

func main() {
	cfg := config.GetConfig()

	// 初始化用户存储（确保管理员存在）
	auth.GetStore()
	// 初始化审计日志
	audit.GetLogger()

	// 从环境变量获取端口
	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.Port
	}

	// 静态文件服务
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	// ========== 公开路由（无需登录） ==========
	http.HandleFunc("/api/auth/login", handler.Login)
	http.HandleFunc("/api/auth/register", handler.Register)

	// ========== 需登录路由 ==========
	authWrap := middleware.AuthMiddleware

	http.HandleFunc("/api/auth/logout", authWrap(handler.Logout))
	http.HandleFunc("/api/auth/me", authWrap(handler.Me))

	// 连接管理
	http.HandleFunc("/api/connections", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authWrap(handler.GetConnections)(w, r)
		case http.MethodPost:
			authWrap(handler.AddConnection)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/connections/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/api/connections/test" {
			if r.Method == http.MethodPost {
				authWrap(handler.TestConnection)(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if path == "/api/connections/query" {
			if r.Method == http.MethodPost {
				authWrap(handler.ExecuteQuery)(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if len(path) > len("/api/connections/") && path[len(path)-7:] == "/tables" {
			if r.Method == http.MethodGet {
				authWrap(handler.ListTables)(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		switch r.Method {
		case http.MethodGet:
			authWrap(handler.GetConnection)(w, r)
		case http.MethodPut:
			authWrap(handler.UpdateConnection)(w, r)
		case http.MethodDelete:
			authWrap(handler.DeleteConnection)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 快捷操作
	http.HandleFunc("/api/quick/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		h := authWrap(func(w http.ResponseWriter, r *http.Request) {
			quickRoute(w, r)
		})
		h(w, r)
		_ = path
	})

	// ========== 管理员路由 ==========
	adminWrap := func(h http.HandlerFunc) http.HandlerFunc {
		return authWrap(middleware.AdminMiddleware(h))
	}

	http.HandleFunc("/api/admin/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			adminWrap(handler.ListUsers)(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	http.HandleFunc("/api/admin/users/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/approve") && r.Method == http.MethodPut:
			adminWrap(handler.ApproveUser)(w, r)
		case strings.HasSuffix(path, "/disable") && r.Method == http.MethodPut:
			adminWrap(handler.DisableUser)(w, r)
		case strings.HasSuffix(path, "/role") && r.Method == http.MethodPut:
			adminWrap(handler.ChangeUserRole)(w, r)
		case r.Method == http.MethodDelete:
			adminWrap(handler.DeleteUser)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/admin/audit-logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			adminWrap(handler.AuditLogs)(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	http.HandleFunc("/api/admin/online-users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			adminWrap(handler.OnlineUsers)(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	http.HandleFunc("/api/admin/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			adminWrap(handler.SystemConfig)(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// CORS中间件
	h := corsMiddleware(http.DefaultServeMux)

	log.Printf("服务器启动，监听端口 %s", port)
	log.Printf("Web UI: http://localhost:%s", port)
	log.Printf("默认管理员账号: %s / %s", cfg.Admin.Username, cfg.Admin.Password)
	if err := http.ListenAndServe(":"+port, h); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}

	// 关闭所有数据库连接
	db.GetManager().CloseAll()
}

// quickRoute 快捷操作路由分发
func quickRoute(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	// ========== Redis 优化操作 ==========
	case strings.Contains(path, "/redis/keys") && r.Method == http.MethodGet:
		handler.RedisListKeys(w, r)
	case strings.Contains(path, "/redis/key") && r.Method == http.MethodGet:
		handler.RedisGetKey(w, r)
	case strings.Contains(path, "/redis/key") && r.Method == http.MethodPost:
		handler.RedisSetKey(w, r)
	case strings.Contains(path, "/redis/key") && r.Method == http.MethodDelete:
		handler.RedisDeleteKey(w, r)
	case strings.Contains(path, "/redis/hash") && r.Method == http.MethodPost:
		handler.RedisSetHash(w, r)
	case strings.Contains(path, "/redis/list") && r.Method == http.MethodPost:
		handler.RedisPushList(w, r)
	case strings.Contains(path, "/redis/set") && r.Method == http.MethodPost:
		handler.RedisAddSet(w, r)
	case strings.Contains(path, "/redis/zset") && r.Method == http.MethodPost:
		handler.RedisAddZSet(w, r)
	case strings.Contains(path, "/redis/ttl") && r.Method == http.MethodPut:
		handler.RedisSetTTL(w, r)
	case strings.Contains(path, "/redis/info") && r.Method == http.MethodGet:
		handler.RedisInfo(w, r)
	case strings.Contains(path, "/redis/dbsize") && r.Method == http.MethodGet:
		handler.RedisDBSize(w, r)
	case strings.Contains(path, "/redis/flushdb") && r.Method == http.MethodPost:
		handler.RedisFlushDB(w, r)

	// ========== MongoDB 优化操作 ==========
	case strings.Contains(path, "/mongo/aggregate") && r.Method == http.MethodPost:
		handler.MongoAggregate(w, r)
	case strings.Contains(path, "/mongo/count") && r.Method == http.MethodGet:
		handler.MongoCount(w, r)
	case strings.Contains(path, "/mongo/distinct") && r.Method == http.MethodGet:
		handler.MongoDistinct(w, r)
	case strings.Contains(path, "/mongo/stats/collection") && r.Method == http.MethodGet:
		handler.MongoCollectionStats(w, r)
	case strings.Contains(path, "/mongo/stats/database") && r.Method == http.MethodGet:
		handler.MongoDatabaseStats(w, r)
	case strings.Contains(path, "/mongo/find") && r.Method == http.MethodPost:
		handler.MongoFindWithOptions(w, r)

	// ========== PostgreSQL 优化操作 ==========
	case strings.Contains(path, "/pg/schemas") && r.Method == http.MethodGet:
		handler.PGListSchemas(w, r)
	case strings.Contains(path, "/pg/schema") && r.Method == http.MethodPost:
		handler.PGCreateSchema(w, r)
	case strings.Contains(path, "/pg/schema") && r.Method == http.MethodDelete:
		handler.PGDropSchema(w, r)
	case strings.Contains(path, "/pg/explain") && r.Method == http.MethodPost:
		handler.PGExplain(w, r)
	case strings.Contains(path, "/pg/vacuum") && r.Method == http.MethodPost:
		handler.PGVacuum(w, r)
	case strings.Contains(path, "/pg/analyze") && r.Method == http.MethodPost:
		handler.PGAnalyze(w, r)
	case strings.Contains(path, "/pg/reindex") && r.Method == http.MethodPost:
		handler.PGReindex(w, r)
	case strings.Contains(path, "/pg/sizes") && r.Method == http.MethodGet:
		handler.PGTableSizes(w, r)
	case strings.Contains(path, "/pg/sequences") && r.Method == http.MethodGet:
		handler.PGSequences(w, r)

	// ========== SQLite 优化操作 ==========
	case strings.Contains(path, "/sqlite/pragmas") && r.Method == http.MethodGet:
		handler.SQLiteGetPragmas(w, r)
	case strings.Contains(path, "/sqlite/pragma") && r.Method == http.MethodPut:
		handler.SQLiteSetPragma(w, r)
	case strings.Contains(path, "/sqlite/vacuum") && r.Method == http.MethodPost:
		handler.SQLiteVacuum(w, r)
	case strings.Contains(path, "/sqlite/reindex") && r.Method == http.MethodPost:
		handler.SQLiteReindex(w, r)
	case strings.Contains(path, "/sqlite/explain") && r.Method == http.MethodPost:
		handler.SQLiteExplain(w, r)
	case strings.Contains(path, "/sqlite/version") && r.Method == http.MethodGet:
		handler.SQLiteVersion(w, r)

	// ========== 原有快捷操作 ==========
	case strings.HasSuffix(path, "/databases") && r.Method == http.MethodGet:
		handler.QuickListDatabases(w, r)
	case strings.HasSuffix(path, "/database") && r.Method == http.MethodPost:
		handler.QuickCreateDatabase(w, r)
	case strings.HasSuffix(path, "/database") && r.Method == http.MethodDelete:
		handler.QuickDropDatabase(w, r)
	case strings.HasSuffix(path, "/tables") && r.Method == http.MethodGet:
		handler.QuickListTables(w, r)
	case strings.HasSuffix(path, "/table/rename") && r.Method == http.MethodPut:
		handler.QuickRenameTable(w, r)
	case strings.HasSuffix(path, "/table") && r.Method == http.MethodPost:
		handler.QuickCreateTable(w, r)
	case strings.HasSuffix(path, "/table") && r.Method == http.MethodDelete:
		handler.QuickDropTable(w, r)
	case strings.HasSuffix(path, "/columns") && r.Method == http.MethodGet:
		handler.QuickGetColumns(w, r)
	case strings.HasSuffix(path, "/indexes") && r.Method == http.MethodGet:
		handler.QuickListIndexes(w, r)
	case strings.HasSuffix(path, "/index") && r.Method == http.MethodPost:
		handler.QuickCreateIndex(w, r)
	case strings.HasSuffix(path, "/index") && r.Method == http.MethodDelete:
		handler.QuickDropIndex(w, r)
	case strings.HasSuffix(path, "/rows") && r.Method == http.MethodGet:
		handler.QuickListRows(w, r)
	case strings.HasSuffix(path, "/row") && r.Method == http.MethodPost:
		handler.QuickInsertRow(w, r)
	case strings.HasSuffix(path, "/row") && r.Method == http.MethodPut:
		handler.QuickUpdateRows(w, r)
	case strings.HasSuffix(path, "/row") && r.Method == http.MethodDelete:
		handler.QuickDeleteRows(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// corsMiddleware CORS中间件
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
