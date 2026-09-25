package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"dbmanager/internal/audit"
	"dbmanager/internal/config"
	"dbmanager/internal/db"
	"dbmanager/internal/middleware"
	"dbmanager/internal/model"

	"github.com/google/uuid"
)

// auditRecord 记录当前用户的操作审计日志
func auditRecord(r *http.Request, action, resource, detail string) {
	user := middleware.UserFromContext(r.Context())
	username, role := "anonymous", ""
	if user != nil {
		username = user.Username
		role = string(user.Role)
	}
	audit.GetLogger().Record(username, role, action, resource, detail, middleware.ClientIP(r), middleware.UserAgent(r))
}

// Response 统一响应
func Response(w http.ResponseWriter, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(model.APIResponse{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// Success 成功响应
func Success(w http.ResponseWriter, data interface{}) {
	Response(w, 0, "success", data)
}

// Error 错误响应
func Error(w http.ResponseWriter, code int, message string) {
	Response(w, code, message, nil)
}

// GetConnections 获取所有连接
func GetConnections(w http.ResponseWriter, r *http.Request) {
	cfg := config.GetConfig()
	conns := cfg.GetConnections()
	Success(w, conns)
}

// GetConnection 获取单个连接
func GetConnection(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/connections/")
	if id == "" {
		Error(w, 400, "连接ID不能为空")
		return
	}
	cfg := config.GetConfig()
	conn := cfg.GetConnectionByID(id)
	if conn == nil {
		Error(w, 404, "连接不存在")
		return
	}
	conn.Password = ""
	Success(w, conn)
}

// AddConnection 添加连接
func AddConnection(w http.ResponseWriter, r *http.Request) {
	var conn model.DBConnection
	if err := json.NewDecoder(r.Body).Decode(&conn); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}
	if conn.Name == "" {
		Error(w, 400, "连接名称不能为空")
		return
	}
	if conn.Type == "" {
		Error(w, 400, "数据库类型不能为空")
		return
	}
	conn.ID = uuid.New().String()

	cfg := config.GetConfig()
	if err := cfg.AddConnection(conn); err != nil {
		Error(w, 500, "保存失败: "+err.Error())
		return
	}
	auditRecord(r, "create_connection", "connection", "创建连接: "+conn.Name+" ("+string(conn.Type)+")")
	Success(w, conn.ID)
}

// UpdateConnection 更新连接
func UpdateConnection(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/connections/")
	if id == "" {
		Error(w, 400, "连接ID不能为空")
		return
	}
	var conn model.DBConnection
	if err := json.NewDecoder(r.Body).Decode(&conn); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}

	cfg := config.GetConfig()
	oldConn := cfg.GetConnectionByID(id)
	if oldConn == nil {
		Error(w, 404, "连接不存在")
		return
	}
	// 如果密码为空，使用旧密码
	if conn.Password == "" {
		conn.Password = oldConn.Password
	}

	if err := cfg.UpdateConnection(id, conn); err != nil {
		Error(w, 500, "更新失败: "+err.Error())
		return
	}
	// 关闭旧连接
	db.GetManager().CloseConnection(id)
	auditRecord(r, "update_connection", "connection", "更新连接: "+conn.Name)
	Success(w, nil)
}

// DeleteConnection 删除连接
func DeleteConnection(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/connections/")
	if id == "" {
		Error(w, 400, "连接ID不能为空")
		return
	}
	cfg := config.GetConfig()
	conn := cfg.GetConnectionByID(id)
	if err := cfg.DeleteConnection(id); err != nil {
		Error(w, 500, "删除失败: "+err.Error())
		return
	}
	db.GetManager().CloseConnection(id)
	name := ""
	if conn != nil {
		name = conn.Name
	}
	auditRecord(r, "delete_connection", "connection", "删除连接: "+name)
	Success(w, nil)
}

// TestConnection 测试连接
func TestConnection(w http.ResponseWriter, r *http.Request) {
	var conn model.DBConnection
	if err := json.NewDecoder(r.Body).Decode(&conn); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}
	// 如果是已保存的连接，从配置中获取完整信息（含密码）
	if conn.ID != "" {
		cfg := config.GetConfig()
		saved := cfg.GetConnectionByID(conn.ID)
		if saved != nil {
			if conn.Password == "" {
				conn.Password = saved.Password
			}
		}
	}
	result := db.GetManager().TestConnection(conn)
	auditRecord(r, "test_connection", "connection", "测试连接: "+conn.Name)
	Success(w, result)
}

// ListTables 列出所有表
func ListTables(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/connections/")
	id = strings.TrimSuffix(id, "/tables")
	if id == "" {
		Error(w, 400, "连接ID不能为空")
		return
	}
	cfg := config.GetConfig()
	conn := cfg.GetConnectionByID(id)
	if conn == nil {
		Error(w, 404, "连接不存在")
		return
	}

	driver, err := db.GetManager().GetConnection(*conn)
	if err != nil {
		Error(w, 500, "连接失败: "+err.Error())
		return
	}

	tables, err := driver.ListTables()
	if err != nil {
		Error(w, 500, "获取表列表失败: "+err.Error())
		return
	}
	if tables == nil {
		tables = []string{}
	}
	Success(w, tables)
}

// ExecuteQuery 执行查询
func ExecuteQuery(w http.ResponseWriter, r *http.Request) {
	var req model.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}
	if req.ConnectionID == "" {
		Error(w, 400, "连接ID不能为空")
		return
	}
	if req.SQL == "" {
		Error(w, 400, "查询语句不能为空")
		return
	}

	cfg := config.GetConfig()
	conn := cfg.GetConnectionByID(req.ConnectionID)
	if conn == nil {
		Error(w, 404, "连接不存在")
		return
	}

	driver, err := db.GetManager().GetConnection(*conn)
	if err != nil {
		Error(w, 500, "连接失败: "+err.Error())
		return
	}

	// 判断是查询还是执行
	sqlTrimmed := strings.TrimSpace(strings.ToUpper(req.SQL))
	isSelect := strings.HasPrefix(sqlTrimmed, "SELECT") || strings.HasPrefix(sqlTrimmed, "SHOW") ||
		strings.HasPrefix(sqlTrimmed, "DESC") || strings.HasPrefix(sqlTrimmed, "DESCRIBE") ||
		strings.HasPrefix(sqlTrimmed, "EXPLAIN")

	var result *model.QueryResult
	if isSelect || conn.Type == model.DBTypeMongo || conn.Type == model.DBTypeRedis {
		result, err = driver.Query(req.SQL, req.Collection)
	} else {
		result, err = driver.Execute(req.SQL, req.Collection)
	}

	if err != nil {
		Error(w, 500, "执行失败: "+err.Error())
		return
	}
	// 审计：记录 SQL 执行（截断过长的 SQL）
	sqlPreview := req.SQL
	if len(sqlPreview) > 200 {
		sqlPreview = sqlPreview[:200] + "..."
	}
	auditRecord(r, "execute_query", "query", sqlPreview)
	Success(w, result)
}
