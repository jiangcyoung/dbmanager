package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"dbmanager/internal/config"
	"dbmanager/internal/db"
	"dbmanager/internal/model"
)

// getQuickConnection 获取连接并返回驱动
func getQuickConnection(w http.ResponseWriter, id string) (model.DBConnection, db.DBDriver) {
	cfg := config.GetConfig()
	conn := cfg.GetConnectionByID(id)
	if conn == nil {
		Error(w, 404, "连接不存在")
		return model.DBConnection{}, nil
	}
	driver, err := db.GetManager().GetConnection(*conn)
	if err != nil {
		Error(w, 500, "连接失败: "+err.Error())
		return model.DBConnection{}, nil
	}
	return *conn, driver
}

// quickParseID 从路径解析连接ID
func quickParseID(r *http.Request, prefix string) string {
	p := r.URL.Path
	if !strings.HasPrefix(p, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(p, prefix)
	idx := strings.Index(rest, "/")
	if idx < 0 {
		return rest
	}
	return rest[:idx]
}

// quoteIdent 标识符引用
func quoteIdent(connType model.DBType, s string) string {
	if s == "" {
		return ""
	}
	if connType == model.DBTypeMySQL {
		return "`" + strings.ReplaceAll(s, "`", "``") + "`"
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// quoteValue 值转义
func quoteValue(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "NULL"
	case bool:
		if val {
			return "TRUE"
		}
		return "FALSE"
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case string:
		return "'" + strings.ReplaceAll(val, "'", "''") + "'"
	default:
		return "'" + strings.ReplaceAll(fmt.Sprintf("%v", val), "'", "''") + "'"
	}
}

// buildWhere SQL WHERE 子句
func buildWhere(connType model.DBType, where map[string]interface{}) (string, error) {
	if len(where) == 0 {
		return "", fmt.Errorf("需要指定条件")
	}
	var conds []string
	for k, v := range where {
		conds = append(conds, fmt.Sprintf("%s = %s", quoteIdent(connType, k), quoteValue(v)))
	}
	return strings.Join(conds, " AND "), nil
}

// ========== 库操作 ==========

// QuickListDatabases 列出数据库
func QuickListDatabases(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	if conn.Type != model.DBTypeMySQL && conn.Type != model.DBTypePostgres {
		Error(w, 400, "当前数据库类型不支持库操作")
		return
	}
	sql := "SHOW DATABASES"
	if conn.Type == model.DBTypePostgres {
		sql = "SELECT datname AS database FROM pg_database WHERE datistemplate = false AND datallowconn = true"
	}
	driver, _ := db.GetManager().GetConnection(conn)
	result, err := driver.Query(sql, "")
	if err != nil {
		Error(w, 500, "获取库列表失败: "+err.Error())
		return
	}
	var names []string
	for _, row := range result.Rows {
		if m, ok := row.(map[string]interface{}); ok {
			for _, v := range m {
				names = append(names, fmt.Sprintf("%v", v))
				break
			}
		}
	}
	Success(w, names)
}

// QuickCreateDatabase 创建数据库
func QuickCreateDatabase(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}
	if req.Name == "" {
		Error(w, 400, "数据库名不能为空")
		return
	}
	if conn.Type == model.DBTypeMySQL {
		sql := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s DEFAULT CHARACTER SET utf8mb4", quoteIdent(conn.Type, req.Name))
		driver, _ := db.GetManager().GetConnection(conn)
		if _, err := driver.Execute(sql, ""); err != nil {
			Error(w, 500, "创建数据库失败: "+err.Error())
			return
		}
		Success(w, "创建成功")
		return
	}
	if conn.Type == model.DBTypePostgres {
		// 通过连到默认库创建
		cfg := config.GetConfig()
		c := cfg.GetConnectionByID(conn.ID)
		tmp := *c
		tmp.Database = "postgres"
		driver, err := db.GetManager().GetConnection(tmp)
		if err != nil {
			Error(w, 500, "连接失败: "+err.Error())
			return
		}
		sql := fmt.Sprintf("CREATE DATABASE %s", quoteIdent(conn.Type, req.Name))
		if _, err := driver.Execute(sql, ""); err != nil {
			Error(w, 500, "创建数据库失败: "+err.Error())
			return
		}
		Success(w, "创建成功")
		return
	}
	Error(w, 400, "当前数据库类型不支持库操作")
}

// QuickDropDatabase 删除数据库
func QuickDropDatabase(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		Error(w, 400, "数据库名不能为空")
		return
	}
	if conn.Type == model.DBTypeMySQL {
		sql := fmt.Sprintf("DROP DATABASE IF EXISTS %s", quoteIdent(conn.Type, name))
		driver, _ := db.GetManager().GetConnection(conn)
		if _, err := driver.Execute(sql, ""); err != nil {
			Error(w, 500, "删除数据库失败: "+err.Error())
			return
		}
		Success(w, "删除成功")
		return
	}
	if conn.Type == model.DBTypePostgres {
		cfg := config.GetConfig()
		c := cfg.GetConnectionByID(conn.ID)
		tmp := *c
		tmp.Database = "postgres"
		driver, err := db.GetManager().GetConnection(tmp)
		if err != nil {
			Error(w, 500, "连接失败: "+err.Error())
			return
		}
		sql := fmt.Sprintf("DROP DATABASE IF EXISTS %s", quoteIdent(conn.Type, name))
		if _, err := driver.Execute(sql, ""); err != nil {
			Error(w, 500, "删除数据库失败: "+err.Error())
			return
		}
		Success(w, "删除成功")
		return
	}
	Error(w, 400, "当前数据库类型不支持库操作")
}

