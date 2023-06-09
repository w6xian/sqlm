package sqlm

import (
	"github.com/w6xian/sqlm/loog"
)

func NewOptions() *Options {
	opts := &Options{}
	opt := Server{}
	opts.Server = opt
	return opts
}

func NewOptionsWithServer(svr Server) *Options {
	opts := &Options{}
	opts.Server = svr
	return opts
}

type Server struct {
	Protocol     string `toml:"protocol"`
	Database     string `toml:"database"`
	Host         string `toml:"host"`
	Port         int    `toml:"port"`
	Username     string `toml:"user"`
	Password     string `toml:"password"`
	Charset      string `toml:"charset"`
	Pretable     string `toml:"pretable"`
	Maxconnetion int    `toml:"maxconnection"`
}

type Options struct {
	Server   Server   `json:"server"`
	Slavers  []Server `json:"slavers"`
	Logger   loog.Logger
	LogLevel loog.LogLevel
	//
	Conn DbConn
}

func (c *Options) AddSlave(svr Server) {
	c.Slavers = append(c.Slavers, svr)
}
