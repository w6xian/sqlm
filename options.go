package sqlm

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/w6xian/sqlm/utils"
)

func NewOptions() *Options {
	opts := &Options{}
	opt := Server{}
	opts.Server = opt
	opts.log = &baseLog{Level: 9}
	return opts
}

func NewOptionsWithServer(profile Server) (*Options, error) {
	opts := &Options{}
	opts.Server = profile
	opts.log = &baseLog{Level: 9}
	return opts, nil
}

func CheckOption(opt *Options) (*Options, error) {
	if opt.Mode != "demo" && opt.Mode != "dev" && opt.Mode != "prod" {
		opt.Mode = "demo"
	}

	if opt.Mode == "prod" && opt.Data == "" {
		if runtime.GOOS == "windows" {
			opt.Data = filepath.Join(os.Getenv("ProgramData"), "memos")

			if _, err := os.Stat(opt.Data); os.IsNotExist(err) {
				if err := os.MkdirAll(opt.Data, 0770); err != nil {
					fmt.Printf("Failed to create data directory: %s, err: %+v\n", opt.Data, err)
					return nil, err
				}
			}
		} else {
			opt.Data = "/var/opt/sqlm"
		}
	}

	dataDir, err := utils.CheckDataDir(opt.Data)
	if err != nil {
		fmt.Printf("Failed to check dsn: %s, err: %+v\n", dataDir, err)
		return nil, err
	}

	opt.Data = dataDir
	if opt.Server.Protocol == "sqlite" && opt.Server.DSN == "" {
		dbFile := fmt.Sprintf("sqlm_%s.db", opt.Mode)
		opt.Server.DSN = filepath.Join(dataDir, dbFile)
	}
	opt.Version = GetCurrentVersion(opt.Mode)
	return opt, nil
}

type Server struct {
	Protocol     string `json:"protocol"`
	Database     string `json:"database"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Charset      string `json:"charset"`
	Pretable     string `json:"pretable"`
	Maxconnetion int    `json:"maxconnection"`
	DSN          string `json:"dsn"`
}

type Options struct {
	Server  Server   `json:"server"`
	Slavers []Server `json:"slavers"`
	log     StdLog
	//
	Conn    DbConn
	Mode    string `json:"mode"`
	Data    string `json:"data"`
	Version string `json:"version"`
}

func (c *Options) AddSlave(svr Server) {
	c.Slavers = append(c.Slavers, svr)
}

func (opts *Options) SetLogger(log StdLog) *Options {
	opts.log = log
	return opts
}

func (opts *Options) GetLogger() StdLog {
	return opts.log
}

func (p *Options) IsDev() bool {
	return p.Mode != "prod"
}