// ========== 表操作 ==========

// QuickCreateTable 创建表
func QuickCreateTable(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	var req model.CreateTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}
	if req.Table == "" || len(req.Columns) == 0 {
		Error(w, 400, "表名和列不能为空")
		return
	}

	// MongoDB: 创建集合
	if conn.Type == model.DBTypeMongo {
		if m, ok := db.GetMongoDriver(conn); ok {
			if err := m.CreateCollection(req.Table); err != nil {
				Error(w, 500, "创建集合失败: "+err.Error())
				return
			}
			Success(w, "创建成功")
			return
		}
	}

	if conn.Type != model.DBTypeMySQL && conn.Type != model.DBTypePostgres && conn.Type != model.DBTypeSQLite {
		Error(w, 400, "当前数据库类型不支持建表")
		return
	}

	var defs []string
	for _, col := range req.Columns {
		if col.Name == "" || col.Type == "" {
			continue
		}
		var def string
		colType := strings.ToUpper(col.Type)
		// 自增列类型适配
		if col.AutoIncrement {
			switch conn.Type {
			case model.DBTypePostgres:
				colType = "SERIAL"
			case model.DBTypeSQLite:
				colType = "INTEGER"
			}
		}
		def = quoteIdent(conn.Type, col.Name) + " " + colType
		if !col.Nullable && !col.AutoIncrement {
			def += " NOT NULL"
		}
		if col.DefaultValue != "" {
			def += " DEFAULT " + quoteValue(col.DefaultValue)
		}
		if col.AutoIncrement {
			switch conn.Type {
			case model.DBTypeMySQL:
				def += " AUTO_INCREMENT"
			case model.DBTypeSQLite:
				def += " PRIMARY KEY AUTOINCREMENT"
			}
		}
		if col.PrimaryKey && !(col.AutoIncrement && conn.Type == model.DBTypeSQLite) {
			def += " PRIMARY KEY"
		}
		defs = append(defs, def)
	}
	if len(defs) == 0 {
		Error(w, 400, "至少需要一列")
		return
	}

	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", quoteIdent(conn.Type, req.Table), strings.Join(defs, ", "))
	driver, _ := db.GetManager().GetConnection(conn)
	if _, err := driver.Execute(sql, ""); err != nil {
		Error(w, 500, "创建表失败: "+err.Error())
		return
	}
	Success(w, "创建成功")
}

