package sqlm

import "fmt"

type StdLog interface {
	Debug(s string)
	Info(s string)
	Warn(s string)
	Error(s string)
	Panic(s string)
	Fatal(s string)
}

type baseLog struct {
	prefix string
	Level  int
}

//6
func (l baseLog) Debug(s string) {
	if l.Level >= 6 {
		fmt.Printf("[DEBU]%s%s\n", l.prefix, s)
	}
}

//5
func (l baseLog) Info(s string) {
	if l.Level >= 5 {
		fmt.Printf("[INFO]%s%s\n", l.prefix, s)
	}
}

//4
func (l baseLog) Warn(s string) {
	if l.Level >= 4 {
		fmt.Printf("[WARN]%s%s\n", l.prefix, s)
	}
}

//3
func (l baseLog) Error(s string) {
	if l.Level >= 3 {
		fmt.Printf("[ERRO]%s%s\n", l.prefix, s)
	}
}

//2
func (l baseLog) Panic(s string) {
	if l.Level >= 2 {
		fmt.Printf("[PANI]%s%s\n", l.prefix, s)
	}
}

// 1
func (l baseLog) Fatal(s string) {
	if l.Level >= 1 {
		fmt.Printf("[FATA]%s%s\n", l.prefix, s)
	}
}
