package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"dbmanager/internal/db"
	"dbmanager/internal/model"
)

// auditOpt 记录优化操作的审计日志
func auditOpt(r *http.Request, action, resource, detail string) {
	auditRecord(r, action, resource, detail)
}

// getQuickConn 获取连接（复用 quick.go）
func getQuickConn(w http.ResponseWriter, r *http.Request) (model.DBConnection, db.DBDriver) {
	return getQuickConnection(w, r, quickParseID(r, "/api/quick/"))
}

// ==================== Redis 优化操作 ====================

// RedisListKeys 列出 key（使用 SCAN，带类型和 TTL）
func RedisListKeys(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	if conn.Type != model.DBTypeRedis {
		Error(w, 400, "仅支持 Redis")
		return
	}
	rd, ok := db.GetRedisDriver(conn)
	if !ok {
		Error(w, 500, "Redis 驱动未就绪")
		return
	}
	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		pattern = "*"
	}
	keys, err := rd.ScanKeys(pattern, 200)
	if err != nil {
		Error(w, 500, "获取 key 失败: "+err.Error())
		return
	}
	maxKeys := 500
	if len(keys) > maxKeys {
		keys = keys[:maxKeys]
	}
	details, err := rd.GetKeysDetail(keys)
	if err != nil {
		Error(w, 500, "获取 key 详情失败: "+err.Error())
		return
	}
	auditOpt(r, "redis_list_keys", "redis", fmt.Sprintf("pattern=%s, count=%d", pattern, len(details)))
	Success(w, map[string]interface{}{
		"keys":  details,
		"total": len(keys),
	})
}

// RedisGetKey 获取 key 详情（类型感知）
func RedisGetKey(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	if conn.Type != model.DBTypeRedis {
		Error(w, 400, "仅支持 Redis")
		return
	}
	rd, ok := db.GetRedisDriver(conn)
	if !ok {
		Error(w, 500, "Redis 驱动未就绪")
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		Error(w, 400, "key 不能为空")
		return
	}
	detail, err := rd.GetKeyDetail(key)
	if err != nil {
		Error(w, 500, "获取 key 失败: "+err.Error())
		return
	}
	Success(w, detail)
}

// RedisSetKey 设置 string 类型 key
func RedisSetKey(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	rd, ok := db.GetRedisDriver(conn)
	if !ok {
		Error(w, 500, "Redis 驱动未就绪")
		return
	}
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
		TTL   int    `json:"ttl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	if req.Key == "" {
		Error(w, 400, "key 不能为空")
		return
	}
	if err := rd.SetString(req.Key, req.Value, req.TTL); err != nil {
		Error(w, 500, "设置失败: "+err.Error())
		return
	}
	auditOpt(r, "redis_set", "redis", "key="+req.Key)
	Success(w, "设置成功")
}

// RedisSetHash 设置 Hash 字段
func RedisSetHash(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	rd, ok := db.GetRedisDriver(conn)
	if !ok {
		Error(w, 500, "Redis 驱动未就绪")
		return
	}
	var req struct {
		Key   string `json:"key"`
		Field string `json:"field"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	if req.Key == "" || req.Field == "" {
		Error(w, 400, "key 和 field 不能为空")
		return
	}
	if err := rd.HSet(req.Key, req.Field, req.Value); err != nil {
		Error(w, 500, "设置失败: "+err.Error())
		return
	}
	auditOpt(r, "redis_hset", "redis", "key="+req.Key+" field="+req.Field)
	Success(w, "设置成功")
}