// QuickDropTable 删除表
func QuickDropTable(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		Error(w, 400, "表名不能为空")
		return
	}
	if conn.Type == model.DBTypeMongo {
		if m, ok := db.GetMongoDriver(conn); ok {
			if err := m.DropCollection(name); err != nil {
				Error(w, 500, "删除集合失败: "+err.Error())
				return
			}
			Success(w, "删除成功")
			return
		}
	}
	if conn.Type != model.DBTypeMySQL && conn.Type != model.DBTypePostgres && conn.Type != model.DBTypeSQLite {
		Error(w, 400, "当前数据库类型不支持删表")
		return
	}
	sql := fmt.Sprintf("DROP TABLE IF EXISTS %s", quoteIdent(conn.Type, name))
	driver, _ := db.GetManager().GetConnection(conn)
	if _, err := driver.Execute(sql, ""); err != nil {
		Error(w, 500, "删除表失败: "+err.Error())
		return
	}
	Success(w, "删除成功")
}

// switchDB 切换当前数据库（MySQL USE / PG search_path）
func switchDB(conn model.DBConnection, driver db.DBDriver, dbName string) error {
	if dbName == "" {
		return nil
	}
	switch conn.Type {
	case model.DBTypeMySQL:
		_, err := driver.Execute("USE "+quoteIdent(conn.Type, dbName), "")
		return err
	case model.DBTypePostgres:
		_, err := driver.Execute("SET search_path TO "+quoteIdent(conn.Type, dbName), "")
		return err
	}
	return nil
}

// QuickListTables 列出表（支持指定数据库）
func QuickListTables(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	dbName := r.URL.Query().Get("db")
	if conn.Type != model.DBTypeMySQL && conn.Type != model.DBTypePostgres && conn.Type != model.DBTypeSQLite && conn.Type != model.DBTypeMongo {
		Error(w, 400, "当前数据库类型不支持表操作")
		return
	}
	driver, _ := db.GetManager().GetConnection(conn)
	if err := switchDB(conn, driver, dbName); err != nil {
		Error(w, 500, "切换数据库失败: "+err.Error())
		return
	}
	tables, err := driver.ListTables()
	if err != nil {
		Error(w, 500, "获取表列表失败: "+err.Error())
		return
	}
	if tables == nil {
		tables = []string{}
	}
	Success(w, tables)
}

// QuickRenameTable 表改名
func QuickRenameTable(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	var req struct {
		Table   string `json:"table"`
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}
	if req.Table == "" || req.NewName == "" {
		Error(w, 400, "表名和新表名不能为空")
		return
	}
	if conn.Type != model.DBTypeMySQL && conn.Type != model.DBTypePostgres && conn.Type != model.DBTypeSQLite {
		Error(w, 400, "当前数据库类型不支持表改名")
		return
	}
	dbName := r.URL.Query().Get("db")
	driver, _ := db.GetManager().GetConnection(conn)
	if err := switchDB(conn, driver, dbName); err != nil {
		Error(w, 500, "切换数据库失败: "+err.Error())
		return
	}
	var sql string
	if conn.Type == model.DBTypeMySQL {
		sql = fmt.Sprintf("RENAME TABLE %s TO %s", quoteIdent(conn.Type, req.Table), quoteIdent(conn.Type, req.NewName))
	} else {
		sql = fmt.Sprintf("ALTER TABLE %s RENAME TO %s", quoteIdent(conn.Type, req.Table), quoteIdent(conn.Type, req.NewName))
	}
	if _, err := driver.Execute(sql, ""); err != nil {
		Error(w, 500, "表改名失败: "+err.Error())
		return
	}
	Success(w, "改名成功")
}

