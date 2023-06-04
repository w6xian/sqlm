package sqlm

import (
	"sync/atomic"

	"github.com/w6xian/sqlm/loog"
)

var opts atomic.Value

func NewOptions() *Options {
	opts := &Options{}
	opt := Server{
		Database:     "",
		Host:         "",
		Port:         0,
		Maxconnetion: 10,
		Protocol:     "mysql",
		Username:     "root",
		Password:     "root",
		Pretable:     "",
		Charset:      "utf8mb4",
	}
	opts.Server = opt
	return opts
}

func NewOptionsWithServer(svr Server) *Options {
	opts := &Options{}
	opts.Server = svr
	return opts
}

type Server struct {
	Protocol     string `yaml:"protocol"`
	Database     string `yaml:"database"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Username     string `yaml:"user"`
	Password     string `yaml:"password"`
	Charset      string `yaml:"charset"`
	Pretable     string `yaml:"pretable"`
	Maxconnetion int    `yaml:"maxconnection"`
}

type Options struct {
	Server   Server
	Slaves   []Server
	Logger   loog.Logger
	LogLevel loog.LogLevel
	//
	Conn DbConn
}

func (c *Options) AddSlave(svr Server) {
	c.Slaves = append(c.Slaves, svr)
}
