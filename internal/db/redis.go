package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dbmanager/internal/model"

	"github.com/redis/go-redis/v9"
)

// RedisDriver Redis驱动
type RedisDriver struct {
	client *redis.Client
}

func (d *RedisDriver) Connect(conn model.DBConnection) error {
	addr := fmt.Sprintf("%s:%d", conn.Host, conn.Port)
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: conn.Password,
		DB:       conn.DBIndex,
		PoolSize: 10,
	})

	d.client = client
	return nil
}

func (d *RedisDriver) Close() error {
	if d.client != nil {
		return d.client.Close()
	}
	return nil
}

func (d *RedisDriver) Ping() (time.Duration, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := d.client.Ping(ctx).Err()
	return time.Since(start), err
}

func (d *RedisDriver) Query(sql string, collection string) (*model.QueryResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// sql字段作为Redis命令处理
	// 支持的命令: GET, KEYS, HGETALL, LRANGE, SMEMBERS
	cmd := sql
	if cmd == "" {
		// 默认查询所有key
		keys, err := d.client.Keys(ctx, "*").Result()
		if err != nil {
			return nil, err
		}
		var result []interface{}
		for _, key := range keys {
			keyType, _ := d.client.Type(ctx, key).Result()
			result = append(result, map[string]interface{}{
				"key":  key,
				"type": keyType,
			})
		}
		return &model.QueryResult{
			Success: true,
			Columns: []string{"key", "type"},
			Rows:    result,
			Count:   int64(len(result)),
		}, nil
	}

	// 简单解析命令，按空格分割
	parts := strings.Fields(cmd)
	// 转换为interface{}切片
	args := make([]interface{}, len(parts))
	for i, p := range parts {
		args[i] = p
	}
	result, err := d.client.Do(ctx, args...).Result()
	if err != nil {
		return nil, err
	}

	return &model.QueryResult{
		Success: true,
		Columns: []string{"result"},
		Rows:    []interface{}{map[string]interface{}{"result": fmt.Sprintf("%v", result)}},
		Count:   1,
	}, nil
}

func (d *RedisDriver) Execute(sql string, collection string) (*model.QueryResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// sql格式: SET key value
	var key, value string
	// 简单解析 SET key value
	cmd := sql
	if len(cmd) > 4 && cmd[:3] == "SET" {
		// 简化解析
		fmt.Sscanf(cmd[4:], "%s %s", &key, &value)
	}
	if key == "" {
		return nil, fmt.Errorf("Redis写操作格式: SET key value")
	}

	err := d.client.Set(ctx, key, value, 0).Err()
	if err != nil {
		return nil, err
	}

	return &model.QueryResult{
		Success: true,
		Message: "执行成功",
		Count:   1,
	}, nil
}

func (d *RedisDriver) ListTables() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	keys, err := d.client.Keys(ctx, "*").Result()
	if err != nil {
		return nil, err
	}
	return keys, nil
}
