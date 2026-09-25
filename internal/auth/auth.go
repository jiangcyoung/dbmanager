package auth

import (
	"errors"
	"time"

	"dbmanager/internal/model"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// 错误定义
var (
	ErrUserNotFound    = errors.New("用户不存在")
	ErrWrongPassword   = errors.New("用户名或密码错误")
	ErrUserDisabled    = errors.New("账号已被禁用")
	ErrUserPending     = errors.New("账号待审批，请联系管理员")
	ErrUserExists      = errors.New("用户名已存在")
	ErrInvalidUsername = errors.New("用户名不能为空")
	ErrInvalidPassword = errors.New("密码长度至少6位")
	ErrCannotDeleteAdmin = errors.New("不能删除管理员账号")
)

// Login 登录
func Login(username, password, ip string) (string, *model.User, error) {
	store := GetStore()
	user := store.GetByUsername(username)
	if user == nil {
		return "", nil, ErrWrongPassword
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", nil, ErrWrongPassword
	}
	if user.Status == model.StatusDisabled {
		return "", nil, ErrUserDisabled
	}
	if user.Status == model.StatusPending {
		return "", nil, ErrUserPending
	}

	// 更新最后登录时间
	now := time.Now()
	_ = store.Update(user.ID, func(u *model.User) {
		u.LastLoginAt = &now
	})
	user.LastLoginAt = &now

	token := GetSessionStore().Create(*user, ip)
	return token, user, nil
}

// Register 注册（普通用户，状态为待审批）
func Register(username, password string) error {
	if username == "" {
		return ErrInvalidUsername
	}
	if len(password) < 6 {
		return ErrInvalidPassword
	}
	store := GetStore()
	if store.GetByUsername(username) != nil {
		return ErrUserExists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u := model.User{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: string(hash),
		Role:         model.RoleUser,
		Status:       model.StatusPending,
		CreatedAt:    time.Now(),
	}
	return store.Create(u)
}

// Logout 登出
func Logout(token string) {
	GetSessionStore().Delete(token)
}

// CreateUser 管理员创建用户（可指定角色、描述，状态默认激活）
func CreateUser(username, password, description string, role model.Role) (*model.User, error) {
	if username == "" {
		return nil, ErrInvalidUsername
	}
	if len(password) < 6 {
		return nil, ErrInvalidPassword
	}
	if role != model.RoleAdmin && role != model.RoleUser {
		role = model.RoleUser
	}
	store := GetStore()
	if store.GetByUsername(username) != nil {
		return nil, ErrUserExists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := model.User{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
		Status:       model.StatusActive,
		Description:  description,
		CreatedAt:    time.Now(),
	}
	if err := store.Create(u); err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByToken 根据 token 获取用户
func GetUserByToken(token string) *model.User {
	sess := GetSessionStore().Get(token)
	if sess == nil {
		return nil
	}
	return &sess.User
}

// ChangePassword 用户修改自己的密码（需验证旧密码）
func ChangePassword(userID, oldPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return ErrInvalidPassword
	}
	store := GetStore()
	user := store.GetByID(userID)
	if user == nil {
		return ErrUserNotFound
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrWrongPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return store.Update(userID, func(u *model.User) {
		u.PasswordHash = string(hash)
	})
}

// ResetPassword 管理员重置用户密码（无需旧密码）
func ResetPassword(adminID, targetUserID, newPassword string) error {
	if len(newPassword) < 6 {
		return ErrInvalidPassword
	}
	store := GetStore()
	target := store.GetByID(targetUserID)
	if target == nil {
		return ErrUserNotFound
	}
	if target.Role == model.RoleAdmin {
		return errors.New("不能重置管理员密码")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return store.Update(targetUserID, func(u *model.User) {
		u.PasswordHash = string(hash)
	})
}