// QuickGetColumns 获取表列信息（含主键标识）
func QuickGetColumns(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	table := r.URL.Query().Get("table")
	if table == "" {
		Error(w, 400, "表名不能为空")
		return
	}
	dbName := r.URL.Query().Get("db")
	driver, _ := db.GetManager().GetConnection(conn)
	if err := switchDB(conn, driver, dbName); err != nil {
		Error(w, 500, "切换数据库失败: "+err.Error())
		return
	}
	var result *model.QueryResult
	var err error
	switch conn.Type {
	case model.DBTypeMySQL:
		result, err = driver.Query(fmt.Sprintf("SHOW COLUMNS FROM %s", quoteIdent(conn.Type, table)), "")
	case model.DBTypePostgres:
		t := strings.ReplaceAll(table, "'", "''")
		sql := fmt.Sprintf(`SELECT c.column_name AS "Field", c.data_type AS "Type",
			CASE WHEN c.is_nullable = 'NO' THEN 'NO' ELSE 'YES' END AS "Null",
			COALESCE(c.column_default::text, '') AS "Default",
			CASE WHEN pk.column_name IS NOT NULL THEN 'PRI' ELSE '' END AS "Key"
			FROM information_schema.columns c
			LEFT JOIN (
				SELECT ku.column_name
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage ku
					ON tc.constraint_name = ku.constraint_name AND tc.table_name = ku.table_name
				WHERE tc.table_name = '%s' AND tc.constraint_type = 'PRIMARY KEY'
			) pk ON pk.column_name = c.column_name
			WHERE c.table_name = '%s'
			ORDER BY c.ordinal_position`, t, t)
		result, err = driver.Query(sql, "")
	case model.DBTypeSQLite:
		result, err = driver.Query(fmt.Sprintf("PRAGMA table_info(%s)", quoteIdent(conn.Type, table)), "")
	default:
		Error(w, 400, "当前数据库类型不支持获取列信息")
		return
	}
	if err != nil {
		Error(w, 500, "获取列信息失败: "+err.Error())
		return
	}
	Success(w, result)
}

// ========== 索引操作 ==========

// QuickListIndexes 列出索引
func QuickListIndexes(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	table := r.URL.Query().Get("table")
	if table == "" {
		Error(w, 400, "表名不能为空")
		return
	}
	if conn.Type == model.DBTypeMongo {
		if m, ok := db.GetMongoDriver(conn); ok {
			idx, err := m.ListIndexes(table)
			if err != nil {
				Error(w, 500, "获取索引失败: "+err.Error())
				return
			}
			Success(w, idx)
			return
		}
	}
	driver, _ := db.GetManager().GetConnection(conn)
	var result *model.QueryResult
	var err error
	switch conn.Type {
	case model.DBTypeMySQL:
		result, err = driver.Query(fmt.Sprintf("SHOW INDEX FROM %s", quoteIdent(conn.Type, table)), "")
	case model.DBTypePostgres:
		result, err = driver.Query(fmt.Sprintf("SELECT indexname, indexdef FROM pg_indexes WHERE tablename = '%s' ORDER BY indexname", strings.ReplaceAll(table, "'", "''")), "")
	case model.DBTypeSQLite:
		result, err = driver.Query(fmt.Sprintf("PRAGMA index_list(%s)", quoteIdent(conn.Type, table)), "")
	default:
		Error(w, 400, "当前数据库类型不支持索引操作")
		return
	}
	if err != nil {
		Error(w, 500, "获取索引失败: "+err.Error())
		return
	}
	Success(w, result)
}

// QuickCreateIndex 创建索引
func QuickCreateIndex(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	var req model.IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}
	if req.Table == "" || len(req.Columns) == 0 {
		Error(w, 400, "表名和索引列不能为空")
		return
	}
	if conn.Type == model.DBTypeMongo {
		if m, ok := db.GetMongoDriver(conn); ok {
			if err := m.CreateIndex(req.Table, req.IndexName, req.Columns, req.Unique); err != nil {
				Error(w, 500, "创建索引失败: "+err.Error())
				return
			}
			Success(w, "创建成功")
			return
		}
	}
	if conn.Type != model.DBTypeMySQL && conn.Type != model.DBTypePostgres && conn.Type != model.DBTypeSQLite {
		Error(w, 400, "当前数据库类型不支持索引操作")
		return
	}
	idxName := req.IndexName
	if idxName == "" {
		idxName = "idx_" + req.Table + "_" + strings.Join(req.Columns, "_")
	}
	unique := ""
	if req.Unique {
		unique = "UNIQUE "
	}
	var cols []string
	for _, c := range req.Columns {
		cols = append(cols, quoteIdent(conn.Type, c))
	}
	var sql string
	if conn.Type == model.DBTypeMySQL {
		sql = fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)",
			unique, quoteIdent(conn.Type, idxName), quoteIdent(conn.Type, req.Table), strings.Join(cols, ", "))
	} else {
		sql = fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s (%s)",
			unique, quoteIdent(conn.Type, idxName), quoteIdent(conn.Type, req.Table), strings.Join(cols, ", "))
	}
	driver, _ := db.GetManager().GetConnection(conn)
	if _, err := driver.Execute(sql, ""); err != nil {
		Error(w, 500, "创建索引失败: "+err.Error())
		return
	}
	Success(w, "创建成功")
}

