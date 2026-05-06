package models

import (
	"log"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/database"
)

// InitDB はconfigのDBDriver設定に基づいてデータベース接続を初期化する。
// "postgres" の場合はPostgreSQLに、それ以外（空文字含む）はMySQLに接続する。
func InitDB() {
	var client database.DBClient

	switch config.Config.DBDriver {
	case "postgres":
		client = database.NewPostgresClient(config.Config.Postgres)
	default:
		client = database.NewMysqlClient(config.Config.MySql)
	}

	if err := client.Connect(); err != nil {
		log.Printf("【ERROR】Failed to connect to database: %v", err)
		panic(err)
	}

	database.Current = client
	AppDB = client.GetDB()
	GormDB = client.GetGormDB()
}
