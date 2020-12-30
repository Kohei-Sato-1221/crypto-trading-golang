package models

import (
	"database/sql"
	"log"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/config"
	_ "github.com/go-sql-driver/mysql"
)

var MysqlDbConn *sql.DB

func NewMysqlBase() {
	db, err := sql.Open("mysql", config.Config.MySql)
	log.Println("config:" + config.Config.MySql)
	if err != nil {
		panic(err.Error())
	}
	err = db.Ping()
	if err != nil {
		panic(err.Error())
	} else {
		log.Println("Ping OK!")
	}
	log.Println("Successfully got MySQL DB connection!!")
	MysqlDbConn = db
}