// RedisPushList 列表 push
func RedisPushList(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	rd, ok := db.GetRedisDriver(conn)
	if !ok {
		Error(w, 500, "Redis 驱动未就绪")
		return
	}
	var req struct {
		Key    string `json:"key"`
		Value  string `json:"value"`
		Direction string `json:"direction"` // "left" or "right"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	var err error
	if req.Direction == "left" {
		err = rd.LPush(req.Key, req.Value)
	} else {
		err = rd.RPush(req.Key, req.Value)
	}
	if err != nil {
		Error(w, 500, "操作失败: "+err.Error())
		return
	}
	auditOpt(r, "redis_list_push", "redis", "key="+req.Key)
	Success(w, "操作成功")
}

// RedisAddSet 集合添加
func RedisAddSet(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	rd, ok := db.GetRedisDriver(conn)
	if !ok {
		Error(w, 500, "Redis 驱动未就绪")
		return
	}
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	if err := rd.SAdd(req.Key, req.Value); err != nil {
		Error(w, 500, "操作失败: "+err.Error())
		return
	}
	auditOpt(r, "redis_sadd", "redis", "key="+req.Key)
	Success(w, "操作成功")
}

// RedisAddZSet 有序集合添加
func RedisAddZSet(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	rd, ok := db.GetRedisDriver(conn)
	if !ok {
		Error(w, 500, "Redis 驱动未就绪")
		return
	}
	var req struct {
		Key    string  `json:"key"`
		Member string  `json:"member"`
		Score  float64 `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	if err := rd.ZAdd(req.Key, req.Member, req.Score); err != nil {
		Error(w, 500, "操作失败: "+err.Error())
		return
	}
	auditOpt(r, "redis_zadd", "redis", "key="+req.Key)
	Success(w, "操作成功")
}

// RedisSetTTL 设置过期时间
func RedisSetTTL(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	rd, ok := db.GetRedisDriver(conn)
	if !ok {
		Error(w, 500, "Redis 驱动未就绪")
		return
	}
	var req struct {
		Key string `json:"key"`
		TTL int    `json:"ttl"` // 秒，-1 表示永久
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	if err := rd.SetTTL(req.Key, req.TTL); err != nil {
		Error(w, 500, "操作失败: "+err.Error())
		return
	}
	auditOpt(r, "redis_ttl", "redis", fmt.Sprintf("key=%s ttl=%d", req.Key, req.TTL))
	Success(w, "操作成功")
}

// RedisDeleteKey 删除 key
func RedisDeleteKey(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	rd, ok := db.GetRedisDriver(conn)
	if !ok {
		Error(w, 500, "Redis 驱动未就绪")
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		Error(w, 400, "key 不能为空")
		return
	}
	if err := rd.DeleteKey(key); err != nil {
		Error(w, 500, "删除失败: "+err.Error())
		return
	}
	auditOpt(r, "redis_del", "redis", "key="+key)
	Success(w, "删除成功")
}

// RedisInfo 获取服务器信息
func RedisInfo(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	rd, ok := db.GetRedisDriver(conn)
	if !ok {
		Error(w, 500, "Redis 驱动未就绪")
		return
	}
	section := r.URL.Query().Get("section")
	info, err := rd.GetInfo(section)
	if err != nil {
		Error(w, 500, "获取信息失败: "+err.Error())
		return
	}
	Success(w, info)
}

// RedisDBSize 获取 key 数量
func RedisDBSize(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	rd, ok := db.GetRedisDriver(conn)
	if !ok {
		Error(w, 500, "Redis 驱动未就绪")
		return
	}
	size, err := rd.GetDBSize()
	if err != nil {
		Error(w, 500, "获取失败: "+err.Error())
		return
	}
	Success(w, map[string]interface{}{"dbsize": size})
}

// RedisFlushDB 清空当前 DB
func RedisFlushDB(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	rd, ok := db.GetRedisDriver(conn)
	if !ok {
		Error(w, 500, "Redis 驱动未就绪")
		return
	}
	if err := rd.FlushDB(); err != nil {
		Error(w, 500, "清空失败: "+err.Error())
		return
	}
	auditOpt(r, "redis_flushdb", "redis", "清空当前数据库")
	Success(w, "已清空当前数据库")
}

// ==================== MongoDB 优化操作 ====================

// MongoAggregate 聚合管道
func MongoAggregate(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	if conn.Type != model.DBTypeMongo {
		Error(w, 400, "仅支持 MongoDB")
		return
	}
	mg, ok := db.GetMongoDriver(conn)
	if !ok {
		Error(w, 500, "Mongo 驱动未就绪")
		return
	}
	var req struct {
		Collection string        `json:"collection"`
		Pipeline   []interface{} `json:"pipeline"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}
	if req.Collection == "" || len(req.Pipeline) == 0 {
		Error(w, 400, "集合名和管道不能为空")
		return
	}
	result, err := mg.Aggregate(req.Collection, req.Pipeline)
	if err != nil {
		Error(w, 500, "聚合失败: "+err.Error())
		return
	}
	auditOpt(r, "mongo_aggregate", "mongo", "collection="+req.Collection)
	Success(w, result)
}

// MongoCount 统计文档数量
func MongoCount(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	mg, ok := db.GetMongoDriver(conn)
	if !ok {
		Error(w, 500, "Mongo 驱动未就绪")
		return
	}
	collection := r.URL.Query().Get("collection")
	count, err := mg.Count(collection, nil)
	if err != nil {
		Error(w, 500, "统计失败: "+err.Error())
		return
	}
	Success(w, map[string]interface{}{"collection": collection, "count": count})
}

// MongoDistinct 获取去重值
func MongoDistinct(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	mg, ok := db.GetMongoDriver(conn)
	if !ok {
		Error(w, 500, "Mongo 驱动未就绪")
		return
	}
	collection := r.URL.Query().Get("collection")
	field := r.URL.Query().Get("field")
	if collection == "" || field == "" {
		Error(w, 400, "集合名和字段名不能为空")
		return
	}
	vals, err := mg.Distinct(collection, field, nil)
	if err != nil {
		Error(w, 500, "查询失败: "+err.Error())
		return
	}
	Success(w, map[string]interface{}{"field": field, "values": vals})
}

// MongoCollectionStats 集合统计
func MongoCollectionStats(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	mg, ok := db.GetMongoDriver(conn)
	if !ok {
		Error(w, 500, "Mongo 驱动未就绪")
		return
	}
	collection := r.URL.Query().Get("collection")
	if collection == "" {
		Error(w, 400, "集合名不能为空")
		return
	}
	stats, err := mg.CollectionStats(collection)
	if err != nil {
		Error(w, 500, "获取失败: "+err.Error())
		return
	}
	Success(w, stats)
}

// MongoDatabaseStats 数据库统计
func MongoDatabaseStats(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	mg, ok := db.GetMongoDriver(conn)
	if !ok {
		Error(w, 500, "Mongo 驱动未就绪")
		return
	}
	stats, err := mg.DatabaseStats()
	if err != nil {
		Error(w, 500, "获取失败: "+err.Error())
		return
	}
	Success(w, stats)
}

// MongoFindWithOptions 高级查询（排序、分页、投影）
func MongoFindWithOptions(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	mg, ok := db.GetMongoDriver(conn)
	if !ok {
		Error(w, 500, "Mongo 驱动未就绪")
		return
	}
	var req struct {
		Collection string                 `json:"collection"`
		Filter     map[string]interface{} `json:"filter"`
		Sort       map[string]int         `json:"sort"`
		Limit      int64                  `json:"limit"`
		Skip       int64                  `json:"skip"`
		Projection map[string]int         `json:"projection"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	if req.Collection == "" {
		Error(w, 400, "集合名不能为空")
		return
	}
	result, err := mg.FindWithOptions(req.Collection, req.Filter, req.Sort, req.Limit, req.Skip, req.Projection)
	if err != nil {
		Error(w, 500, "查询失败: "+err.Error())
		return
	}
	auditOpt(r, "mongo_find", "mongo", "collection="+req.Collection)
	Success(w, result)
}

// ==================== PostgreSQL 优化操作 ====================

// PGListSchemas 列出 schema
func PGListSchemas(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	if conn.Type != model.DBTypePostgres {
		Error(w, 400, "仅支持 PostgreSQL")
		return
	}
	result, err := driver.Query(`SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT IN ('pg_catalog','information_schema') ORDER BY schema_name`, "")
	if err != nil {
		Error(w, 500, "获取 schema 失败: "+err.Error())
		return
	}
	Success(w, result)
}

// PGCreateSchema 创建 schema
func PGCreateSchema(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	if req.Name == "" {
		Error(w, 400, "schema 名不能为空")
		return
	}
	sql := fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, strings.ReplaceAll(req.Name, `"`, `""`))
	if _, err := driver.Execute(sql, ""); err != nil {
		Error(w, 500, "创建失败: "+err.Error())
		return
	}
	auditOpt(r, "pg_create_schema", "postgres", "schema="+req.Name)
	Success(w, "创建成功")
}

// PGDropSchema 删除 schema
func PGDropSchema(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		Error(w, 400, "schema 名不能为空")
		return
	}
	sql := fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, strings.ReplaceAll(name, `"`, `""`))
	if _, err := driver.Execute(sql, ""); err != nil {
		Error(w, 500, "删除失败: "+err.Error())
		return
	}
	auditOpt(r, "pg_drop_schema", "postgres", "schema="+name)
	Success(w, "删除成功")
}

