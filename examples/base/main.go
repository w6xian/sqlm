package main

import (
	"fmt"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

func main() {
	sqlsvr := sqlm.Server{
		Database: "cloud",
		Host:     "127.0.0.1",
		Port:     3306,
		Protocol: "mysql",
		Username: "root",
		Password: "1Qazxsw2",
		Pretable: "mi_",
		Charset:  "utf8mb4",
	}
	sqlc := sqlm.NewOptionsWithServer(sqlsvr)

	// 使用mysql
	con, err := store.NewMysql(&sqlc.Server)
	if err != nil {
		fmt.Println("not conne")
	}

	sqlm.New(sqlc, con)

	db := sqlm.Master()

	// return nil
	// 操作表
	row, err := db.Table("mall_so").Where("id=%d", 1).Query()
	fmt.Printf("%v", err)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(row.Get("com_name").String())
	}

	code, err := db.Action(func(tx *sqlm.Tx, args ...interface{}) (int64, error) {
		rows, err := tx.Table("mall_so").Where("proxy_id=%d", 2).Limit(0, 10).QueryMulti()
		if err != nil {
			fmt.Println(err.Error())
		}
		for rows.Next() != nil {
			fmt.Println(rows.Get("com_name").String())
		}
		pos, err := tx.Table("cloud_mark").Insert(sqlm.KeyValue{
			"com_id":  161,
			"prd_pos": 1,
		})
		fmt.Printf("%v,$v", pos, err)
		return 1, nil
	})
	fmt.Println(code)
}
