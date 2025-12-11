package main

import (
	"context"
	"fmt"
	"time"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

type Casher struct {
	Id           int64  `json:"id"`
	ProxyID      int64  `json:"proxy_id"`
	ShopId       int64  `json:"shop_id"`
	UserId       int64  `json:"user_id"`
	EmployeeId   int64  `json:"emp_id"`
	EmployeeName string `json:"name"`
	Mobile       string `json:"mobile" ignore:"io"`
	Avatar       string `json:"avatar"`
	Leader       int64  `json:"is_leader"`
}
type Products struct {
	Id   int64  `json:"prd_id,omitempty" ignore:"api"`
	Name string `json:"name"`
}

type SyncTable struct {
	Id         int64  `json:"id"`
	Name       string `json:"name"`
	PkCol      string `json:"pk_col"`
	LimitNum   int64  `json:"limit_num"`
	PragmaData int64  `json:"pragma_data"`
	Cols       string `json:"cols"`
	Intime     int64  `json:"intime"`
}

func main() {

	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Database:     "cloud",
		Host:         "127.0.0.1",
		Port:         3306,
		Protocol:     "mysql",
		Username:     "root",
		Password:     "1Qazxsw2",
		Pretable:     "mi_",
		Charset:      "utf8mb4",
		MaxOpenConns: 64,
		MaxIdleConns: 64,
		MaxLifetime:  int(time.Second) * 60,
		DSN:          "sqlm_demo.db", //"cloud?charset=utf8mb4&parseTime=True&loc=Local",
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

	opt1, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol:     "sqlite",
		Pretable:     "mi_",
		Charset:      "utf8mb4",
		MaxOpenConns: 64,
		MaxIdleConns: 64,
		MaxLifetime:  int(time.Second) * 60,
		DSN:          "sqlm_demo.db", //"cloud?charset=utf8mb4&parseTime=True&loc=Local",
	}, "sqlite")
	if err != nil {
		fmt.Println(err.Error())
	}
	con1, err := store.NewDriver(opt1)
	if err != nil {
		fmt.Println("not conne", err.Error())
	}

	sqlm.Use(con, con1)

	db := sqlm.NewInstance(context.Background(), "def")
	defer db.Close()
	db1 := sqlm.NewInstance(context.Background(), "sqlite")
	defer db1.Close()

	syncTable := `
	CREATE TABLE [mi_mall_temp] (
		[id] INTEGER AUTO_INCREMENT NULL,
		[proxy_id] INTEGER NOT NULL,
		[ticket] VARCHAR(45) NOT NULL,
		[token] VARCHAR(250) NOT NULL,
		[intime] INT NOT NULL,
		 PRIMARY KEY ([id])
	  );
	  CREATE INDEX [idx_ticket]
	  ON [mi_mall_temp] (
		[ticket] ASC
	  );
	  CREATE INDEX [idx_proxy]
	  ON [mi_mall_temp] (
		[proxy_id] ASC
	  );
	  CREATE TABLE [mi_mall_temp_items] (
		[id] INTEGER AUTO_INCREMENT NULL,
		[temp_id] INTEGER NOT NULL,
		[name] VARCHAR(64) NOT NULL,
		[sn] VARCHAR(64) NOT NULL,
		[num]INTEGER(64) NOT NULL,
		[price]INTEGER(64) NOT NULL,
		[total]INTEGER(64) NOT NULL,
		[discount]INTEGER(64) NOT NULL,
		[payed]INTEGER(64) NOT NULL,
		[off]INTEGER(64) NOT NULL,
		[abatement]INTEGER(64) NOT NULL,
		[debit]INTEGER(64) NOT NULL,
		[intime] INT NOT NULL,
		 PRIMARY KEY ([id])
	  );
	  CREATE INDEX [idx_tmp_id]
	  ON [mi_mall_temp_items] (
		[temp_id] ASC
	  );
	`

	// 不存在就创建
	if _, err := db1.Query(`SELECT * FROM sqlite_master  WHERE type='table' and name='mi_mall_temp'`); err != nil {
		if _, err = db1.Exec(syncTable); err != nil {
			return
		}
	}

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
	fmt.Println("----222")
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
