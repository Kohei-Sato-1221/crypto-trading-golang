package models

import (
	"log"
	"database/sql"
	"config"
	_ "github.com/mattn/go-sqlite3"
)

var DbConnection *sql.DB

func init(){
	var err error
	DbConnection, err = sql.Open(config.Config.SQLDriver, config.Config.DbName)
	if err != nil {
		log.Fatalln(err)
	}
}
