package models

import (
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

var MysqlDbConn *sql.DB

func init() {
	var err error
	MysqlDbConn, err = sql.Open("mysql", "trading:trading1221!@tcp(192.168.0.16:3306)/trading")
	if err != nil {
		log.Fatalln(err)
		log.Println(err)
	} else {
		log.Println("Successfully got MySQL DB connection!!")
	}
}
