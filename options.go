package sqlm

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/w6xian/sqlm/utils"
)

type Option func(o *Options)

func WithName(name string) Option {
	return func(o *Options) {
		o.Name = name
	}
}

func WithLogger(logger StdLog) Option {
	return func(o *Options) {
		o.log = logger
	}
}

func WithMysqlServer(opts ...ServerOption) Option {
	return func(o *Options) {
		o.Server.Protocol = "mysql"
		o.Server.Host = "127.0.0.1"
		o.Server.Port = 3306
		o.Server.Username = "root"
		o.Server.Password = ""
		o.Server.Charset = "utf8mb4"
		o.Server.Pretable = "mi_"
		o.Server.MaxOpenConns = 100
		o.Server.MaxIdleConns = 10
		o.Server.MaxLifetime = int(time.Minute)
		for _, opt := range opts {
			opt(o.Server)
		}
	}
}

func NewOptions(opts ...ServerOption) *Options {
	options := &Options{}
	options.Name = DEFAULT_KEY
	options.Server = newDefaultMysqlServer()
	options.log = &baseLog{Level: 9}
	for _, o := range opts {
		o(options.Server)
	}
	return options
}
func NewDefaultOptions(opts ...ServerOption) *Options {
	options := &Options{}
	options.Name = DEFAULT_KEY
	options.Server = newDefaultMysqlServer()
	options.log = &baseLog{Level: 9}
	for _, o := range opts {
		o(options.Server)
	}
	return options
}

func NewOptionsWithServer(profile Server, args ...string) (*Options, error) {
	if len(args) == 0 {
		args = []string{DEFAULT_KEY}
	}
	opts := &Options{}
	opts.Name = args[0]
	opts.Server = &profile
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
	DSN          string `json:"dsn"`
	MaxOpenConns int    `json:"max_open_conns"`
	MaxIdleConns int    `json:"max_idel_conns"`
	MaxLifetime  int    `json:"max_life_time"`
}

type Options struct {
	Name    string    `json:"name"`
	Server  *Server   `json:"server"`
	Slavers []*Server `json:"slavers"`
	log     StdLog
	//
	Conn DbConn
	Mode string `json:"mode"`
	Data string `json:"data"`
}

func (c *Options) AddSlave(svr *Server) {
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

func NewServer() *Server {
	return &Server{}
}

func newDefaultMysqlServer(opts ...ServerOption) *Server {
	s := &Server{
		Database:     "cloud",
		Host:         "127.0.0.1",
		Port:         3306,
		Protocol:     "mysql",
		Username:     "root",
		Password:     "",
		Pretable:     "mi_",
		Charset:      "utf8mb4",
		MaxOpenConns: 64,
		MaxIdleConns: 64,
		MaxLifetime:  int(time.Second) * 60,
		DSN:          "sqlm_demo.db", //"cloud?charset=utf8mb4&parseTime=True&loc=Local",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
