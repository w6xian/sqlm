package utils

import (
	"strings"
)

func Orz3Decode(token string) string {
	if strings.HasPrefix(token, "orz:L/") {
		s := strings.Split(token, "/")
		if len(s) == 3 {
			session := strings.TrimRight(s[1], "G")
			// ip, _ := strconv.ParseInt(strings.Split(s[2], ";")[0], 10, 64)
			// ipv4 := util.InetNtoA(ip)
			return session
		}
	}
	return ""
}
