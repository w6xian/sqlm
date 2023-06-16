package sqlm

func NewOptions() *Options {
	opts := &Options{}
	opt := Server{}
	opts.Server = opt
	opts.log = &baseLog{Level: 9}
	return opts
}

func NewOptionsWithServer(svr Server) *Options {
	opts := &Options{}
	opts.Server = svr
	opts.log = &baseLog{Level: 9}
	return opts
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
