package config

import (
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func InitDB() {
	var err error
	// 更新数据库连接信息，密码为 123456
	DB, err = sql.Open("mysql", "root:12345678@tcp(localhost:3306)/duanmu_db?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		log.Fatal("数据库连接失败：", err)
	}

	// 测试数据库连接
	err = DB.Ping()
	if err != nil {
		log.Fatal("数据库连接测试失败：", err)
	}

	log.Println("数据库连接成功！")
}
