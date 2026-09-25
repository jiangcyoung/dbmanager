package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"dbmanager/internal/auth"
	"dbmanager/internal/model"
)

type contextKey string

const userContextKey contextKey = "current_user"
const tokenContextKey contextKey = "current_token"

// UserFromContext 从请求上下文获取当前用户
func UserFromContext(ctx context.Context) *model.User {
	if u, ok := ctx.Value(userContextKey).(*model.User); ok {
		return u
	}
	return nil
}

// TokenFromContext 从请求上下文获取 token
func TokenFromContext(ctx context.Context) string {
	if t, ok := ctx.Value(tokenContextKey).(string); ok {
		return t
	}
	return ""
}

// extractToken 从请求头提取 token
func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return auth
}

// clientIP 获取客户端 IP
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}
	return r.RemoteAddr
}

// AuthMiddleware 认证中间件：校验 token 并注入用户
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			writeUnauthorized(w)
			return
		}
		user := auth.GetUserByToken(token)
		if user == nil {
			writeUnauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		ctx = context.WithValue(ctx, tokenContextKey, token)
		next(w, r.WithContext(ctx))
	}
}

// AdminMiddleware 管理员权限中间件（需先经过 AuthMiddleware）
func AdminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || user.Role != model.RoleAdmin {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(model.APIResponse{
				Code:    403,
				Message: "需要管理员权限",
			})
			return
		}
		next(w, r)
	}
}

// ClientIP 获取客户端 IP（导出）
func ClientIP(r *http.Request) string {
	return clientIP(r)
}

// UserAgent 获取 User-Agent
func UserAgent(r *http.Request) string {
	return r.Header.Get("User-Agent")
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(model.APIResponse{
		Code:    401,
		Message: "未登录或登录已过期",
	})
}
