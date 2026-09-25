package db

import (
	"sync"
	"time"

	"dbmanager/internal/model"
)

// DBDriver 数据库驱动接口
type DBDriver interface {
	// Connect 建立连接
	Connect(conn model.DBConnection) error
	// Close 关闭连接
	Close() error
	// Ping 测试连接
	Ping() (time.Duration, error)
	// Query 执行查询
	Query(sql string, collection string) (*model.QueryResult, error)
	// Execute 执行写操作
	Execute(sql string, collection string) (*model.QueryResult, error)
	// ListTables 列出所有表/集合
	ListTables() ([]string, error)
}

// PooledConnection 连接池中的连接
type PooledConnection struct {
	Conn      DBDriver
	Config    model.DBConnection
	LastUsed  time.Time
	CreatedAt time.Time
}

// Manager 数据库连接管理器
type Manager struct {
	pool map[string]*PooledConnection
	mu   sync.RWMutex
}

var (
	manager *Manager
	once    sync.Once
)

// GetManager 获取连接管理器单例
func GetManager() *Manager {
	once.Do(func() {
		manager = &Manager{
			pool: make(map[string]*PooledConnection),
		}
	})
	return manager
}

// GetConnection 获取或创建数据库连接
func (m *Manager) GetConnection(conn model.DBConnection) (DBDriver, error) {
	m.mu.RLock()
	if pc, ok := m.pool[conn.ID]; ok {
		// 检查连接是否还活着
		if _, err := pc.Conn.Ping(); err == nil {
			pc.LastUsed = time.Now()
			m.mu.RUnlock()
			return pc.Conn, nil
		}
		// 连接失效，关闭后重新创建
		pc.Conn.Close()
		delete(m.pool, conn.ID)
	}
	m.mu.RUnlock()

	// 创建新连接
	driver, err := m.createDriver(conn)
	if err != nil {
		return nil, err
	}
	if err := driver.Connect(conn); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.pool[conn.ID] = &PooledConnection{
		Conn:      driver,
		Config:    conn,
		LastUsed:  time.Now(),
		CreatedAt: time.Now(),
	}
	m.mu.Unlock()

	return driver, nil
}

// TestConnection 测试连接（不放入连接池）
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

// CloseConnection 关闭指定连接
func (m *Manager) CloseConnection(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pc, ok := m.pool[id]; ok {
		pc.Conn.Close()
		delete(m.pool, id)
	}
}

// CloseAll 关闭所有连接
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, pc := range m.pool {
		pc.Conn.Close()
		delete(m.pool, id)
	}
}

// createDriver 根据数据库类型创建驱动
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

// UnsupportedDBTypeError 不支持的数据库类型错误
type UnsupportedDBTypeError struct {
	Type string
}

func (e *UnsupportedDBTypeError) Error() string {
	return "不支持的数据库类型: " + e.Type
}
