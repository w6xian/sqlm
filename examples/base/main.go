package main

import (
	"context"
	"fmt"
	"time"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

type City struct {
	Id   int
	Name string
}

func main() {

	// db, err := sql.Open("mysql", "root:1Qazxsw2@tcp(127.0.0.1:3306)/cloud")
	// defer db.Close()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// var myid int = 2

	// res, err := db.Query("SELECT id,com_name FROM mi_mall_so WHERE id = ?", myid)
	// defer res.Close()

	// if err != nil {
	// 	log.Fatal(err)
	// }

	// if res.Next() {

	// 	var city City
	// 	err := res.Scan(&city.Id, &city.Name)

	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}

	// 	fmt.Printf("%v\n", city)
	// } else {

	// 	fmt.Println("No city found")
	// }
	// os.Exit(0)
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Database:     "cloud",
		Host:         "127.0.0.1",
		Port:         3306,
		Protocol:     "sqlite",
		Username:     "root",
		Password:     "1Qazxsw2",
		Pretable:     "mi_",
		Charset:      "utf8mb4",
		MaxOpenConns: 64,
		MaxIdleConns: 64,
		MaxLifetime:  int(time.Second) * 60,
	})
	opt.SetLogger(&bLog{Prefix: "[ABC]", Level: 8})
	if err != nil {
		fmt.Println("not conne")
	}
	// 使用mysql
	con, err := store.NewDriver(opt)
	if err != nil {
		fmt.Println("not conne", err.Error())
	}
	fmt.Println(opt.Mode)

	sqlm.New(opt, con)

	db := sqlm.Major(context.Background())
	defer db.Close()

	mRow, err := db.QueryMulti("SELECT id,parent_track,user_name FROM mi_mall_so WHERE id=? limit 10", 1)
	if err == nil {
		fmt.Println(mRow.Next().Get("user_name").String())
	}

	sRow, err := db.Query("SELECT id,parent_track,user_name FROM mi_mall_so WHERE id = ?", 1)
	if err == nil {
		fmt.Println(sRow.Get("user_name").String())
	}

	query, _ := db.Conn()
	// myid := 2
	res, err := query.Query("SELECT id,parent_track,user_name FROM mi_mall_so WHERE id = ?", 1)
	if err == nil {
		row, err := sqlm.GetRow(res)
		if err == nil {
			fmt.Println(row.Get("user_name").String())
		}
	}

	row, err := db.Table("mall_so").Where("id=%d", 1).Query()

	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(row.Get("com_name").String())
	}

	code, err := db.Action(func(tx *sqlm.Tx, args ...interface{}) (int64, error) {

		rows, err := tx.Table("mall_so").Select("id", "com_name").Where("proxy_id=%d", 2).Limit(0, 10).QueryMulti()
		if err != nil {
			fmt.Println(err.Error())
		}
		for rows.Next() != nil {
			fmt.Println("com_name", rows.Get("com_name").String())
		}
		_, err = tx.Table("cloud_mark").Insert(sqlm.KeyValue{
			"com_id":  161,
			"prd_pos": "ABCD",
		})
		if err != nil {
			fmt.Println(err.Error())
		}
		return tx.Table("cloud_mark").Insert(sqlm.KeyValue{
			"com_id":    161,
			"prd_pos":   "ABC",
			"prd_pos_t": "ABC",
		})
	})
	fmt.Println(code, err)
}

type bLog struct {
	Prefix string
	Level  int
}

// 6
func (l bLog) Debug(s string) {
	if l.Level >= 6 {
		fmt.Printf("[DEBU--]%s%s\n", l.Prefix, s)
	}
}

// 5
func (l bLog) Info(s string) {
	if l.Level >= 5 {
		fmt.Printf("[INFO--]%s%s\n", l.Prefix, s)
	}
}

// 4
func (l bLog) Warn(s string) {
	if l.Level >= 4 {
		fmt.Printf("[WARN--]%s%s\n", l.Prefix, s)
	}
}

// 3
func (l bLog) Error(s string) {
	if l.Level >= 3 {
		fmt.Printf("[ERRO--]%s%s\n", l.Prefix, s)
	}
}

// 2
func (l bLog) Panic(s string) {
	if l.Level >= 2 {
		fmt.Printf("[PANI--]%s%s\n", l.Prefix, s)
	}
}

// 1
func (l bLog) Fatal(s string) {
	if l.Level >= 1 {
		fmt.Printf("[FATA--]%s%s\n", l.Prefix, s)
	}
}
