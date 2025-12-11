package sqlm

type ServerOption func(o *Server)

/*
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

*/

func DSN(dsn string) ServerOption {
	return func(o *Server) {
		o.DSN = dsn
	}
}
func MaxOpenConns(max int) ServerOption {
	return func(o *Server) {
		o.MaxOpenConns = max
	}
}
func MaxIdleConns(max int) ServerOption {
	return func(o *Server) {
		o.MaxIdleConns = max
	}
}
func MaxLifetime(max int) ServerOption {
	return func(o *Server) {
		o.MaxLifetime = max
	}
}

func Protocol(protocol string) ServerOption {
	return func(o *Server) {
		o.Protocol = protocol
	}
}
func Database(db string) ServerOption {
	return func(o *Server) {
		o.Database = db
	}
}

func Host(host string) ServerOption {
	return func(o *Server) {
		o.Host = host
	}
}
func Port(port int) ServerOption {
	return func(o *Server) {
		o.Port = port
	}
}
func Username(user string) ServerOption {
	return func(o *Server) {
		o.Username = user
	}
}
func Password(pwd string) ServerOption {
	return func(o *Server) {
		o.Password = pwd
	}
}

func Charset(charset string) ServerOption {
	return func(o *Server) {
		o.Charset = charset
	}
}

func Pretable(pretable string) ServerOption {
	return func(o *Server) {
		o.Pretable = pretable
	}
}
