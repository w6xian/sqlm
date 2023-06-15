package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func JoinInt64(arr []int64, s string) string {
	str := strings.Trim(fmt.Sprint(arr), "[ ]")
	return strings.Replace(str, " ", s, -1)
}

func Decimal(num float64, f int) float64 {
	fs := fmt.Sprintf("%%.%df", f)
	num, _ = strconv.ParseFloat(fmt.Sprintf(fs, num), 64)
	return num
}
