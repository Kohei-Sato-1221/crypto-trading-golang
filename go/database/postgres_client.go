package database

import (
	"database/sql"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// PostgresClient はPostgreSQL用のDBClient実装。
type PostgresClient struct {
	dsn    string
	db     *sql.DB
	gormDB *gorm.DB
}

// NewPostgresClient は新しいPostgreSQLクライアントを生成する。
func NewPostgresClient(dsn string) *PostgresClient {
	return &PostgresClient{dsn: dsn}
}

func (c *PostgresClient) Connect() error {
	db, err := gorm.Open(postgres.Open(c.dsn), &gorm.Config{})
	if err != nil {
		log.Printf("【ERROR】Failed to open PostgreSQL connection: %v", err)
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("【ERROR】Failed to get SQL DB from GORM: %v", err)
		return err
	}

	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		log.Printf("【ERROR】Failed to ping PostgreSQL: %v", err)
		sqlDB.Close()
		return err
	}

	log.Println("Ping OK!")
	log.Println("Successfully got PostgreSQL DB connection!!")

	c.db = sqlDB
	c.gormDB = db
	return nil
}

func (c *PostgresClient) GetDB() *sql.DB {
	return c.db
}

func (c *PostgresClient) GetGormDB() *gorm.DB {
	return c.gormDB
}

func (c *PostgresClient) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func (c *PostgresClient) DriverName() string {
	return "postgres"
}
