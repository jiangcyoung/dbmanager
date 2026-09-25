package audit

import (
	"database/sql"
	"sync"
	"time"

	"dbmanager/internal/model"
	"dbmanager/internal/store"

	"github.com/google/uuid"
)

const (
	maxBufferSize = 100
	flushInterval = 5 * time.Second
)

type Logger struct {
	mu     sync.Mutex
	buffer []model.AuditLog
	done   chan struct{}
}

var (
	loggerOnce sync.Once
	loggerInst *Logger
)

func GetLogger() *Logger {
	loggerOnce.Do(func() {
		loggerInst = &Logger{
			done: make(chan struct{}),
		}
		go loggerInst.flushLoop()
	})
	return loggerInst
}

func (l *Logger) flushLoop() {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.flush()
		case <-l.done:
			l.flush()
			return
		}
	}
}

func (l *Logger) Record(username, role, action, resource, detail, ip, userAgent string) {
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

	l.mu.Lock()
	l.buffer = append(l.buffer, entry)
	shouldFlush := len(l.buffer) >= maxBufferSize
	l.mu.Unlock()

	if shouldFlush {
		l.flush()
	}
}

func (l *Logger) flush() {
	l.mu.Lock()
	if len(l.buffer) == 0 {
		l.mu.Unlock()
		return
	}
	entries := l.buffer
	l.buffer = nil
	l.mu.Unlock()

	db := store.Get().DB()
	tx, err := db.Begin()
	if err != nil {
		return
	}
	stmt, err := tx.Prepare(
		`INSERT INTO audit_logs (id, time, username, role, action, resource, detail, ip, user_agent)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.Exec(e.ID, e.Time, e.Username, e.Role, e.Action, e.Resource, e.Detail, e.IP, e.UserAgent); err != nil {
			tx.Rollback()
			return
		}
	}
	tx.Commit()
}

func (l *Logger) Flush() {
	l.flush()
}

func (l *Logger) Query(username, action string, limit int) ([]model.AuditLog, error) {
	l.flush()

	db := store.Get().DB()
	var rows *sql.Rows
	var err error

	query := "SELECT id, time, username, role, action, resource, detail, ip, user_agent FROM audit_logs WHERE 1=1"
	var args []interface{}

	if username != "" {
		query += " AND username = ?"
		args = append(args, username)
	}
	if action != "" {
		query += " AND action LIKE ?"
		args = append(args, "%"+action+"%")
	}
	query += " ORDER BY time DESC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err = db.Query(query, args...)
	if err != nil {
		return []model.AuditLog{}, nil
	}
	defer rows.Close()

	var logs []model.AuditLog
	for rows.Next() {
		var entry model.AuditLog
		if err := rows.Scan(&entry.ID, &entry.Time, &entry.Username, &entry.Role, &entry.Action, &entry.Resource, &entry.Detail, &entry.IP, &entry.UserAgent); err != nil {
			continue
		}
		logs = append(logs, entry)
	}
	if logs == nil {
		logs = []model.AuditLog{}
	}
	return logs, nil
}
