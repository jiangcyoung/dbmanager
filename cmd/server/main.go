package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"dbmanager/internal/config"
	"dbmanager/internal/db"
	"dbmanager/internal/handler"
)

func main() {
	cfg := config.GetConfig()

	// 从环境变量获取端口
	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.Port
	}

	// 静态文件服务
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	// API路由
	http.HandleFunc("/api/connections", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.GetConnections(w, r)
		case http.MethodPost:
			handler.AddConnection(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/connections/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// /api/connections/test
		if path == "/api/connections/test" {
			if r.Method == http.MethodPost {
				handler.TestConnection(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// /api/connections/query
		if path == "/api/connections/query" {
			if r.Method == http.MethodPost {
				handler.ExecuteQuery(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// /api/connections/{id}/tables
		if len(path) > len("/api/connections/") && path[len(path)-7:] == "/tables" {
			if r.Method == http.MethodGet {
				handler.ListTables(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// /api/connections/{id}
		switch r.Method {
		case http.MethodGet:
			handler.GetConnection(w, r)
		case http.MethodPut:
			handler.UpdateConnection(w, r)
		case http.MethodDelete:
			handler.DeleteConnection(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 快捷操作路由
	http.HandleFunc("/api/quick/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
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
	})

	// CORS中间件
	handler := corsMiddleware(http.DefaultServeMux)

	log.Printf("服务器启动，监听端口 %s", port)
	log.Printf("Web UI: http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}

	// 关闭所有数据库连接
	db.GetManager().CloseAll()
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