// PGExplain EXPLAIN ANALYZE 查询执行计划
func PGExplain(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	var req struct {
		SQL string `json:"sql"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	if req.SQL == "" {
		Error(w, 400, "SQL 不能为空")
		return
	}
	sql := "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) " + req.SQL
	result, err := driver.Query(sql, "")
	if err != nil {
		Error(w, 500, "执行失败: "+err.Error())
		return
	}
	auditOpt(r, "pg_explain", "postgres", "EXPLAIN ANALYZE")
	Success(w, result)
}

// PGVacuum VACUUM 表（回收空间、更新统计）
func PGVacuum(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	var req struct {
		Table  string `json:"table"`
		Full   bool   `json:"full"`
		Analyze bool  `json:"analyze"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	opts := ""
	if req.Full {
		opts += "FULL "
	}
	if req.Analyze {
		opts += "ANALYZE "
	}
	var sql string
	if req.Table != "" {
		sql = fmt.Sprintf("VACUUM %s\"%s\"", opts, strings.ReplaceAll(req.Table, `"`, `""`))
	} else {
		sql = "VACUUM " + opts
	}
	if _, err := driver.Execute(sql, ""); err != nil {
		Error(w, 500, "VACUUM 失败: "+err.Error())
		return
	}
	auditOpt(r, "pg_vacuum", "postgres", "table="+req.Table)
	Success(w, "VACUUM 完成")
}

// PGAnalyze ANALYZE 表
func PGAnalyze(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	var req struct {
		Table string `json:"table"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	var sql string
	if req.Table != "" {
		sql = fmt.Sprintf(`ANALYZE "%s"`, strings.ReplaceAll(req.Table, `"`, `""`))
	} else {
		sql = "ANALYZE"
	}
	if _, err := driver.Execute(sql, ""); err != nil {
		Error(w, 500, "ANALYZE 失败: "+err.Error())
		return
	}
	auditOpt(r, "pg_analyze", "postgres", "table="+req.Table)
	Success(w, "ANALYZE 完成")
}

// PGReindex REINDEX 表
func PGReindex(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	var req struct {
		Table string `json:"table"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	var sql string
	if req.Table != "" {
		sql = fmt.Sprintf(`REINDEX TABLE "%s"`, strings.ReplaceAll(req.Table, `"`, `""`))
	} else {
		sql = "REINDEX DATABASE CURRENT"
	}
	if _, err := driver.Execute(sql, ""); err != nil {
		Error(w, 500, "REINDEX 失败: "+err.Error())
		return
	}
	auditOpt(r, "pg_reindex", "postgres", "table="+req.Table)
	Success(w, "REINDEX 完成")
}

// PGTableSizes 表大小统计
func PGTableSizes(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	sql := `SELECT
		relname AS table_name,
		pg_size_pretty(pg_total_relation_size(relid)) AS total_size,
		pg_size_pretty(pg_relation_size(relid)) AS table_size,
		pg_size_pretty(pg_indexes_size(relid)) AS index_size,
		n_live_tup AS row_count
	FROM pg_catalog.pg_statio_user_tables
	ORDER BY pg_total_relation_size(relid) DESC
	LIMIT 50`
	result, err := driver.Query(sql, "")
	if err != nil {
		Error(w, 500, "获取失败: "+err.Error())
		return
	}
	Success(w, result)
}

// PGSequences 列出序列
func PGSequences(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	sql := `SELECT sequence_schema, sequence_name, data_type, start_value, increment
		FROM information_schema.sequences ORDER BY sequence_schema, sequence_name`
	result, err := driver.Query(sql, "")
	if err != nil {
		Error(w, 500, "获取序列失败: "+err.Error())
		return
	}
	Success(w, result)
}

// ==================== SQLite 优化操作 ====================

// SQLiteGetPragmas 获取关键 PRAGMA 值
func SQLiteGetPragmas(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	if conn.Type != model.DBTypeSQLite {
		Error(w, 400, "仅支持 SQLite")
		return
	}
	pragmas := []string{"journal_mode", "synchronous", "foreign_keys", "cache_size", "page_size", "auto_vacuum", "busy_timeout", "wal_autocheckpoint"}
	result := make(map[string]interface{})
	for _, p := range pragmas {
		res, err := driver.Query(fmt.Sprintf("PRAGMA %s", p), "")
		if err != nil {
			continue
		}
		if res.Rows != nil && len(res.Rows) > 0 {
			if m, ok := res.Rows[0].(map[string]interface{}); ok {
				for _, v := range m {
					result[p] = v
					break
				}
			}
		}
	}
	Success(w, result)
}

// SQLiteSetPragma 设置 PRAGMA
func SQLiteSetPragma(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	var req struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	// 允许设置的 PRAGMA 白名单
	allowed := map[string]bool{
		"journal_mode": true, "synchronous": true, "foreign_keys": true,
		"cache_size": true, "page_size": true, "auto_vacuum": true,
		"busy_timeout": true, "wal_autocheckpoint": true,
	}
	if !allowed[req.Name] {
		Error(w, 400, "不允许设置该 PRAGMA")
		return
	}
	sql := fmt.Sprintf("PRAGMA %s = %s", req.Name, req.Value)
	if _, err := driver.Execute(sql, ""); err != nil {
		Error(w, 500, "设置失败: "+err.Error())
		return
	}
	auditOpt(r, "sqlite_pragma", "sqlite", fmt.Sprintf("%s=%s", req.Name, req.Value))
	Success(w, "设置成功")
}

// SQLiteVacuum VACUUM 回收空间
func SQLiteVacuum(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	if _, err := driver.Execute("VACUUM", ""); err != nil {
		Error(w, 500, "VACUUM 失败: "+err.Error())
		return
	}
	auditOpt(r, "sqlite_vacuum", "sqlite", "VACUUM")
	Success(w, "VACUUM 完成")
}

// SQLiteReindex REINDEX 重建索引
func SQLiteReindex(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	table := r.URL.Query().Get("table")
	var sql string
	if table != "" {
		sql = fmt.Sprintf("REINDEX \"%s\"", strings.ReplaceAll(table, `"`, `""`))
	} else {
		sql = "REINDEX"
	}
	if _, err := driver.Execute(sql, ""); err != nil {
		Error(w, 500, "REINDEX 失败: "+err.Error())
		return
	}
	auditOpt(r, "sqlite_reindex", "sqlite", "table="+table)
	Success(w, "REINDEX 完成")
}

// SQLiteExplain EXPLAIN QUERY PLAN
func SQLiteExplain(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	var req struct {
		SQL string `json:"sql"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误")
		return
	}
	sql := "EXPLAIN QUERY PLAN " + req.SQL
	result, err := driver.Query(sql, "")
	if err != nil {
		Error(w, 500, "执行失败: "+err.Error())
		return
	}
	auditOpt(r, "sqlite_explain", "sqlite", "EXPLAIN QUERY PLAN")
	Success(w, result)
}

// SQLiteVersion 获取 SQLite 版本
func SQLiteVersion(w http.ResponseWriter, r *http.Request) {
	conn, driver := getQuickConn(w, r)
	if conn.ID == "" {
		return
	}
	result, err := driver.Query("SELECT sqlite_version() AS version", "")
	if err != nil {
		Error(w, 500, "获取失败: "+err.Error())
		return
	}
	Success(w, result)
}

