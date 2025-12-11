package main

import (
	"context"
	"fmt"
	"time"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

type Products struct {
	Id   int64  `json:"prd_id,omitempty" ignore:"api"`
	Name string `json:"name"`
}

func main() {
	fmt.Println("options sqlm version:", sqlm.Version)
	// 使用mysql
	con, err := store.NewDefaultDriver(sqlm.Password("1Qazxsw2"))
	if err != nil {
		fmt.Println("not conne", err.Error())
	}
	sqlm.Use(con)

	db := sqlm.NewDefaultInstance(context.Background())
	defer db.Close()

	// p := &Products{}
	ps, err := db.Table("com_products").Select("prd_id,name").
		Where("proxy_id=%d", 2).
		Query()
	if err == nil {
		rst := []Products{}
		ps.ScanMulti(&rst)
		fmt.Println("ScanMulti slice:", rst)
		rst1 := Products{}
		err = ps.ScanMulti(&rst1)
		fmt.Println("ScanMulti struct:", rst1)
	}
	t := time.Now().Unix()
	for i := 0; i < 1; i++ {
		p := []Products{}
		db.Table("com_products").Select("prd_id,name").
			Where("proxy_id=%d", 2).
			Limit(10).
			ScanMulti(&p)
	}
	fmt.Println("ScanMulti slice: time", time.Now().Unix()-t)
	fmt.Println("---------")
	p := Products{}
	db.Table("com_products").Select("prd_id,name").
		Where("proxy_id=%d", 2).
		Limit(10).
		Scan(&p)
	fmt.Println("ScanMulti slice:", p)
	count := struct {
		Total int64 `json:"total"`
	}{}
	fmt.Println("----------------")
	db.Table("com_products").
		Count().
		Where("proxy_id=%d", 2).
		Scan(&count)
	fmt.Println("Scan count:", count.Total)
}

func ita(sqlm.ITable) {
	fmt.Println(1)
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
