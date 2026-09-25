package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dbmanager/internal/audit"
	"dbmanager/internal/auth"
	"dbmanager/internal/config"
	"dbmanager/internal/db"
	"dbmanager/internal/handler"
	"dbmanager/internal/middleware"
	"dbmanager/internal/store"
)

func main() {
	cfg := config.GetConfig()

	if _, err := store.Init(cfg.DataDir); err != nil {
		log.Fatalf("初始化存储失败: %v", err)
	}

	auth.GetStore()
	audit.GetLogger()

	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.Port
	}

	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("./web"))
	mux.Handle("/", fs)

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/api/auth/login", handler.Login)
	mux.HandleFunc("/api/auth/register", handler.Register)

	authWrap := middleware.AuthMiddleware

	mux.HandleFunc("/api/auth/logout", authWrap(handler.Logout))
	mux.HandleFunc("/api/auth/me", authWrap(handler.Me))
	mux.HandleFunc("/api/auth/change-password", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			authWrap(handler.ChangePassword)(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/connections", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authWrap(handler.GetConnections)(w, r)
		case http.MethodPost:
			authWrap(handler.AddConnection)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/connections/", func(w http.ResponseWriter, r *http.Request) {
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

		// 连接操作：/connect, /disconnect, /status
		if strings.HasSuffix(path, "/connect") {
			if r.Method == http.MethodPost {
				authWrap(handler.ConnectConnection)(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.HasSuffix(path, "/disconnect") {
			if r.Method == http.MethodPost {
				authWrap(handler.DisconnectConnection)(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.HasSuffix(path, "/status") {
			if r.Method == http.MethodGet {
				authWrap(handler.ConnectionStatus)(w, r)
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

	mux.HandleFunc("/api/quick/", func(w http.ResponseWriter, r *http.Request) {
		h := authWrap(func(w http.ResponseWriter, r *http.Request) {
			quickRoute(w, r)
		})
		h(w, r)
	})

	adminWrap := func(h http.HandlerFunc) http.HandlerFunc {
		return authWrap(middleware.AdminMiddleware(h))
	}

	mux.HandleFunc("/api/admin/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			adminWrap(handler.ListUsers)(w, r)
		case http.MethodPost:
			adminWrap(handler.CreateUser)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/admin/users/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/approve") && r.Method == http.MethodPut:
			adminWrap(handler.ApproveUser)(w, r)
		case strings.HasSuffix(path, "/disable") && r.Method == http.MethodPut:
			adminWrap(handler.DisableUser)(w, r)
		case strings.HasSuffix(path, "/role") && r.Method == http.MethodPut:
			adminWrap(handler.ChangeUserRole)(w, r)
		case strings.HasSuffix(path, "/reset-password") && r.Method == http.MethodPut:
			adminWrap(handler.ResetPassword)(w, r)
		case r.Method == http.MethodDelete:
			adminWrap(handler.DeleteUser)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/admin/audit-logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			adminWrap(handler.AuditLogs)(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/admin/online-users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			adminWrap(handler.OnlineUsers)(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/admin/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			adminWrap(handler.SystemConfig)(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	var h http.Handler = mux
	h = requestLogging(h)
	h = corsMiddleware(h)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      h,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("服务器启动，监听端口 %s", port)
		log.Printf("Web UI: http://localhost:%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("正在关闭服务器...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("服务器关闭异常: %v", err)
	}

	audit.GetLogger().Flush()
	db.GetManager().CloseAll()
	store.Get().Close()
	log.Println("服务器已关闭")
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func requestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	var originSet map[string]bool
	if allowedOrigins != "" {
		originSet = make(map[string]bool)
		for _, o := range strings.Split(allowedOrigins, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				originSet[o] = true
			}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if originSet != nil {
			if originSet[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func quickRoute(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
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
