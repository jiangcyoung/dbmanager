package config

import (
	"database/sql"
	"encoding/json"
	"os"
	"sync"
	"time"

	"dbmanager/internal/model"
	"dbmanager/internal/store"
)

type AdminConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Config struct {
	Port              string      `json:"port"`
	DataDir           string      `json:"data_dir"`
	SessionTimeoutMin int         `json:"session_timeout_minutes"`
	Admin             AdminConfig `json:"admin"`
}

var (
	cfg  *Config
	once sync.Once
)

func GetConfig() *Config {
	once.Do(func() {
		cfg = &Config{
			Port:              "8080",
			DataDir:           "data",
			SessionTimeoutMin: 120,
			Admin: AdminConfig{
				Username: "admin",
				Password: "admin123",
			},
		}
		cfg.loadSystem()
	})
	return cfg
}

func (c *Config) loadSystem() {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, c)
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.DataDir == "" {
		c.DataDir = "data"
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
}

func (c *Config) db() *sql.DB {
	return store.Get().DB()
}

func scanConnection(rows *sql.Rows) (model.DBConnection, error) {
	var conn model.DBConnection
	var paramsJSON sql.NullString
	err := rows.Scan(&conn.ID, &conn.Name, &conn.Type, &conn.Host, &conn.Port,
		&conn.Username, &conn.Password, &conn.Database, &conn.FilePath,
		&conn.DBIndex, &paramsJSON, &conn.CreatedBy)
	if err != nil {
		return conn, err
	}
	if paramsJSON.Valid && paramsJSON.String != "" {
		_ = json.Unmarshal([]byte(paramsJSON.String), &conn.Params)
	}
	return conn, nil
}

const connCols = `id, name, type, host, port, username, password, database_name, file_path, db_index, params_json, created_by`

func (c *Config) GetConnections() []model.DBConnection {
	rows, err := c.db().Query("SELECT " + connCols + " FROM connections ORDER BY name")
	if err != nil {
		return []model.DBConnection{}
	}
	defer rows.Close()
	var conns []model.DBConnection
	for rows.Next() {
		conn, err := scanConnection(rows)
		if err != nil {
			continue
		}
		conn.Password = ""
		conns = append(conns, conn)
	}
	if conns == nil {
		conns = []model.DBConnection{}
	}
	return conns
}

func (c *Config) GetConnectionByID(id string) *model.DBConnection {
	rows, err := c.db().Query("SELECT "+connCols+" FROM connections WHERE id = ?", id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	if !rows.Next() {
		return nil
	}
	conn, err := scanConnection(rows)
	if err != nil {
		return nil
	}
	return &conn
}

func (c *Config) AddConnection(conn model.DBConnection) error {
	paramsJSON, _ := json.Marshal(conn.Params)
	_, err := c.db().Exec(
		`INSERT INTO connections (id, name, type, host, port, username, password, database_name, file_path, db_index, params_json, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		conn.ID, conn.Name, conn.Type, conn.Host, conn.Port,
		conn.Username, conn.Password, conn.Database, conn.FilePath,
		conn.DBIndex, string(paramsJSON), conn.CreatedBy, time.Now(),
	)
	return err
}

func (c *Config) UpdateConnection(id string, conn model.DBConnection) error {
	paramsJSON, _ := json.Marshal(conn.Params)
	_, err := c.db().Exec(
		`UPDATE connections SET name=?, type=?, host=?, port=?, username=?, password=?,
		 database_name=?, file_path=?, db_index=?, params_json=?, created_by=? WHERE id=?`,
		conn.Name, conn.Type, conn.Host, conn.Port, conn.Username, conn.Password,
		conn.Database, conn.FilePath, conn.DBIndex, string(paramsJSON), conn.CreatedBy, id,
	)
	return err
}

func (c *Config) DeleteConnection(id string) error {
	_, err := c.db().Exec("DELETE FROM connections WHERE id = ?", id)
	return err
}
