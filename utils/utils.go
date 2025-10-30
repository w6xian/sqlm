package utils

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
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

func DeepCopy(dst, src any) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(src); err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewBuffer(buf.Bytes())).Decode(dst)
}

func Copy(dst, src any) {
	aj, _ := json.Marshal(src)
	_ = json.Unmarshal(aj, dst)
}

func GetInt64(val any) int64 {
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

func GetString(value any) string {
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

func IsEmpty(value any) bool {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.String:
		return v.Len() == 0
	case reflect.Ptr, reflect.Slice, reflect.Map:
		return v.IsNil()
	case reflect.Struct:
		return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
	default:
		return false
	}
}

func GetOrDefault[T any](v T, def T) T {
	if IsEmpty(v) {
		return def
	}
	return v
}

// sql格式化 根据数据类型替换"?", 字符串类型会自动添加单引号, 数字类型直接替换
// 示例: SqlFilter("select * from table where id = ? and name = ?", 1, "张三")
// 结果: select * from table where id = 1 and name = '张三'
func SqlParse(str string, values ...any) string {
	var result string
	var valueIndex int
	for i := 0; i < len(str); i++ {
		if str[i] == '?' && valueIndex < len(values) {
			// 找到问号占位符，需要替换
			switch v := values[valueIndex].(type) {
			case string:
				// 字符串类型，添加单引号并转义内部的单引号
				escaped := ""
				for _, ch := range v {
					if ch == '\'' {
						escaped += "''"
					} else {
						escaped += string(ch)
					}
				}
				result += "'" + escaped + "'"
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
				// 数字类型，直接转换为字符串
				result += GetString(v)
			case nil:
				// nil值转换为NULL
				result += "NULL"
			default:
				// 其他类型，转换为字符串并添加单引号
				result += "'" + GetString(v) + "'"
			}
			valueIndex++
		} else {
			// 复制原字符
			result += string(str[i])
		}
	}

	return result
}
