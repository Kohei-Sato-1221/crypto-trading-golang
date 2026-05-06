package database

import (
	"database/sql"

	"gorm.io/gorm"
)

// DBClient はデータベース接続を抽象化するinterface。
// MySQL/PostgreSQLの切り替えを可能にする。
type DBClient interface {
	// Connect はデータベースに接続する。
	Connect() error
	// GetDB は標準のsql.DBインスタンスを返す（rawクエリ用）。
	GetDB() *sql.DB
	// GetGormDB はGORMインスタンスを返す（ORM操作用）。
	GetGormDB() *gorm.DB
	// Close はデータベース接続を閉じる。
	Close() error
	// DriverName は使用中のドライバ名を返す（"mysql" or "postgres"）。
	DriverName() string
}

// Current は現在使用中のDBClientインスタンス。
// アプリケーション起動時に一度だけセットされ、以降は読み取り専用として扱う。
var Current DBClient

// CurrentDriver は現在のドライバ名を返すヘルパー。
func CurrentDriver() string {
	if Current == nil {
		return ""
	}
	return Current.DriverName()
}
