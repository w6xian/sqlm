package sqlm

import (
	"strconv"
	"strings"
)

type Svr struct {
	Host string
	Port int
}

func NewSvr(svr string) *Svr {
	sp := strings.Split(svr, ":")
	s := &Svr{}

	s.Host = strings.TrimSpace(sp[0])
	if len(sp) > 1 {
		pt, err := strconv.Atoi(sp[1])
		if err != nil {
			pt = 0
		}
		s.Port = pt
	}
	return s
}