// QuickDropIndex 删除索引
func QuickDropIndex(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	var req model.IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}
	if req.Table == "" || req.IndexName == "" {
		Error(w, 400, "表名和索引名不能为空")
		return
	}
	if conn.Type == model.DBTypeMongo {
		if m, ok := db.GetMongoDriver(conn); ok {
			if err := m.DropIndex(req.Table, req.IndexName); err != nil {
				Error(w, 500, "删除索引失败: "+err.Error())
				return
			}
			Success(w, "删除成功")
			return
		}
	}
	if conn.Type != model.DBTypeMySQL && conn.Type != model.DBTypePostgres && conn.Type != model.DBTypeSQLite {
		Error(w, 400, "当前数据库类型不支持索引操作")
		return
	}
	var sql string
	if conn.Type == model.DBTypeMySQL {
		sql = fmt.Sprintf("DROP INDEX %s ON %s", quoteIdent(conn.Type, req.IndexName), quoteIdent(conn.Type, req.Table))
	} else {
		sql = fmt.Sprintf("DROP INDEX IF EXISTS %s", quoteIdent(conn.Type, req.IndexName))
	}
	driver, _ := db.GetManager().GetConnection(conn)
	if _, err := driver.Execute(sql, ""); err != nil {
		Error(w, 500, "删除索引失败: "+err.Error())
		return
	}
	Success(w, "删除成功")
}

// ========== 行操作 ==========

// QuickListRows 查询数据
func QuickListRows(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	table := r.URL.Query().Get("table")
	limit := r.URL.Query().Get("limit")
	if table == "" {
		Error(w, 400, "表名不能为空")
		return
	}
	if limit == "" {
		limit = "100"
	}
	if _, err := strconv.Atoi(limit); err != nil || limit == "" {
		limit = "100"
	}
	driver, _ := db.GetManager().GetConnection(conn)
	if err := switchDB(conn, driver, r.URL.Query().Get("db")); err != nil {
		Error(w, 500, "切换数据库失败: "+err.Error())
		return
	}

	if conn.Type == model.DBTypeMongo {
		result, err := driver.Query("{}", table)
		if err != nil {
			Error(w, 500, "查询失败: "+err.Error())
			return
		}
		Success(w, result)
		return
	}
	if conn.Type == model.DBTypeRedis {
		result, err := driver.Query("", "")
		if err != nil {
			Error(w, 500, "查询失败: "+err.Error())
			return
		}
		Success(w, result)
		return
	}
	sql := fmt.Sprintf("SELECT * FROM %s LIMIT %s", quoteIdent(conn.Type, table), limit)
	result, err := driver.Query(sql, "")
	if err != nil {
		Error(w, 500, "查询失败: "+err.Error())
		return
	}
	Success(w, result)
}

// QuickInsertRow 插入行
func QuickInsertRow(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	var req model.RowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}
	if req.Table == "" || len(req.Data) == 0 {
		Error(w, 400, "表名和数据不能为空")
		return
	}
	driver, _ := db.GetManager().GetConnection(conn)

	if conn.Type == model.DBTypeMongo {
		if m, ok := db.GetMongoDriver(conn); ok {
			if err := m.Insert(req.Table, req.Data); err != nil {
				Error(w, 500, "插入失败: "+err.Error())
				return
			}
			Success(w, "插入成功")
			return
		}
	}
	if conn.Type == model.DBTypeRedis {
		for k, v := range req.Data {
			result, err := driver.Query(fmt.Sprintf("SET %s %s", k, quoteValue(v)), "")
			if err != nil {
				Error(w, 500, "写入失败: "+err.Error())
				return
			}
			_ = result
		}
		Success(w, "写入成功")
		return
	}
	if conn.Type != model.DBTypeMySQL && conn.Type != model.DBTypePostgres && conn.Type != model.DBTypeSQLite {
		Error(w, 400, "当前数据库类型不支持行操作")
		return
	}
	var cols, vals []string
	for k, v := range req.Data {
		cols = append(cols, quoteIdent(conn.Type, k))
		vals = append(vals, quoteValue(v))
	}
	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteIdent(conn.Type, req.Table), strings.Join(cols, ", "), strings.Join(vals, ", "))
	if _, err := driver.Execute(sql, ""); err != nil {
		Error(w, 500, "插入失败: "+err.Error())
		return
	}
	Success(w, "插入成功")
}

