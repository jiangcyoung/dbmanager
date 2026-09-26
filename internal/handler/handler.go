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

// canAccessConnection 判断当前用户是否有权访问连接（管理员可见全部，普通用户仅本人创建）
func canAccessConnection(r *http.Request, conn *model.DBConnection) bool {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		return false
	}
	if user.Role == model.RoleAdmin {
		return true
	}
	return conn.CreatedBy != "" && conn.CreatedBy == user.ID
}

// accessDenied 无权限响应
func accessDenied(w http.ResponseWriter) {
	Error(w, 403, "无权访问该连接")
}

// httpStatus 将业务 code 映射为 HTTP 状态码
func httpStatus(code int) int {
	switch code {
	case 0:
		return http.StatusOK
	case 400:
		return http.StatusBadRequest
	case 401:
		return http.StatusUnauthorized
	case 403:
		return http.StatusForbidden
	case 404:
		return http.StatusNotFound
	case 500:
		return http.StatusInternalServerError
	default:
		return http.StatusOK
	}
}

// Response 统一响应
func Response(w http.ResponseWriter, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus(code))
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

// GetConnections 获取所有连接（含在线状态）
func GetConnections(w http.ResponseWriter, r *http.Request) {
	cfg := config.GetConfig()
	conns := cfg.GetConnections()
	mgr := db.GetManager()
	user := middleware.UserFromContext(r.Context())
	result := make([]map[string]interface{}, 0, len(conns))
	for _, c := range conns {
		// 用户隔离：普通用户仅可见自己创建的连接
		if user == nil || (user.Role != model.RoleAdmin && (c.CreatedBy == "" || c.CreatedBy != user.ID)) {
			continue
		}
		online := mgr.IsConnected(c.ID)
		// 密码不返回前端
		c.Password = ""
		result = append(result, map[string]interface{}{
			"id":         c.ID,
			"name":       c.Name,
			"type":       c.Type,
			"host":       c.Host,
			"port":       c.Port,
			"username":   c.Username,
			"database":   c.Database,
			"file_path":  c.FilePath,
			"db_index":   c.DBIndex,
			"params":     c.Params,
			"created_by": c.CreatedBy,
			"online":     online,
		})
	}
	Success(w, result)
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
	if !canAccessConnection(r, conn) {
		accessDenied(w)
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
	if user := middleware.UserFromContext(r.Context()); user != nil {
		conn.CreatedBy = user.ID
	}

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
	if !canAccessConnection(r, oldConn) {
		accessDenied(w)
		return
	}
	// 如果密码为空，使用旧密码
	if conn.Password == "" {
		conn.Password = oldConn.Password
	}
	// 保留原创建者，防止被篡改
	conn.CreatedBy = oldConn.CreatedBy

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
	if conn == nil {
		Error(w, 404, "连接不存在")
		return
	}
	if !canAccessConnection(r, conn) {
		accessDenied(w)
		return
	}
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
			if !canAccessConnection(r, saved) {
				accessDenied(w)
				return
			}
			if conn.Password == "" {
				conn.Password = saved.Password
			}
		}
	}
	result := db.GetManager().TestConnection(conn)
	auditRecord(r, "test_connection", "connection", "测试连接: "+conn.Name)
	Success(w, result)
}

// ConnectConnection 显式连接（建立并放入连接池）
func ConnectConnection(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/connections/")
	id = strings.TrimSuffix(id, "/connect")
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
	if !canAccessConnection(r, conn) {
		accessDenied(w)
		return
	}
	mgr := db.GetManager()
	// 若已在线，无需重复连接
	if mgr.IsConnected(id) {
		Success(w, map[string]interface{}{"online": true, "message": "连接已在线"})
		return
	}
	if err := mgr.Connect(*conn); err != nil {
		Error(w, 500, "连接失败: "+err.Error())
		return
	}
	auditRecord(r, "connect", "connection", "连接: "+conn.Name)
	Success(w, map[string]interface{}{"online": true, "message": "连接成功"})
}

// DisconnectConnection 断开连接（从连接池移除）
func DisconnectConnection(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/connections/")
	id = strings.TrimSuffix(id, "/disconnect")
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
	if !canAccessConnection(r, conn) {
		accessDenied(w)
		return
	}
	db.GetManager().CloseConnection(id)
	auditRecord(r, "disconnect", "connection", "断开连接: "+conn.Name)
	Success(w, map[string]interface{}{"online": false, "message": "已断开"})
}

// ConnectionStatus 查询连接在线状态
func ConnectionStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/connections/")
	id = strings.TrimSuffix(id, "/status")
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
	if !canAccessConnection(r, conn) {
		accessDenied(w)
		return
	}
	online := db.GetManager().IsConnected(id)
	Success(w, map[string]interface{}{"id": id, "online": online})
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
	if !canAccessConnection(r, conn) {
		accessDenied(w)
		return
	}

	driver, err := db.GetManager().GetActiveConnection(conn.ID)
	if err != nil {
		if err == db.ErrNotConnected {
			Error(w, 400, err.Error())
		} else {
			Error(w, 500, "连接失败: "+err.Error())
		}
		return
	}

	tables, err := driver.ListTables("")
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
	if !canAccessConnection(r, conn) {
		accessDenied(w)
		return
	}

	driver, err := db.GetManager().GetActiveConnection(conn.ID)
	if err != nil {
		if err == db.ErrNotConnected {
			Error(w, 400, err.Error())
		} else {
			Error(w, 500, "连接失败: "+err.Error())
		}
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
