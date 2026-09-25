package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"dbmanager/internal/audit"
	"dbmanager/internal/auth"
	"dbmanager/internal/middleware"
	"dbmanager/internal/model"
)

// CreateUser 管理员创建用户
func CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		Role        string `json:"role"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}
	role := model.Role(req.Role)
	user, err := auth.CreateUser(req.Username, req.Password, req.Description, role)
	if err != nil {
		Error(w, 400, err.Error())
		return
	}
	admin := middleware.UserFromContext(r.Context())
	audit.GetLogger().Record(admin.Username, string(admin.Role), "create_user", "user",
		"创建用户: "+user.Username+" ("+string(user.Role)+")", middleware.ClientIP(r), middleware.UserAgent(r))
	Success(w, user.ToPublic())
}

// ListUsers 获取所有用户（管理员）
func ListUsers(w http.ResponseWriter, r *http.Request) {
	users := auth.GetStore().All()
	public := make([]model.PublicUser, 0, len(users))
	for _, u := range users {
		public = append(public, u.ToPublic())
	}
	Success(w, public)
}

// ApproveUser 审批通过用户
func ApproveUser(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	id = strings.TrimSuffix(id, "/approve")
	if id == "" {
		Error(w, 400, "用户ID不能为空")
		return
	}
	store := auth.GetStore()
	user := store.GetByID(id)
	if user == nil {
		Error(w, 404, "用户不存在")
		return
	}
	if user.Status != model.StatusPending {
		Error(w, 400, "该用户无需审批")
		return
	}
	if err := store.Update(id, func(u *model.User) {
		u.Status = model.StatusActive
	}); err != nil {
		Error(w, 500, "操作失败")
		return
	}
	admin := middleware.UserFromContext(r.Context())
	audit.GetLogger().Record(admin.Username, string(admin.Role), "approve_user", "user",
		"审批通过用户: "+user.Username, middleware.ClientIP(r), middleware.UserAgent(r))
	Success(w, "已通过审批")
}

// DisableUser 禁用用户
func DisableUser(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	id = strings.TrimSuffix(id, "/disable")
	if id == "" {
		Error(w, 400, "用户ID不能为空")
		return
	}
	store := auth.GetStore()
	user := store.GetByID(id)
	if user == nil {
		Error(w, 404, "用户不存在")
		return
	}
	if user.Role == model.RoleAdmin {
		Error(w, 400, "不能禁用管理员")
		return
	}
	if err := store.Update(id, func(u *model.User) {
		if u.Status == model.StatusDisabled {
			u.Status = model.StatusActive
		} else {
			u.Status = model.StatusDisabled
		}
	}); err != nil {
		Error(w, 500, "操作失败")
		return
	}
	// 禁用时清除该用户会话
	auth.GetSessionStore().DeleteByUser(id)
	admin := middleware.UserFromContext(r.Context())
	audit.GetLogger().Record(admin.Username, string(admin.Role), "disable_user", "user",
		"切换用户状态: "+user.Username, middleware.ClientIP(r), middleware.UserAgent(r))
	Success(w, "操作成功")
}

// ChangeUserRole 切换用户角色（管理员 <-> 普通用户）
func ChangeUserRole(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	id = strings.TrimSuffix(id, "/role")
	if id == "" {
		Error(w, 400, "用户ID不能为空")
		return
	}
	store := auth.GetStore()
	user := store.GetByID(id)
	if user == nil {
		Error(w, 404, "用户不存在")
		return
	}

	// 不能修改自己的角色
	admin := middleware.UserFromContext(r.Context())
	if admin.ID == id {
		Error(w, 400, "不能修改自己的角色")
		return
	}

	// 统计当前管理员数量，取消管理员时需保证至少保留一个管理员
	if user.Role == model.RoleAdmin {
		adminCount := 0
		for _, u := range store.All() {
			if u.Role == model.RoleAdmin {
				adminCount++
			}
		}
		if adminCount <= 1 {
			Error(w, 400, "至少需要保留一个管理员")
			return
		}
	}

	newRole := model.RoleUser
	if user.Role == model.RoleUser {
		newRole = model.RoleAdmin
	}
	if err := store.Update(id, func(u *model.User) {
		u.Role = newRole
	}); err != nil {
		Error(w, 500, "操作失败")
		return
	}
	// 角色变更后清除该用户会话，使其重新登录获取新权限
	auth.GetSessionStore().DeleteByUser(id)

	roleLabel := "管理员"
	if newRole == model.RoleUser {
		roleLabel = "普通用户"
	}
	audit.GetLogger().Record(admin.Username, string(admin.Role), "change_role", "user",
		"将用户 "+user.Username+" 角色改为 "+roleLabel, middleware.ClientIP(r), middleware.UserAgent(r))
	Success(w, "已将 "+user.Username+" 设置为 "+roleLabel)
}

// DeleteUser 删除用户
func DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	if id == "" {
		Error(w, 400, "用户ID不能为空")
		return
	}
	store := auth.GetStore()
	user := store.GetByID(id)
	if user == nil {
		Error(w, 404, "用户不存在")
		return
	}
	if user.Role == model.RoleAdmin {
		Error(w, 400, auth.ErrCannotDeleteAdmin.Error())
		return
	}
	if err := store.Delete(id); err != nil {
		Error(w, 500, "删除失败")
		return
	}
	auth.GetSessionStore().DeleteByUser(id)
	admin := middleware.UserFromContext(r.Context())
	audit.GetLogger().Record(admin.Username, string(admin.Role), "delete_user", "user",
		"删除用户: "+user.Username, middleware.ClientIP(r), middleware.UserAgent(r))
	Success(w, "删除成功")
}

// AuditLogs 查询审计日志
func AuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	username := q.Get("username")
	action := q.Get("action")
	limit := 200
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	logs, err := audit.GetLogger().Query(username, action, limit)
	if err != nil {
		Error(w, 500, "查询失败")
		return
	}
	Success(w, logs)
}

// OnlineUsers 在线用户统计
func OnlineUsers(w http.ResponseWriter, r *http.Request) {
	users := auth.GetSessionStore().OnlineUsers()
	Success(w, map[string]interface{}{
		"count": len(users),
		"users": users,
	})
}

// SystemConfig 获取系统配置（敏感信息脱敏）
func SystemConfig(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFromContext(r.Context())
	audit.GetLogger().Record(admin.Username, string(admin.Role), "view_config", "system",
		"查看系统配置", middleware.ClientIP(r), middleware.UserAgent(r))
	Success(w, map[string]interface{}{
		"port":                   "***",
		"data_dir":               "data",
		"connections_file":       "data/connections.json",
		"users_file":             "data/users.json",
		"audit_log_file":         "data/audit.log",
		"session_timeout_minutes": 120,
		"admin_username":         "admin",
	})
}
