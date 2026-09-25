package auth

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"dbmanager/internal/config"
	"dbmanager/internal/model"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// UserStore 用户存储（基于 JSON 文件）
type UserStore struct {
	mu    sync.RWMutex
	users []model.User
}

var (
	storeOnce sync.Once
	store     *UserStore
)

// GetStore 获取用户存储单例
func GetStore() *UserStore {
	storeOnce.Do(func() {
		store = &UserStore{}
		store.load()
		store.ensureAdmin()
	})
	return store
}

// load 从文件加载用户
func (s *UserStore) load() {
	cfg := config.GetConfig()
	data, err := os.ReadFile(cfg.UsersFile)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &s.users)
}

// save 保存用户到文件
func (s *UserStore) save() error {
	cfg := config.GetConfig()
	_ = os.MkdirAll(cfg.DataDir, 0755)
	data, err := json.MarshalIndent(s.users, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfg.UsersFile, data, 0644)
}

// ensureAdmin 确保管理员用户存在
func (s *UserStore) ensureAdmin() {
	cfg := config.GetConfig()
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, u := range s.users {
		if u.Role == model.RoleAdmin {
			// 管理员已存在，检查是否需要更新密码
			if cfg.Admin.Username != "" && u.Username != cfg.Admin.Username {
				continue
			}
			return
		}
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(cfg.Admin.Password), bcrypt.DefaultCost)
	admin := model.User{
		ID:           uuid.New().String(),
		Username:     cfg.Admin.Username,
		PasswordHash: string(hash),
		Role:         model.RoleAdmin,
		Status:       model.StatusActive,
		CreatedAt:    time.Now(),
	}
	s.users = append(s.users, admin)
	_ = s.save()
}

// GetByUsername 根据用户名查找用户
func (s *UserStore) GetByUsername(username string) *model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.users {
		if s.users[i].Username == username {
			u := s.users[i]
			return &u
		}
	}
	return nil
}

// GetByID 根据ID查找用户
func (s *UserStore) GetByID(id string) *model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.users {
		if s.users[i].ID == id {
			u := s.users[i]
			return &u
		}
	}
	return nil
}

// All 获取所有用户
func (s *UserStore) All() []model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.User, len(s.users))
	copy(result, s.users)
	return result
}

// Create 创建用户
func (s *UserStore) Create(u model.User) error {
	s.mu.Lock()
	s.users = append(s.users, u)
	s.mu.Unlock()
	return s.save()
}

// Update 更新用户
func (s *UserStore) Update(id string, fn func(*model.User)) error {
	s.mu.Lock()
	for i := range s.users {
		if s.users[i].ID == id {
			fn(&s.users[i])
			break
		}
	}
	s.mu.Unlock()
	return s.save()
}

// Delete 删除用户
func (s *UserStore) Delete(id string) error {
	s.mu.Lock()
	for i, u := range s.users {
		if u.ID == id {
			s.users = append(s.users[:i], s.users[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
	return s.save()
}
