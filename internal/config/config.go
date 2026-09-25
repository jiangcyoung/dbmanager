package config

import (
	"encoding/json"
	"os"
	"sync"

	"dbmanager/internal/model"
)

// AdminConfig 管理员配置
type AdminConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Config 应用配置
type Config struct {
	Port                 string               `json:"port"`
	DataDir              string               `json:"data_dir"`
	ConnectionsFile      string               `json:"connections_file"`
	UsersFile            string               `json:"users_file"`
	AuditLogFile         string               `json:"audit_log_file"`
	SessionTimeoutMin    int                  `json:"session_timeout_minutes"`
	Admin                AdminConfig          `json:"admin"`
	Connections          []model.DBConnection `json:"-"`
}

var (
	cfg  *Config
	once sync.Once
	mu   sync.RWMutex
)

// GetConfig 获取配置单例
func GetConfig() *Config {
	once.Do(func() {
		cfg = &Config{
			Port:              "8080",
			DataDir:           "data",
			ConnectionsFile:   "data/connections.json",
			UsersFile:         "data/users.json",
			AuditLogFile:      "data/audit.log",
			SessionTimeoutMin: 120,
			Admin: AdminConfig{
				Username: "admin",
				Password: "admin123",
			},
		}
		cfg.loadSystem()
		cfg.loadConnections()
	})
	return cfg
}

// loadSystem 从 config.json 加载系统配置
func (c *Config) loadSystem() {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, c)
	// 确保默认值
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.DataDir == "" {
		c.DataDir = "data"
	}
	if c.ConnectionsFile == "" {
		c.ConnectionsFile = c.DataDir + "/connections.json"
	}
	if c.UsersFile == "" {
		c.UsersFile = c.DataDir + "/users.json"
	}
	if c.AuditLogFile == "" {
		c.AuditLogFile = c.DataDir + "/audit.log"
	}
	if c.SessionTimeoutMin <= 0 {
		c.SessionTimeoutMin = 120
	}
	if c.Admin.Username == "" {
		c.Admin.Username = "admin"
	}
	if c.Admin.Password == "" {
		c.Admin.Password = "admin123"
	}
	_ = os.MkdirAll(c.DataDir, 0755)
}

// loadConnections 从文件加载连接
func (c *Config) loadConnections() {
	data, err := os.ReadFile(c.ConnectionsFile)
	if err != nil {
		return
	}
	var conns []model.DBConnection
	if err := json.Unmarshal(data, &conns); err == nil {
		c.Connections = conns
	}
}

// SaveConnections 保存连接到文件
func (c *Config) SaveConnections() error {
	mu.Lock()
	defer mu.Unlock()

	_ = os.MkdirAll(c.DataDir, 0755)
	data, err := json.MarshalIndent(c.Connections, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.ConnectionsFile, data, 0644)
}

// GetConnections 获取所有连接
func (c *Config) GetConnections() []model.DBConnection {
	mu.RLock()
	defer mu.RUnlock()
	result := make([]model.DBConnection, len(c.Connections))
	for i, conn := range c.Connections {
		result[i] = conn
		result[i].Password = ""
	}
	return result
}

// GetConnectionByID 根据ID获取连接
func (c *Config) GetConnectionByID(id string) *model.DBConnection {
	mu.RLock()
	defer mu.RUnlock()
	for _, conn := range c.Connections {
		if conn.ID == id {
			result := conn
			return &result
		}
	}
	return nil
}

// AddConnection 添加连接
func (c *Config) AddConnection(conn model.DBConnection) error {
	mu.Lock()
	c.Connections = append(c.Connections, conn)
	mu.Unlock()
	return c.SaveConnections()
}

// UpdateConnection 更新连接
func (c *Config) UpdateConnection(id string, conn model.DBConnection) error {
	mu.Lock()
	for i, item := range c.Connections {
		if item.ID == id {
			conn.ID = id
			c.Connections[i] = conn
			break
		}
	}
	mu.Unlock()
	return c.SaveConnections()
}

// DeleteConnection 删除连接
func (c *Config) DeleteConnection(id string) error {
	mu.Lock()
	for i, conn := range c.Connections {
		if conn.ID == id {
			c.Connections = append(c.Connections[:i], c.Connections[i+1:]...)
			break
		}
	}
	mu.Unlock()
	return c.SaveConnections()
}
