package utils

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func RandBytes(size int) []byte {
	buf := make([]byte, size)
	for i := 0; i < size; i++ {
		buf[i] = byte(rand.Int31n(128))
	}
	return buf
}

func DeepCopy(dst, src interface{}) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(src); err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewBuffer(buf.Bytes())).Decode(dst)
}

func Copy(dst, src interface{}) {
	aj, _ := json.Marshal(src)
	_ = json.Unmarshal(aj, dst)
}

func GetInt64(val interface{}) int64 {
	switch value := val.(type) {
	case string:
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0
		}
		return val
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	default:
		return 0
	}
	return 0
}

func GetInt(value string) int {
	val, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return val
}

func ParseInt64(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

func GetFloat64(value string) float64 {
	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return val
}

func GetString(value interface{}) string {
	var key string
	if value == nil {
		return key
	}
	switch ft := value.(type) {
	case float64:
		key = strconv.FormatFloat(ft, 'f', -1, 64)
	case float32:
		key = strconv.FormatFloat(float64(ft), 'f', -1, 64)
	case int:
		key = strconv.Itoa(ft)
	case uint:
		key = strconv.Itoa(int(ft))
	case int8:
		key = strconv.Itoa(int(ft))
	case uint8:
		key = strconv.Itoa(int(ft))
	case int16:
		key = strconv.Itoa(int(ft))
	case uint16:
		key = strconv.Itoa(int(ft))
	case int32:
		key = strconv.Itoa(int(ft))
	case uint32:
		key = strconv.Itoa(int(ft))
	case int64:
		key = strconv.FormatInt(ft, 10)
	case uint64:
		key = strconv.FormatUint(ft, 10)
	case string:
		key = value.(string)
	case []byte:
		key = string(value.([]byte))
	default:
		newValue, _ := json.Marshal(value)
		key = string(newValue)
	}
	return key
}

func BuildSqlQ(num int) []string {
	rst := []string{}
	for i := 0; i < num; i++ {
		rst = append(rst, "?")
	}
	return rst
}

func CheckDataDir(dataDir string) (string, error) {
	// Convert to absolute path if relative path is supplied.
	if !filepath.IsAbs(dataDir) {
		relativeDir := filepath.Join(filepath.Dir(os.Args[0]), dataDir)
		absDir, err := filepath.Abs(relativeDir)
		if err != nil {
			return "", err
		}
		dataDir = absDir
	}

	// Trim trailing \ or / in case user supplies
	dataDir = strings.TrimRight(dataDir, "\\/")

	if _, err := os.Stat(dataDir); err != nil {
		return "", errors.Wrapf(err, "unable to access data folder %s", dataDir)
	}

	return dataDir, nil
}
