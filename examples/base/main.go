package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

type City struct {
	Id   int
	Name string
}

type Casher struct {
	Id           int64  `json:"id"`
	ProxyID      int64  `json:"proxy_id"`
	ShopId       int64  `json:"shop_id"`
	UserId       int64  `json:"user_id"`
	EmployeeId   int64  `json:"emp_id"`
	EmployeeName string `json:"name"`
	Mobile       string `json:"mobile"`
	Avatar       string `json:"avatar"`
	Leader       int64  `json:"is_leader"`
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

	type SyncTable struct {
		Id         int64  `json:"id"`
		Name       string `json:"name"`
		PkCol      string `json:"pk_col"`
		LimitNum   int64  `json:"limit_num"`
		PragmaData int64  `json:"pragma_data"`
		Cols       string `json:"cols"`
		Intime     int64  `json:"intime"`
	}

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
	fmt.Println(con, con1)

	// sqlm.Use(con, con1)

	db := sqlm.NewInstance(context.Background(), "def")
	defer db.Close()
	db1 := sqlm.NewInstance(context.Background(), "sqlite")
	defer db1.Close()
	// syncTable := `
	// CREATE TABLE [mi_sync_tables] (
	// 	[id] INTEGER AUTO_INCREMENT NULL,
	// 	[name] VARCHAR(250) NOT NULL,
	// 	[pk_col] VARCHAR(250) NOT NULL,
	// 	[limit_num] INT NOT NULL,
	// 	[cols] VARCHAR(250) NOT NULL,
	// 	[pragma_data] VARCHAR(250) NOT NULL,
	// 	[intime] INT NOT NULL,
	// 	 PRIMARY KEY ([id])
	//   );
	//   CREATE INDEX [idx_name]
	//   ON [mi_sync_tables] (
	// 	[name] ASC
	//   );
	// `

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

	// if _, err := db1.Query(`SELECT * FROM sqlite_master  WHERE type='table' and name='mi_sync_tables'`); err != nil {
	// 	if _, err = db1.Exec(syncTable); err != nil {
	// 		return
	// 	}
	// }
	c := &Casher{}
	cs, err := db.Table("com_shops_cashers").Select("*").
		Where("shop_id=%d", 2).
		And("mobile='%s'", "").
		Query()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		cs.Scan(c)
		fmt.Println(c.Id, c.EmployeeId, c.EmployeeName)
	}
	return
	v := &SyncTable{}
	row, err := db.Table("sync_tables").Select("*").Query()
	row.Scan(v)
	fmt.Println(v.Id, v.Name, v.Cols, v.Intime, v.LimitNum)
	return
	// 同步同步表
	maxId := db1.MaxId(db1.TableName("sync_tables"))
	cols := strings.Split("id,name,pk_col,limit_num,cols,intime", ",")
	fmt.Println(cols)
	for _, v := range cols {
		fmt.Println(v)
	}
	fmt.Println(maxId)

	mRow, err := db.QueryMulti("SELECT id,parent_track,user_name FROM mi_mall_so WHERE id=? limit 10", 1)
	if err == nil {
		fmt.Println(mRow.Next().Get("user_name").String())
	}

	sRow, err := db.Query("SELECT id,parent_track,user_name FROM mi_mall_so WHERE id = ?", 1)
	if err == nil {
		fmt.Println("id=1")
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

	row, err = db.Table("mall_so").Where("id=%d", 1).Query()

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

		fmt.Println(rows.ToKeyValueMap("id", "com_name"))
		fmt.Println(rows.ToKeyMap("id"))
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
