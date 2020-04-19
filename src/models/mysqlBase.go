package models

import (
	"config"
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

var MysqlDbConn *sql.DB

func init() {
	db, err := sql.Open("mysql", config.Config.MySql)
	log.Println("config:" + config.Config.MySql)
	if err != nil {
		panic(err.Error())
	}

	err2 := db.Ping()
	if err2 != nil {
		panic(err2.Error())
	} else {
		log.Println("Ping OK!")
	}
	log.Println("Successfully got MySQL DB connection!!")
	MysqlDbConn = db
}
