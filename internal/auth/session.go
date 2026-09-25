package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"dbmanager/internal/config"
	"dbmanager/internal/model"
)

// Session 会话信息
type Session struct {
	Token      string
	User       model.User
	LoginAt    time.Time
	LastActive time.Time
	IP         string
}

// SessionStore 会话存储（内存）
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

var (
	sessOnce sync.Once
	sess     *SessionStore
)

// GetSessionStore 获取会话存储单例
func GetSessionStore() *SessionStore {
	sessOnce.Do(func() {
		sess = &SessionStore{
			sessions: make(map[string]*Session),
		}
	})
	return sess
}

// generateToken 生成随机 token
func generateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Create 创建会话
func (s *SessionStore) Create(user model.User, ip string) string {
	token := generateToken()
	s.mu.Lock()
	s.sessions[token] = &Session{
		Token:      token,
		User:       user,
		LoginAt:    time.Now(),
		LastActive: time.Now(),
		IP:         ip,
	}
	s.mu.Unlock()
	return token
}

// Get 获取会话（同时更新活跃时间）
func (s *SessionStore) Get(token string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok {
		return nil
	}
	cfg := config.GetConfig()
	timeout := time.Duration(cfg.SessionTimeoutMin) * time.Minute
	if time.Since(sess.LastActive) > timeout {
		delete(s.sessions, token)
		return nil
	}
	sess.LastActive = time.Now()
	return sess
}

// Delete 删除会话
func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// DeleteByUser 删除某用户的所有会话
func (s *SessionStore) DeleteByUser(userID string) {
	s.mu.Lock()
	for t, sess := range s.sessions {
		if sess.User.ID == userID {
			delete(s.sessions, t)
		}
	}
	s.mu.Unlock()
}

// OnlineUsers 获取在线用户列表
func (s *SessionStore) OnlineUsers() []model.OnlineUser {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := config.GetConfig()
	timeout := time.Duration(cfg.SessionTimeoutMin) * time.Minute
	result := make([]model.OnlineUser, 0, len(s.sessions))
	seen := make(map[string]bool)
	for _, sess := range s.sessions {
		if time.Since(sess.LastActive) > timeout {
			continue
		}
		if seen[sess.User.Username] {
			continue
		}
		seen[sess.User.Username] = true
		result = append(result, model.OnlineUser{
			Username:   sess.User.Username,
			Role:       string(sess.User.Role),
			LoginAt:    sess.LoginAt,
			LastActive: sess.LastActive,
			IP:         sess.IP,
		})
	}
	return result
}

// Count 在线用户数
func (s *SessionStore) Count() int {
	return len(s.OnlineUsers())
}
