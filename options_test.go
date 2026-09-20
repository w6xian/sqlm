package sqlm_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm"
)

func TestNewOptionsDefaults(t *testing.T) {
	opt := sqlm.NewOptions()
	require.NotNil(t, opt)
	assert.Equal(t, sqlm.DEFAULT_KEY, opt.Name)
	assert.Equal(t, "mysql", opt.Server.Protocol)
	assert.Equal(t, "mi_", opt.Server.Pretable)
	assert.True(t, opt.IsDev())
	assert.NotNil(t, opt.Logger())

	opt = sqlm.NewDefaultOptions(sqlm.MaxIdleConns(3), sqlm.MaxLifetime(int(time.Second)))
	assert.Equal(t, 3, opt.Server.MaxIdleConns)
	assert.Equal(t, int(time.Second), opt.Server.MaxLifetime)
}

func TestNewOptionsWithServerAndName(t *testing.T) {
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{Protocol: "sqlite", DSN: "x.db"}, "named")
	require.NoError(t, err)
	assert.Equal(t, "named", opt.Name)
	assert.Equal(t, "x.db", opt.Server.DSN)

	def, err := sqlm.NewOptionsWithServer(sqlm.Server{Protocol: "sqlite"})
	require.NoError(t, err)
	assert.Equal(t, sqlm.DEFAULT_KEY, def.Name)
}

func TestCheckOptionNormalizesMode(t *testing.T) {
	opt := sqlm.NewOptions()
	opt.Mode = "whatever"
	checked, err := sqlm.CheckOption(opt)
	require.NoError(t, err)
	assert.Equal(t, "demo", checked.Mode)
	assert.NotEmpty(t, checked.Data)

	dev := sqlm.NewOptions()
	dev.Mode = "dev"
	checked, err = sqlm.CheckOption(dev)
	require.NoError(t, err)
	assert.Equal(t, "dev", checked.Mode)
}

func TestCheckOptionSQLiteDefaultDSN(t *testing.T) {
	opt := sqlm.NewOptions(sqlm.Protocol("sqlite"), sqlm.DSN(""))
	opt.Mode = "dev"
	opt.Data = t.TempDir()
	checked, err := sqlm.CheckOption(opt)
	require.NoError(t, err)
	assert.Contains(t, checked.Server.DSN, "sqlm_dev.db")
}

func TestOptionsLogger(t *testing.T) {
	opt := sqlm.NewOptions()
	lg := sqlm.NewMemLogger(sqlm.DEBUG)
	require.Same(t, opt, opt.SetLogger(lg))
	assert.Same(t, lg, opt.GetLogger())
	assert.Same(t, lg, opt.Logger())

	// 没有配置logger时也不能返回nil
	bare := &sqlm.Options{}
	assert.NotNil(t, bare.Logger())
	var nilOpts *sqlm.Options
	assert.NotNil(t, nilOpts.Logger())
}

func TestOptionHelpers(t *testing.T) {
	opt := &sqlm.Options{Server: sqlm.NewServer()}
	sqlm.WithName("custom")(opt)
	assert.Equal(t, "custom", opt.Name)

	sqlm.WithMysqlServer(sqlm.Database("cloud"), sqlm.Username("root"), sqlm.Password("pwd"),
		sqlm.Host("db.host"), sqlm.Port(3307), sqlm.Charset("utf8"), sqlm.Pretable("mi_"),
		sqlm.MaxOpenConns(20))(opt)
	assert.Equal(t, "cloud", opt.Server.Database)
	assert.Equal(t, "root", opt.Server.Username)
	assert.Equal(t, "pwd", opt.Server.Password)
	assert.Equal(t, "db.host", opt.Server.Host)
	assert.Equal(t, 3307, opt.Server.Port)
	assert.Equal(t, "utf8", opt.Server.Charset)
	assert.Equal(t, "mi_", opt.Server.Pretable)
	assert.Equal(t, 20, opt.Server.MaxOpenConns)

	// Server为nil时也不能panic
	empty := &sqlm.Options{}
	sqlm.WithMysqlServer()(empty)
	require.NotNil(t, empty.Server)
	assert.Equal(t, "mysql", empty.Server.Protocol)

	opt.AddSlave(&sqlm.Server{Protocol: "mysql"})
	assert.Len(t, opt.Slavers, 1)
}

func TestIgnoreClearsTaggedFields(t *testing.T) {
	type item struct {
		Id     int64  `json:"id"`
		Name   string `json:"name"`
		Secret string `json:"secret" ignore:"api"`
	}

	one := &item{Id: 1, Name: "a", Secret: "s"}
	sqlm.Ignore(one)
	assert.Equal(t, int64(1), one.Id)
	assert.Empty(t, one.Secret)

	list := []item{{Id: 2, Secret: "x"}, {Id: 3, Secret: "y"}}
	sqlm.Ignore(&list)
	assert.Equal(t, int64(2), list[0].Id)
	assert.Empty(t, list[0].Secret)
	assert.Empty(t, list[1].Secret)

	// 没有ignore标记的结构体不受影响
	assert.NotPanics(t, func() { sqlm.Ignore(struct{ A int }{A: 1}) })
}

