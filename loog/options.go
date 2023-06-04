package loog

import "go.uber.org/zap/zapcore"

type Options struct {
	FilePath    string `yaml:"filePath"`    // 日志文件路径
	Level       int8   `yaml:"level"`       // 日志级别
	MaxSize     int    `yaml:"maxSize"`     // 每个日志文件保存的最大尺寸 单位：M
	MaxBackups  int    `yaml:"maxBackups"`  // 日志文件最多保存多少个备份
	MaxAge      int    `yaml:"maxAge"`      // 文件最多保存多少天
	Compress    bool   `yaml:"compress"`    // 是否压缩
	ServiceName string `yaml:"serviceName"` // 服务名
}

func NewOptions() *Options {

	return &Options{
		FilePath:    "./main.log",
		Level:       int8(zapcore.InfoLevel),
		MaxSize:     128,
		MaxBackups:  30,
		MaxAge:      7,
		Compress:    true,
		ServiceName: "Main",
	}
}
