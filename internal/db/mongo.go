package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"dbmanager/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDriver MongoDB驱动
type MongoDriver struct {
	client *mongo.Client
	db     *mongo.Database
}

func (d *MongoDriver) Connect(conn model.DBConnection) error {
	uri := fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin",
		conn.Username, conn.Password, conn.Host, conn.Port, conn.Database)
	// 如果没有用户名密码，使用简化连接
	if conn.Username == "" {
		uri = fmt.Sprintf("mongodb://%s:%d/%s", conn.Host, conn.Port, conn.Database)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}

	d.client = client
	d.db = client.Database(conn.Database)
	return nil
}

func (d *MongoDriver) Close() error {
	if d.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return d.client.Disconnect(ctx)
	}
	return nil
}

func (d *MongoDriver) Ping() (time.Duration, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := d.client.Ping(ctx, nil)
	return time.Since(start), err
}

func (d *MongoDriver) Query(sql string, collection string) (*model.QueryResult, error) {
	if collection == "" {
		return nil, fmt.Errorf("MongoDB查询需要指定集合名")
	}

	var filter bson.M
	if sql != "" {
		if err := json.Unmarshal([]byte(sql), &filter); err != nil {
			return nil, fmt.Errorf("查询条件JSON解析失败: %v", err)
		}
	}
	if filter == nil {
		filter = bson.M{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cursor, err := d.db.Collection(collection).Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []interface{}
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		// 转换ObjectID为字符串
		if id, ok := doc["_id"]; ok {
			doc["_id"] = fmt.Sprintf("%v", id)
		}
		results = append(results, doc)
	}

	return &model.QueryResult{
		Success: true,
		Columns: getMongoColumns(results),
		Rows:    results,
		Count:   int64(len(results)),
	}, nil
}

func (d *MongoDriver) Execute(sql string, collection string) (*model.QueryResult, error) {
	if collection == "" {
		return nil, fmt.Errorf("MongoDB操作需要指定集合名")
	}

	var doc bson.M
	if sql != "" {
		if err := json.Unmarshal([]byte(sql), &doc); err != nil {
			return nil, fmt.Errorf("文档JSON解析失败: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := d.db.Collection(collection).InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}

	return &model.QueryResult{
		Success: true,
		Message: fmt.Sprintf("插入成功，ID: %v", result.InsertedID),
		Count:   1,
	}, nil
}

func (d *MongoDriver) ListTables() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collections, err := d.db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	return collections, nil
}

// CreateCollection 创建集合
func (d *MongoDriver) CreateCollection(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.db.CreateCollection(ctx, name)
}

// DropCollection 删除集合
func (d *MongoDriver) DropCollection(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.db.Collection(name).Drop(ctx)
}

// Insert 插入文档
func (d *MongoDriver) Insert(collection string, data map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	doc := toBSON(data)
	_, err := d.db.Collection(collection).InsertOne(ctx, doc)
	return err
}

// Update 更新文档（where为过滤条件）
func (d *MongoDriver) Update(collection string, where, data map[string]interface{}) error {
	if len(where) == 0 {
		return fmt.Errorf("需要指定条件")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	filter := toBSON(where)
	update := bson.M{"$set": toBSON(data)}
	_, err := d.db.Collection(collection).UpdateMany(ctx, filter, update)
	return err
}

// Delete 删除文档
func (d *MongoDriver) Delete(collection string, where map[string]interface{}) error {
	if len(where) == 0 {
		return fmt.Errorf("需要指定条件")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	filter := toBSON(where)
	_, err := d.db.Collection(collection).DeleteMany(ctx, filter)
	return err
}

// ListIndexes 列出集合索引
func (d *MongoDriver) ListIndexes(collection string) ([]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cursor, err := d.db.Collection(collection).Indexes().List(ctx)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var result []interface{}
	for cursor.Next(ctx) {
		var idx bson.M
		if err := cursor.Decode(&idx); err != nil {
			return nil, err
		}
		result = append(result, idx)
	}
	return result, nil
}

// CreateIndex 创建索引
func (d *MongoDriver) CreateIndex(collection, name string, columns []string, unique bool) error {
	if len(columns) == 0 {
		return fmt.Errorf("索引列不能为空")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	keys := bson.D{}
	for _, c := range columns {
		keys = append(keys, bson.E{Key: c, Value: 1})
	}
	opts := options.Index()
	if name != "" {
		opts.SetName(name)
	}
	if unique {
		opts.SetUnique(true)
	}
	_, err := d.db.Collection(collection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    keys,
		Options: opts,
	})
	return err
}

// DropIndex 删除索引
func (d *MongoDriver) DropIndex(collection, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := d.db.Collection(collection).Indexes().DropOne(ctx, name)
	return err
}

// toBSON map转bson.M
func toBSON(m map[string]interface{}) bson.M {
	doc := bson.M{}
	for k, v := range m {
		doc[k] = v
	}
	return doc
}

// GetMongoDriver 获取Mongo驱动（供handler类型断言使用）
func GetMongoDriver(conn model.DBConnection) (*MongoDriver, bool) {
	manager := GetManager()
	manager.mu.RLock()
	pc, ok := manager.pool[conn.ID]
	manager.mu.RUnlock()
	if !ok {
		return nil, false
	}
	d, ok := pc.Conn.(*MongoDriver)
	return d, ok
}

func getMongoColumns(docs []interface{}) []string {
	if len(docs) == 0 {
		return nil
	}
	if doc, ok := docs[0].(bson.M); ok {
		cols := make([]string, 0, len(doc))
		for k := range doc {
			cols = append(cols, k)
		}
		return cols
	}
	return nil
}
