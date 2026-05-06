package database

import (
	"database/sql"
	"log"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// MysqlClient はMySQL用のDBClient実装。
type MysqlClient struct {
	dsn    string
	db     *sql.DB
	gormDB *gorm.DB
}

// NewMysqlClient は新しいMySQLクライアントを生成する。
func NewMysqlClient(dsn string) *MysqlClient {
	return &MysqlClient{dsn: dsn}
}

func (c *MysqlClient) Connect() error {
	dsn := c.dsn
	if !strings.Contains(dsn, "timeout") {
		if strings.Contains(dsn, "?") {
			dsn += "&timeout=10s&readTimeout=10s&writeTimeout=10s"
		} else {
			dsn += "?timeout=10s&readTimeout=10s&writeTimeout=10s"
		}
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("【ERROR】Failed to open MySQL connection: %v", err)
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("【ERROR】Failed to get SQL DB from GORM: %v", err)
		return err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		log.Printf("【ERROR】Failed to ping MySQL: %v", err)
		sqlDB.Close()
		return err
	}

	log.Println("Ping OK!")
	log.Println("Successfully got MySQL DB connection!!")

	c.db = sqlDB
	c.gormDB = db
	return nil
}

func (c *MysqlClient) GetDB() *sql.DB {
	return c.db
}

func (c *MysqlClient) GetGormDB() *gorm.DB {
	return c.gormDB
}

func (c *MysqlClient) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func (c *MysqlClient) DriverName() string {
	return "mysql"
}
