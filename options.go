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

	if profile.Mode != "demo" && profile.Mode != "dev" && profile.Mode != "prod" {
		profile.Mode = "demo"
	}

	if profile.Mode == "prod" && profile.Data == "" {
		if runtime.GOOS == "windows" {
			profile.Data = filepath.Join(os.Getenv("ProgramData"), "memos")

			if _, err := os.Stat(profile.Data); os.IsNotExist(err) {
				if err := os.MkdirAll(profile.Data, 0770); err != nil {
					fmt.Printf("Failed to create data directory: %s, err: %+v\n", profile.Data, err)
					return nil, err
				}
			}
		} else {
			profile.Data = "/var/opt/sqlm"
		}
	}

	dataDir, err := utils.CheckDataDir(profile.Data)
	if err != nil {
		fmt.Printf("Failed to check dsn: %s, err: %+v\n", dataDir, err)
		return nil, err
	}

	profile.Data = dataDir
	if profile.Protocol == "sqlite" && profile.DSN == "" {
		dbFile := fmt.Sprintf("sqlm_%s.db", profile.Mode)
		profile.DSN = filepath.Join(dataDir, dbFile)
	}
	profile.Version = GetCurrentVersion(profile.Mode)

	return opts, nil
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
	Mode         string `json:"mode"`
	Data         string `json:"data"`
	Version      string `json:"version"`
}

type Options struct {
	Server  Server   `json:"server"`
	Slavers []Server `json:"slavers"`
	log     StdLog
	//
	Conn DbConn
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
	return p.Server.Mode != "prod"
}
