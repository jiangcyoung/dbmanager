package handler

import (
	"encoding/json"
	"net/http"

	"dbmanager/internal/audit"
	"dbmanager/internal/auth"
	"dbmanager/internal/middleware"
)

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login 登录
func Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	ip := middleware.ClientIP(r)
	ua := middleware.UserAgent(r)

	token, user, err := auth.Login(req.Username, req.Password, ip)
	if err != nil {
		audit.GetLogger().Record(req.Username, "", "login_failed", "auth", err.Error(), ip, ua)
		Error(w, 401, err.Error())
		return
	}

	audit.GetLogger().Record(user.Username, string(user.Role), "login", "auth", "登录成功", ip, ua)

	Success(w, map[string]interface{}{
		"token": token,
		"user":  user.ToPublic(),
	})
}

// Register 注册
func Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	ip := middleware.ClientIP(r)
	ua := middleware.UserAgent(r)

	if err := auth.Register(req.Username, req.Password); err != nil {
		audit.GetLogger().Record(req.Username, "", "register_failed", "auth", err.Error(), ip, ua)
		Error(w, 400, err.Error())
		return
	}

	audit.GetLogger().Record(req.Username, "user", "register", "auth", "用户注册，待审批", ip, ua)
	Success(w, "注册成功，请等待管理员审批")
}

// Logout 登出
func Logout(w http.ResponseWriter, r *http.Request) {
	token := middleware.TokenFromContext(r.Context())
	user := middleware.UserFromContext(r.Context())
	if user != nil {
		audit.GetLogger().Record(user.Username, string(user.Role), "logout", "auth", "登出", middleware.ClientIP(r), middleware.UserAgent(r))
	}
	if token != "" {
		auth.Logout(token)
	}
	Success(w, "已退出登录")
}

// Me 获取当前用户信息
func Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		Error(w, 401, "未登录")
		return
	}
	Success(w, user.ToPublic())
}

// ChangePassword 用户修改自己的密码
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		Error(w, 401, "未登录")
		return
	}
	ip := middleware.ClientIP(r)
	ua := middleware.UserAgent(r)

	if err := auth.ChangePassword(user.ID, req.OldPassword, req.NewPassword); err != nil {
		audit.GetLogger().Record(user.Username, string(user.Role), "change_password_failed", "auth", err.Error(), ip, ua)
		Error(w, 400, err.Error())
		return
	}
	audit.GetLogger().Record(user.Username, string(user.Role), "change_password", "auth", "修改密码成功", ip, ua)
	Success(w, "密码修改成功")
}
