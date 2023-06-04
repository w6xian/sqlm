package sqlm

import "github.com/w6xian/sqlm/loog"

func (d *Db) logf(level loog.LogLevel, f string, args ...interface{}) {
	if d.Logger != nil {
		loog.Logf(d.Logger, d.LogLevel, level, f, args...)
	}
}
