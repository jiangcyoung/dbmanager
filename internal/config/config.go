package config

import (
	"encoding/json"
	"os"
	"sync"

	"dbmanager/internal/model"
)

// Config 应用配置
type Config struct {
	Port        string               `json:"port"`
	DataFile    string               `json:"data_file"`
	Connections []model.DBConnection `json:"connections"`
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
			Port:     "8080",
			DataFile: "data/connections.json",
		}
		cfg.load()
	})
	return cfg
}

// load 从文件加载配置
func (c *Config) load() {
	data, err := os.ReadFile(c.DataFile)
	if err != nil {
		// 文件不存在，使用空配置
		return
	}
	var conns []model.DBConnection
	if err := json.Unmarshal(data, &conns); err == nil {
		c.Connections = conns
	}
}

// Save 保存配置到文件
func (c *Config) Save() error {
	mu.Lock()
	defer mu.Unlock()

	dir := "data"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}
	data, err := json.MarshalIndent(c.Connections, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.DataFile, data, 0644)
}

// GetConnections 获取所有连接
func (c *Config) GetConnections() []model.DBConnection {
	mu.RLock()
	defer mu.RUnlock()
	// 返回副本，不暴露密码
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
	return c.Save()
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
	return c.Save()
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
	return c.Save()
}