func TestLogLevelString(t *testing.T) {
	assert.Equal(t, "FATAL", sqlm.FATAL.String())
	assert.Equal(t, "ERROR", sqlm.ERROR.String())
	assert.Equal(t, "WARN", sqlm.WARN.String())
	assert.Equal(t, "INFO", sqlm.INFO.String())
	assert.Equal(t, "DEBUG", sqlm.DEBUG.String())
	assert.Equal(t, "TRACE", sqlm.TRACE.String())
	assert.Equal(t, "UNKNOWN", sqlm.LogLevel(99).String())
}

func TestBaseLoggerWritesWhenEnabled(t *testing.T) {
	buf := &bytes.Buffer{}
	lg := sqlm.NewBaseLogger(sqlm.DEBUG, "prefix:", buf)
	lg.Debug("hello")
	assert.Contains(t, buf.String(), "[DEBUG]")
	assert.Contains(t, buf.String(), "prefix:")
	assert.Contains(t, buf.String(), "hello")

	buf.Reset()
	lg.Info("info")
	lg.Warn("warn")
	lg.Error("error")
	assert.Contains(t, buf.String(), "[INFO]")
	assert.Contains(t, buf.String(), "[WARN]")
	assert.Contains(t, buf.String(), "[ERROR]")

	// Fatal 级别以下全部被过滤
	buf.Reset()
	lg.SetLevel(sqlm.FATAL)
	lg.Debug("nope")
	lg.Info("nope")
	assert.Empty(t, buf.String())

	// 关闭后什么都不写
	buf.Reset()
	lg.SetLevel(sqlm.DEBUG)
	lg.SetEnabled(false)
	lg.Debug("nope")
	assert.Empty(t, buf.String())

	// 派生logger保持writer
	lg.SetEnabled(true)
	derived := lg.WithPrefix("derived:").WithLevel(sqlm.INFO)
	buf.Reset()
	derived.Debug("hidden")
	derived.Info("shown")
	assert.NotContains(t, buf.String(), "hidden")
	assert.Contains(t, buf.String(), "derived:")
	assert.Contains(t, buf.String(), "shown")
}

func TestBaseLoggerPanic(t *testing.T) {
	buf := &bytes.Buffer{}
	lg := sqlm.NewBaseLogger(sqlm.FATAL, "", buf)
	assert.Panics(t, func() { lg.Panic("boom") })
}

func TestBaseLoggerImplementsStdLog(t *testing.T) {
	var lg sqlm.StdLog = sqlm.NewBaseLogger(sqlm.DEBUG, "", &bytes.Buffer{})
	require.NotNil(t, lg)
	assert.Implements(t, (*sqlm.StdLog)(nil), sqlm.NewMemLogger(sqlm.DEBUG))
	assert.Implements(t, (*sqlm.StdLog)(nil), sqlm.NewNoopLogger())
	assert.Implements(t, (*sqlm.StdLog)(nil), sqlm.NewNullLogger())
}

func TestMemLoggerFiltering(t *testing.T) {
	lg := sqlm.NewMemLogger(sqlm.WARN)
	lg.Debug("ignored")
	lg.Info("ignored")
	lg.Warn("kept")
	lg.Error("kept")

	assert.Empty(t, lg.Messages(sqlm.DEBUG))
	assert.Empty(t, lg.Messages(sqlm.INFO))
	assert.Len(t, lg.Messages(sqlm.WARN), 1)
	assert.Len(t, lg.Messages(sqlm.ERROR), 1)
	assert.Equal(t, "kept", lg.Last())

	full := sqlm.NewMemLogger(sqlm.DEBUG)
	full.Panic("panic-recorded")
	assert.Len(t, full.Messages(sqlm.ERROR), 1)
	full.Reset()
	assert.Empty(t, full.Messages(sqlm.ERROR))
	assert.Empty(t, full.Last())
}

func TestNoopLoggerIsSilent(t *testing.T) {
	no := sqlm.NewNoopLogger()
	assert.NotPanics(t, func() {
		no.Debug("d")
		no.Info("i")
		no.Warn("w")
		no.Error("e")
		no.Panic("p")
		no.Fatal("f")
	})
}

func TestLogThroughDb(t *testing.T) {
	db, lg := newSQLite(t, "log_db")
	createUsers(t, db)

	_, err := db.Table("users").Insert(map[string]any{"name": "Logged"})
	require.NoError(t, err)

	// 只有 mysql 使用反引号，其它引擎（含 sqlite）走标准双引号标识符
	assert.True(t, strings.HasPrefix(lg.LastSQL(), `INSERT INTO "mi_users"`), lg.LastSQL())
	assert.Empty(t, lg.Errors())
}

func TestVersion(t *testing.T) {
	assert.NotEmpty(t, sqlm.Version)
}
