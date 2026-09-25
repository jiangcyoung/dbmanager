package db

import (
	"sync"
	"time"

	"dbmanager/internal/model"
)

const (
	maxPoolSize     = 50
	idleTimeout     = 30 * time.Minute
	cleanupInterval = 5 * time.Minute
)

type DBDriver interface {
	Connect(conn model.DBConnection) error
	Close() error
	Ping() (time.Duration, error)
	Query(sql string, collection string) (*model.QueryResult, error)
	Execute(sql string, collection string) (*model.QueryResult, error)
	ListTables() ([]string, error)
}

type PooledConnection struct {
	Conn      DBDriver
	Config    model.DBConnection
	LastUsed  time.Time
	CreatedAt time.Time
}

type Manager struct {
	pool map[string]*PooledConnection
	mu   sync.RWMutex
	done chan struct{}
}

var (
	manager *Manager
	once    sync.Once
)

func GetManager() *Manager {
	once.Do(func() {
		manager = &Manager{
			pool: make(map[string]*PooledConnection),
			done: make(chan struct{}),
		}
		go manager.cleanupLoop()
	})
	return manager
}

func (m *Manager) GetConnection(conn model.DBConnection) (DBDriver, error) {
	m.mu.RLock()
	if pc, ok := m.pool[conn.ID]; ok {
		if _, err := pc.Conn.Ping(); err == nil {
			pc.LastUsed = time.Now()
			m.mu.RUnlock()
			return pc.Conn, nil
		}
		pc.Conn.Close()
		delete(m.pool, conn.ID)
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if pc, ok := m.pool[conn.ID]; ok {
		if _, err := pc.Conn.Ping(); err == nil {
			pc.LastUsed = time.Now()
			return pc.Conn, nil
		}
		pc.Conn.Close()
		delete(m.pool, conn.ID)
	}

	if len(m.pool) >= maxPoolSize {
		var oldestID string
		var oldestTime time.Time
		for id, pc := range m.pool {
			if oldestID == "" || pc.LastUsed.Before(oldestTime) {
				oldestID = id
				oldestTime = pc.LastUsed
			}
		}
		if oldestID != "" {
			m.pool[oldestID].Conn.Close()
			delete(m.pool, oldestID)
		}
	}

	driver, err := m.createDriver(conn)
	if err != nil {
		return nil, err
	}
	if err := driver.Connect(conn); err != nil {
		return nil, err
	}

	now := time.Now()
	m.pool[conn.ID] = &PooledConnection{
		Conn:      driver,
		Config:    conn,
		LastUsed:  now,
		CreatedAt: now,
	}
	return driver, nil
}

func (m *Manager) TestConnection(conn model.DBConnection) *model.TestResult {
	start := time.Now()
	driver, err := m.createDriver(conn)
	if err != nil {
		return &model.TestResult{
			Success: false,
			Message: "创建驱动失败: " + err.Error(),
			Latency: time.Since(start).Milliseconds(),
		}
	}
	defer driver.Close()

	if err := driver.Connect(conn); err != nil {
		return &model.TestResult{
			Success: false,
			Message: "连接失败: " + err.Error(),
			Latency: time.Since(start).Milliseconds(),
		}
	}

	latency, err := driver.Ping()
	if err != nil {
		return &model.TestResult{
			Success: false,
			Message: "Ping失败: " + err.Error(),
			Latency: time.Since(start).Milliseconds(),
		}
	}

	return &model.TestResult{
		Success: true,
		Message: "连接成功",
		Latency: latency.Milliseconds(),
	}
}

func (m *Manager) CloseConnection(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pc, ok := m.pool[id]; ok {
		pc.Conn.Close()
		delete(m.pool, id)
	}
}

// IsConnected 检查连接是否在线（存在于连接池且可 Ping）
func (m *Manager) IsConnected(id string) bool {
	m.mu.RLock()
	pc, ok := m.pool[id]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	_, err := pc.Conn.Ping()
	return err == nil
}

// Connect 显式建立连接（放入连接池）
func (m *Manager) Connect(conn model.DBConnection) error {
	_, err := m.GetConnection(conn)
	return err
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, pc := range m.pool {
		pc.Conn.Close()
		delete(m.pool, id)
	}
	select {
	case <-m.done:
	default:
		close(m.done)
	}
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.cleanupIdle()
		case <-m.done:
			return
		}
	}
}

func (m *Manager) cleanupIdle() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, pc := range m.pool {
		if now.Sub(pc.LastUsed) > idleTimeout {
			pc.Conn.Close()
			delete(m.pool, id)
		}
	}
	for len(m.pool) > maxPoolSize {
		var oldestID string
		var oldestTime time.Time
		for id, pc := range m.pool {
			if oldestID == "" || pc.LastUsed.Before(oldestTime) {
				oldestID = id
				oldestTime = pc.LastUsed
			}
		}
		if oldestID == "" {
			break
		}
		m.pool[oldestID].Conn.Close()
		delete(m.pool, oldestID)
	}
}

func (m *Manager) createDriver(conn model.DBConnection) (DBDriver, error) {
	switch conn.Type {
	case model.DBTypeMySQL:
		return &MySQLDriver{}, nil
	case model.DBTypePostgres:
		return &PostgresDriver{}, nil
	case model.DBTypeSQLite:
		return &SQLiteDriver{}, nil
	case model.DBTypeMongo:
		return &MongoDriver{}, nil
	case model.DBTypeRedis:
		return &RedisDriver{}, nil
	default:
		return nil, &UnsupportedDBTypeError{Type: string(conn.Type)}
	}
}

type UnsupportedDBTypeError struct {
	Type string
}

func (e *UnsupportedDBTypeError) Error() string {
	return "不支持的数据库类型: " + e.Type
}
