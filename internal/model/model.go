package model

import "time"

// Role 用户角色
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// UserStatus 用户状态
type UserStatus string

const (
	StatusPending  UserStatus = "pending"
	StatusActive   UserStatus = "active"
	StatusDisabled UserStatus = "disabled"
)

// User 系统用户
type User struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"password_hash"`
	Role         Role       `json:"role"`
	Status       UserStatus `json:"status"`
	Description  string     `json:"description"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

// PublicUser 对外暴露的用户信息（不含密码）
type PublicUser struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Role        Role       `json:"role"`
	Status      UserStatus `json:"status"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// ToPublic 转换为公开信息
func (u *User) ToPublic() PublicUser {
	return PublicUser{
		ID:          u.ID,
		Username:    u.Username,
		Role:        u.Role,
		Status:      u.Status,
		Description: u.Description,
		CreatedAt:   u.CreatedAt,
		LastLoginAt: u.LastLoginAt,
	}
}

// AuditLog 审计日志条目
type AuditLog struct {
	ID        string    `json:"id"`
	Time      time.Time `json:"time"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Detail    string    `json:"detail"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
}

// OnlineUser 在线用户
type OnlineUser struct {
	Username   string    `json:"username"`
	Role       string    `json:"role"`
	LoginAt    time.Time `json:"login_at"`
	LastActive time.Time `json:"last_active"`
	IP         string    `json:"ip"`
}

// DBType 数据库类型
type DBType string

const (
	DBTypeMySQL    DBType = "mysql"
	DBTypePostgres DBType = "postgres"
	DBTypeSQLite   DBType = "sqlite"
	DBTypeMongo    DBType = "mongodb"
	DBTypeRedis    DBType = "redis"
)

// DBConnection 数据库连接配置
type DBConnection struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     DBType `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Database string `json:"database"`
	// SQLite专用
	FilePath string `json:"file_path,omitempty"`
	// Redis专用
	DBIndex int `json:"db_index,omitempty"`
	// 额外参数
	Params map[string]string `json:"params,omitempty"`
	// 创建者（用户ID），普通用户仅可见自己创建的连接
	CreatedBy string `json:"created_by,omitempty"`
}

// TestResult 连接测试结果
type TestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency int64  `json:"latency_ms"`
}

// QueryRequest 查询请求
type QueryRequest struct {
	ConnectionID string `json:"connection_id"`
	SQL          string `json:"sql"`
	// Mongo专用
	Collection string `json:"collection,omitempty"`
}

// QueryResult 查询结果
type QueryResult struct {
	Success bool          `json:"success"`
	Message string        `json:"message"`
	Columns []string      `json:"columns,omitempty"`
	Rows    []interface{} `json:"rows,omitempty"`
	Count   int64         `json:"count"`
}

// APIResponse 统一API响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// QuickColumn 快捷建表列定义
type QuickColumn struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Nullable      bool   `json:"nullable"`
	PrimaryKey    bool   `json:"primary_key"`
	AutoIncrement bool   `json:"auto_increment"`
	DefaultValue  string `json:"default_value"`
}

// CreateTableRequest 快捷建表请求
type CreateTableRequest struct {
	Table   string        `json:"table"`
	Columns []QuickColumn `json:"columns"`
}

// IndexRequest 索引操作请求
type IndexRequest struct {
	Table     string   `json:"table"`
	IndexName string   `json:"index_name"`
	Columns   []string `json:"columns"`
	Unique    bool     `json:"unique"`
}

// RowRequest 行操作请求
type RowRequest struct {
	Table string                 `json:"table"`
	Data  map[string]interface{} `json:"data"`
	Where map[string]interface{} `json:"where"`
}
