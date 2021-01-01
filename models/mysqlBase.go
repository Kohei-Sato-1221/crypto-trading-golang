package models

import (
	"database/sql"
	"log"

	"gorm.io/driver/mysql"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/config"
	_ "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var AppDB *sql.DB
var GormDB *gorm.DB

func NewMysqlBase() {
	db, err := gorm.Open(mysql.Open(config.Config.MySql), &gorm.Config{})
	if err != nil {
		panic(err.Error())
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic(err.Error())
	}
	err = sqlDB.Ping()
	if err != nil {
		panic(err.Error())
	} else {
		log.Println("Ping OK!")
	}
	log.Println("Successfully got MySQL DB connection!!")
	AppDB = sqlDB
	GormDB = db
}
