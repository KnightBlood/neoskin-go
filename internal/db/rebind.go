package db

import (
	"fmt"
	"strings"
	"time"
)

// Rebind 把 `?` 占位符转换成当前方言的占位符。
// PostgreSQL: $1..$n；SQL Server: @p1..@pN；MySQL/SQLite 保持 `?`。
func Rebind(dialect *Dialect, query string) string {
	if dialect.Name != "pgsql" && dialect.Name != "sqlsrv" {
		return query
	}

	var builder strings.Builder
	builder.Grow(len(query) + 8)
	index := 0
	inSingle := false
	inDouble := false
	for i := 0; i < len(query); i++ {
		char := query[i]
		switch char {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '?':
			if !inSingle && !inDouble {
				index++
				if dialect.Name == "pgsql" {
					builder.WriteString(fmt.Sprintf("$%d", index))
				} else {
					builder.WriteString(fmt.Sprintf("@p%d", index))
				}
				continue
			}
		}
		builder.WriteByte(char)
	}

	return builder.String()
}

// TimeString 把驱动返回的时间值统一成 PHP 的 Y-m-d H:i:s。
func TimeString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case time.Time:
		return typed.Format("2006-01-02 15:04:05")
	case []byte:
		return string(typed)
	case string:
		return typed
	}

	return fmt.Sprint(value)
}
