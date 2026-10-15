package mongo

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const connectTimeout = 15 * time.Second

func clientFromConn(conn *Conn) (*mongo.Client, error) {
	uri := conn.URI
	if uri == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return client, nil
}

// normalizeValue 将 BSON 值转为 JSON 友好类型
func normalizeValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case primitive.ObjectID:
		return val.Hex()
	case primitive.DateTime:
		return time.UnixMilli(int64(val)).Format("2006-01-02 15:04:05")
	case primitive.D:
		m := make(map[string]interface{}, len(val))
		for _, e := range val {
			m[e.Key] = normalizeValue(e.Value)
		}
		return m
	case primitive.A:
		arr := make([]interface{}, len(val))
		for i, item := range val {
			arr[i] = normalizeValue(item)
		}
		return arr
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, vv := range val {
			out[k] = normalizeValue(vv)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, vv := range val {
			out[i] = normalizeValue(vv)
		}
		return out
	default:
		return v
	}
}

// ListConnections GET /api/mongo/connections
func ListConnections(c *gin.Context) {
	list, err := ListConns()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"list": list})
}

// SaveConnection POST /api/mongo/connections
func SaveConnection(c *gin.Context) {
	var conn Conn
	if err := c.ShouldBindJSON(&conn); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if conn.Name == "" || conn.URI == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and uri required"})
		return
	}
	if err := SaveConn(conn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": conn.ID})
}

// DeleteConnection DELETE /api/mongo/connections/:id
func DeleteConnection(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	if err := DeleteConn(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TreeItem 树节点
type TreeItem struct {
	ID       string      `json:"id"`
	Label    string      `json:"label"`
	Type     string      `json:"type"` // database / collection
	Children []*TreeItem `json:"children,omitempty"`
}

// GetTree GET /api/mongo/tree?connectionId=xxx
func GetTree(c *gin.Context) {
	connID := c.Query("connectionId")
	if connID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connectionId required"})
		return
	}
	conn, err := GetConn(connID)
	if err != nil || conn == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connection not found"})
		return
	}
	client, err := clientFromConn(conn)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connect failed: " + err.Error()})
		return
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = client.Disconnect(ctx)
		cancel()
	}()

	ctx := context.Background()
	names, err := client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var databases []*TreeItem
	for _, name := range names {
		if name == "admin" || name == "local" || name == "config" {
			continue
		}
		db := client.Database(name)
		colls, err := db.ListCollectionNames(ctx, bson.M{})
		if err != nil {
			colls = nil
		}
		children := make([]*TreeItem, 0, len(colls))
		for _, c := range colls {
			children = append(children, &TreeItem{ID: name + "." + c, Label: c, Type: "collection"})
		}
		databases = append(databases, &TreeItem{ID: name, Label: name, Type: "database", Children: children})
	}
	c.JSON(http.StatusOK, gin.H{"databases": databases})
}

// QueryRequest 查询请求
type QueryRequest struct {
	ConnectionID string                 `json:"connectionId"`
	Database     string                 `json:"database"`
	Collection   string                 `json:"collection"`
	Filter       map[string]interface{} `json:"filter"` // find 过滤，空则 {}
	Limit        int64                  `json:"limit"`
	Skip         int64                  `json:"skip"`
	Sort         map[string]int         `json:"sort"` // 如 {"_id": -1}
}

// QueryResult POST /api/mongo/query
func QueryResult(c *gin.Context) {
	var req QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ConnectionID == "" || req.Database == "" || req.Collection == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connectionId, database, collection required"})
		return
	}
	conn, err := GetConn(req.ConnectionID)
	if err != nil || conn == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connection not found"})
		return
	}
	client, err := clientFromConn(conn)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connect failed: " + err.Error()})
		return
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = client.Disconnect(ctx)
		cancel()
	}()

	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.Limit > 5000 {
		req.Limit = 5000
	}
	if req.Filter == nil {
		req.Filter = bson.M{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	coll := client.Database(req.Database).Collection(req.Collection)
	opts := options.Find().SetLimit(req.Limit).SetSkip(req.Skip)
	if len(req.Sort) > 0 {
		opts.SetSort(req.Sort)
	}
	cursor, err := coll.Find(ctx, req.Filter, opts)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error(), "columns": nil, "rows": nil})
		return
	}
	defer cursor.Close(ctx)

	var rows []map[string]interface{}
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		row := make(map[string]interface{})
		for k, v := range doc {
			row[k] = normalizeValue(v)
		}
		rows = append(rows, row)
	}
	if err := cursor.Err(); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error(), "columns": nil, "rows": nil})
		return
	}

	// 列名：所有出现过的 key 的并集，按首次出现顺序
	colSet := make(map[string]int)
	var columns []string
	for _, row := range rows {
		for k := range row {
			if _, ok := colSet[k]; !ok {
				colSet[k] = len(columns)
				columns = append(columns, k)
			}
		}
	}
	// 转为行数组，与 SQL 结果格式一致
	tableRows := make([][]interface{}, len(rows))
	for i, row := range rows {
		tableRows[i] = make([]interface{}, len(columns))
		for j, col := range columns {
			tableRows[i][j] = row[col]
		}
	}
	c.JSON(http.StatusOK, gin.H{"columns": columns, "rows": tableRows})
}
