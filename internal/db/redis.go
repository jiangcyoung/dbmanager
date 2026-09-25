package db

import (
	"context"
	"fmt"
	"strconv"
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
			ttl, _ := d.client.TTL(ctx, key).Result()
			ttlStr := "-1"
			if ttl > 0 {
				ttlStr = fmt.Sprintf("%ds", int(ttl.Seconds()))
			} else if ttl == -1 {
				ttlStr = "永久"
			}
			result = append(result, map[string]interface{}{
				"key":  key,
				"type": keyType,
				"ttl":  ttlStr,
			})
		}
		return &model.QueryResult{
			Success: true,
			Columns: []string{"key", "type", "ttl"},
			Rows:    result,
			Count:   int64(len(result)),
		}, nil
	}

	// 简单解析命令
	parts := strings.Fields(cmd)
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

	var key, value string
	cmd := sql
	if len(cmd) > 4 && cmd[:3] == "SET" {
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

// ========== Redis 类型感知优化操作 ==========

// ScanKeys 使用 SCAN 迭代 key（替代 KEYS，生产环境安全）
func (d *RedisDriver) ScanKeys(pattern string, count int64) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if count <= 0 {
		count = 100
	}
	var keys []string
	var cursor uint64
	for {
		ks, next, err := d.client.Scan(ctx, cursor, pattern, count).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, ks...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return keys, nil
}

// GetKeyDetail 获取 key 的类型化详情
func (d *RedisDriver) GetKeyDetail(key string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	keyType, err := d.client.Type(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	ttl, _ := d.client.TTL(ctx, key).Result()
	detail := map[string]interface{}{
		"key":  key,
		"type": keyType,
		"ttl":  int64(ttl.Seconds()),
	}
	switch keyType {
	case "string":
		val, err := d.client.Get(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		detail["value"] = val
	case "hash":
		vals, err := d.client.HGetAll(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		detail["value"] = vals
	case "list":
		vals, err := d.client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			return nil, err
		}
		detail["value"] = vals
	case "set":
		vals, err := d.client.SMembers(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		detail["value"] = vals
	case "zset":
		vals, err := d.client.ZRangeWithScores(ctx, key, 0, -1).Result()
		if err != nil {
			return nil, err
		}
		list := make([]map[string]interface{}, 0, len(vals))
		for _, z := range vals {
			list = append(list, map[string]interface{}{"member": z.Member, "score": z.Score})
		}
		detail["value"] = list
	}
	return detail, nil
}

// SetString 设置字符串值（可带 TTL 秒）
func (d *RedisDriver) SetString(key, value string, ttl int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exp := time.Duration(ttl) * time.Second
	return d.client.Set(ctx, key, value, exp).Err()
}

// HSet 设置 Hash 字段
func (d *RedisDriver) HSet(key, field, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.client.HSet(ctx, key, field, value).Err()
}

// LPush / RPush 列表操作
func (d *RedisDriver) LPush(key, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.client.LPush(ctx, key, value).Err()
}
func (d *RedisDriver) RPush(key, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.client.RPush(ctx, key, value).Err()
}

// SAdd 集合添加
func (d *RedisDriver) SAdd(key, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.client.SAdd(ctx, key, value).Err()
}

// ZAdd 有序集合添加
func (d *RedisDriver) ZAdd(key, member string, score float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.client.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

// SetTTL 设置过期时间（秒），-1 表示永久
func (d *RedisDriver) SetTTL(key string, ttl int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if ttl < 0 {
		return d.client.Persist(ctx, key).Err()
	}
	return d.client.Expire(ctx, key, time.Duration(ttl)*time.Second).Err()
}

// DeleteKey 删除 key
func (d *RedisDriver) DeleteKey(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.client.Del(ctx, key).Err()
}

// GetInfo 获取服务器信息（可指定 section: server, memory, clients, stats 等）
func (d *RedisDriver) GetInfo(section string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	info, err := d.client.Info(ctx, section).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, line := range strings.Split(info, "\r\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			result[line[:idx]] = line[idx+1:]
		}
	}
	return result, nil
}

// GetDBSize 获取当前 DB key 数量
func (d *RedisDriver) GetDBSize() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.client.DBSize(ctx).Result()
}

// FlushDB 清空当前数据库
func (d *RedisDriver) FlushDB() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.client.FlushDB(ctx).Err()
}

// GetRedisDriver 获取 Redis 驱动
func GetRedisDriver(conn model.DBConnection) (*RedisDriver, bool) {
	manager := GetManager()
	manager.mu.RLock()
	pc, ok := manager.pool[conn.ID]
	manager.mu.RUnlock()
	if !ok {
		return nil, false
	}
	d, ok := pc.Conn.(*RedisDriver)
	return d, ok
}

// 辅助：将 interface{} 转为 float64
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}
