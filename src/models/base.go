package models

import (
	"time"
	"fmt"
	"log"
	"database/sql"
	"config"
	_ "github.com/mattn/go-sqlite3"
)

const (
	tableNameSignalEvents = "signal_events"
)

var DbConnection *sql.DB

func GetCandleTableName(productCode string, duration time.Duration) string {
	return fmt.Sprintf("%s_%s", productCode, duration)
}

func init(){
	var err error
	DbConnection, err = sql.Open(config.Config.SQLDriver, config.Config.DbName)
	if err != nil {
		log.Fatalln(err)
		log.Println(err)
	}else{
		log.Println("Successfully got DB connection!!")
	}
	
	// filled 0:買い注文発注済 1:買い注文約定済み 2:対応する売り注文発注済
	cmd := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS buy_orders (
			orderid STRING,
			time DATETIME NOT NULL,
			product_code STRING,
			side STRING,
			price FLOAT,
			size FLOAT,
			exchange STRING,
			filled INTEGER DEFAULT 0)`)
	DbConnection.Exec(cmd)

	// filled 0:売り注文発注 1:売り注文約定済み
	cmd = fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS sell_orders (
			parentid STRING,
			orderid STRING,
			time DATETIME NOT NULL,
			product_code STRING,
			side STRING,
			price FLOAT,
			size FLOAT,
			exchange STRING,
			filled INTEGER DEFAULT 0)`)
	DbConnection.Exec(cmd)
	
	cmd  = fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			time DATETIME PRIMARY KEY NOT NULL,
			product_code STRING,
			side STRING,
			price FLOAT,
			size FLOAT)`, tableNameSignalEvents)
	DbConnection.Exec(cmd)
	
	for _, duration := range config.Config.Durations {
		tableName := GetCandleTableName(config.Config.ProductCode, duration)
		c := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				time DATETIME PRIMARY KEY NOT NULL,
				open FLOAT,
				close FLOAT,
				high FLOAT,
				low open FLOAT,
				volume FLOAT)`, tableName)
		DbConnection.Exec(c)
	}
}