// QuickUpdateRows 更新行
func QuickUpdateRows(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	var req model.RowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}
	if req.Table == "" || len(req.Data) == 0 {
		Error(w, 400, "表名和更新数据不能为空")
		return
	}
	if conn.Type == model.DBTypeMongo {
		if m, ok := db.GetMongoDriver(conn); ok {
			if err := m.Update(req.Table, req.Where, req.Data); err != nil {
				Error(w, 500, "更新失败: "+err.Error())
				return
			}
			Success(w, "更新成功")
			return
		}
	}
	if conn.Type != model.DBTypeMySQL && conn.Type != model.DBTypePostgres && conn.Type != model.DBTypeSQLite {
		Error(w, 400, "当前数据库类型不支持行操作")
		return
	}
	var sets []string
	for k, v := range req.Data {
		sets = append(sets, fmt.Sprintf("%s = %s", quoteIdent(conn.Type, k), quoteValue(v)))
	}
	where, err := buildWhere(conn.Type, req.Where)
	if err != nil {
		Error(w, 400, "更新条件: "+err.Error())
		return
	}
	sql := fmt.Sprintf("UPDATE %s SET %s WHERE %s", quoteIdent(conn.Type, req.Table), strings.Join(sets, ", "), where)
	if _, err := driverExecute(conn, sql); err != nil {
		Error(w, 500, "更新失败: "+err.Error())
		return
	}
	Success(w, "更新成功")
}

// QuickDeleteRows 删除行
func QuickDeleteRows(w http.ResponseWriter, r *http.Request) {
	conn, _ := getQuickConnection(w, quickParseID(r, "/api/quick/"))
	if conn.ID == "" {
		return
	}
	var req model.RowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 400, "参数错误: "+err.Error())
		return
	}
	if req.Table == "" {
		Error(w, 400, "表名不能为空")
		return
	}
	if conn.Type == model.DBTypeMongo {
		if m, ok := db.GetMongoDriver(conn); ok {
			if err := m.Delete(req.Table, req.Where); err != nil {
				Error(w, 500, "删除失败: "+err.Error())
				return
			}
			Success(w, "删除成功")
			return
		}
	}
	if conn.Type == model.DBTypeRedis {
		driver, _ := db.GetManager().GetConnection(conn)
		for k := range req.Where {
			if _, err := driver.Query(fmt.Sprintf("DEL %s", k), ""); err != nil {
				Error(w, 500, "删除失败: "+err.Error())
				return
			}
		}
		Success(w, "删除成功")
		return
	}
	if conn.Type != model.DBTypeMySQL && conn.Type != model.DBTypePostgres && conn.Type != model.DBTypeSQLite {
		Error(w, 400, "当前数据库类型不支持行操作")
		return
	}
	where, err := buildWhere(conn.Type, req.Where)
	if err != nil {
		Error(w, 400, "删除条件: "+err.Error())
		return
	}
	sql := fmt.Sprintf("DELETE FROM %s WHERE %s", quoteIdent(conn.Type, req.Table), where)
	if _, err := driverExecute(conn, sql); err != nil {
		Error(w, 500, "删除失败: "+err.Error())
		return
	}
	Success(w, "删除成功")
}

// driverExecute 便捷执行
func driverExecute(conn model.DBConnection, sql string) (*model.QueryResult, error) {
	driver, _ := db.GetManager().GetConnection(conn)
	return driver.Execute(sql, "")
}
