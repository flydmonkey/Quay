package db

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/gin-gonic/gin"
)

func dsn(c *Conn, database string) string {
	port := c.Port
	if port == 0 {
		if c.Driver == "mysql" {
			port = 3306
		} else {
			port = 5432
		}
	}
	dbName := database
	if dbName == "" {
		dbName = c.Database
	}
	switch c.Driver {
	case "mysql":
		return c.User + ":" + c.Password + "@tcp(" + c.Host + ":" + strconv.Itoa(port) + ")/" + dbName + "?charset=utf8mb4&parseTime=True"
	case "postgres", "postgresql":
		dsn := "host=" + c.Host + " port=" + strconv.Itoa(port) + " user=" + c.User + " password=" + c.Password + " sslmode=disable"
		if dbName != "" {
			dsn += " dbname=" + dbName
		}
		return dsn
	default:
		return ""
	}
}

func openDB(conn *Conn, database string) (*sql.DB, error) {
	if database == "" && conn.Database == "" {
		if conn.Driver == "mysql" {
			database = "mysql"
		} else {
			database = "postgres"
		}
	}
	return sql.Open(conn.Driver, dsn(conn, database))
}

// TreeItem 树节点
type TreeItem struct {
	ID       string      `json:"id"`
	Label    string      `json:"label"`
	Type     string      `json:"type"` // database / table
	Children []*TreeItem `json:"children,omitempty"`
}

// ListConnections GET /api/db/connections
func ListConnections(c *gin.Context) {
	list, err := ListConns()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"list": list})
}

// SaveConnection POST /api/db/connections
func SaveConnection(c *gin.Context) {
	var conn Conn
	if err := c.ShouldBindJSON(&conn); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if conn.Name == "" || conn.Driver == "" || conn.Host == "" || conn.User == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, driver, host, user required"})
		return
	}
	if conn.Driver != "mysql" && conn.Driver != "postgres" && conn.Driver != "postgresql" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "driver must be mysql or postgres"})
		return
	}
	if conn.Port == 0 {
		if conn.Driver == "mysql" {
			conn.Port = 3306
		} else {
			conn.Port = 5432
		}
	}
	if err := SaveConn(conn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": conn.ID})
}

// DeleteConnection DELETE /api/db/connections/:id
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

// GetTree GET /api/db/tree?connectionId=xxx
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
	db, err := openDB(conn, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connect failed: " + err.Error()})
		return
	}
	var databases []*TreeItem
	if conn.Driver == "mysql" {
		rows, err := db.QueryContext(ctx, "SHOW DATABASES")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				continue
			}
			if name == "information_schema" || name == "performance_schema" || name == "sys" {
				continue
			}
			tables, _ := getMySQLTables(db, name)
			databases = append(databases, &TreeItem{ID: name, Label: name, Type: "database", Children: tables})
		}
	} else {
		rows, err := db.QueryContext(ctx, "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				continue
			}
			tables, _ := getPGTables(conn, name)
			databases = append(databases, &TreeItem{ID: name, Label: name, Type: "database", Children: tables})
		}
	}
	c.JSON(http.StatusOK, gin.H{"databases": databases})
}

func getMySQLTables(db *sql.DB, database string) ([]*TreeItem, error) {
	_, err := db.Exec("USE `" + database + "`")
	if err != nil {
		return nil, err
	}
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*TreeItem
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		list = append(list, &TreeItem{ID: database + "." + name, Label: name, Type: "table"})
	}
	return list, nil
}

func getPGTables(conn *Conn, database string) ([]*TreeItem, error) {
	db, err := openDB(conn, database)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query("SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*TreeItem
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		list = append(list, &TreeItem{ID: database + "." + name, Label: name, Type: "table"})
	}
	return list, nil
}

// normalizeCell 将驱动返回的各种类型转为 JSON 可序列化的 string/number/bool/nil，便于前端展示
func normalizeCell(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []byte:
		return string(val)
	case time.Time:
		return val.Format("2006-01-02 15:04:05")
	case *sql.NullString:
		if val.Valid {
			return val.String
		}
		return nil
	case *sql.NullInt64:
		if val.Valid {
			return val.Int64
		}
		return nil
	case *sql.NullFloat64:
		if val.Valid {
			return val.Float64
		}
		return nil
	case *sql.NullBool:
		if val.Valid {
			return val.Bool
		}
		return nil
	case *sql.NullTime:
		if val.Valid {
			return val.Time.Format("2006-01-02 15:04:05")
		}
		return nil
	default:
		return v
	}
}

// QueryRequest 执行 SQL 请求
type QueryRequest struct {
	ConnectionID string `json:"connectionId"`
	Database     string `json:"database"`
	SQL          string `json:"sql"`
}

// QueryResult 查询结果
func QueryResult(c *gin.Context) {
	var req QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ConnectionID == "" || req.SQL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connectionId and sql required"})
		return
	}
	conn, err := GetConn(req.ConnectionID)
	if err != nil || conn == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connection not found"})
		return
	}
	db, err := openDB(conn, req.Database)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connect failed: " + err.Error()})
		return
	}
	rows, err := db.QueryContext(ctx, req.SQL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error(), "columns": nil, "rows": nil})
		return
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	columns := make([]string, len(cols))
	copy(columns, cols)
	rawRows := make([][]interface{}, 0)
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		row := make([]interface{}, len(cols))
		for i, v := range vals {
			row[i] = normalizeCell(v)
		}
		rawRows = append(rawRows, row)
	}
	c.JSON(http.StatusOK, gin.H{"columns": columns, "rows": rawRows})
}
