package audit

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"dbmanager/internal/config"
	"dbmanager/internal/model"

	"github.com/google/uuid"
)

// Logger 审计日志记录器（基于 JSON Lines 文件）
type Logger struct {
	mu sync.Mutex
}

var (
	loggerOnce sync.Once
	logger     *Logger
)

// GetLogger 获取日志记录器单例
func GetLogger() *Logger {
	loggerOnce.Do(func() {
		logger = &Logger{}
	})
	return logger
}

// Record 记录一条审计日志
func (l *Logger) Record(username, role, action, resource, detail, ip, userAgent string) {
	cfg := config.GetConfig()
	_ = os.MkdirAll(cfg.DataDir, 0755)

	entry := model.AuditLog{
		ID:        uuid.New().String(),
		Time:      time.Now(),
		Username:  username,
		Role:      role,
		Action:    action,
		Resource:  resource,
		Detail:    detail,
		IP:        ip,
		UserAgent: userAgent,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.OpenFile(cfg.AuditLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}

// Query 查询审计日志（支持用户名、动作关键字筛选，返回最近 limit 条）
func (l *Logger) Query(username, action string, limit int) ([]model.AuditLog, error) {
	cfg := config.GetConfig()
	data, err := os.ReadFile(cfg.AuditLogFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.AuditLog{}, nil
		}
		return nil, err
	}

	var logs []model.AuditLog
	lines := splitLines(data)
	// 从后往前解析
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if len(line) == 0 {
			continue
		}
		var entry model.AuditLog
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if username != "" && entry.Username != username {
			continue
		}
		if action != "" && !containsFold(entry.Action, action) {
			continue
		}
		logs = append(logs, entry)
		if limit > 0 && len(logs) >= limit {
			break
		}
	}
	if logs == nil {
		logs = []model.AuditLog{}
	}
	return logs, nil
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

func containsFold(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	ls := toLower(s)
	lsub := toLower(sub)
	for i := 0; i+len(lsub) <= len(ls); i++ {
		if ls[i:i+len(lsub)] == lsub {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
