package models

import (
	"database/sql"
	"log"
	"strings"
	"time"

	"gorm.io/driver/mysql"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	_ "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	AppDB  *sql.DB
	GormDB *gorm.DB
)

func NewMysqlBase() {
	// 接続文字列にタイムアウトパラメータを追加（既に存在する場合は追加しない）
	dsn := config.Config.MySql
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
		panic(err.Error())
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("【ERROR】Failed to get SQL DB: %v", err)
		panic(err.Error())
	}

	// 接続プールの設定
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 接続テスト（タイムアウト付き）
	err = sqlDB.Ping()
	if err != nil {
		log.Printf("【ERROR】Failed to ping MySQL: %v", err)
		panic(err.Error())
	} else {
		log.Println("Ping OK!")
	}
	log.Println("Successfully got MySQL DB connection!!")
	AppDB = sqlDB
	GormDB = db
}
