package laravel

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

var ErrNoKey = errors.New("NEOSKIN_APP_KEY is not configured")

// RawKey 返回 .env 中 APP_KEY 的原始字符串。
// Laravel 的签名 URL（hash_hmac）直接使用该字符串（含 "base64:" 前缀）。
func RawKey() (string, error) {
	raw := strings.TrimSpace(os.Getenv("NEOSKIN_APP_KEY"))
	if raw == "" {
		return "", ErrNoKey
	}
	return raw, nil
}

// Key 返回 PHP 端 APP_KEY 的原始字节。
// 支持 Laravel 的 "base64:" 前缀；普通字符串按 UTF-8 字节使用。
func Key() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("NEOSKIN_APP_KEY"))
	if raw == "" {
		return nil, ErrNoKey
	}
	if strings.HasPrefix(raw, "base64:") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(raw, "base64:"))
		if err != nil {
			return nil, err
		}
		return decoded, nil
	}
	return []byte(raw), nil
}

// SignHS256 生成 Passport laravel_token cookie 所使用的 HS256 JWT。
func SignHS256(claims map[string]any, key []byte) (string, error) {
	header := map[string]any{"typ": "JWT", "alg": "HS256"}
	head, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	input := base64URL(head) + "." + base64URL(body)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(input))
	return input + "." + base64URL(mac.Sum(nil)), nil
}

// VerifyHS256 校验 HS256 JWT，返回 claims；失败返回 nil。
func VerifyHS256(token string, key []byte) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	input := parts[0] + "." + parts[1]
	signature, err := decodeBase64URL(parts[2])
	if err != nil {
		return nil
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(input))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil
	}
	payload, err := decodeBase64URL(parts[1])
	if err != nil {
		return nil
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return nil
	}
	return claims
}

// ValidSignature 对应 Laravel 的 hasValidSignature(false)：使用相对路径校验。
func ValidSignature(path, rawQuery string, now time.Time, ignore ...string) bool {
	raw, err := RawKey()
	if err != nil {
		return false
	}
	key := []byte(raw)
	values := parseQuery(rawQuery)
	signature := values.Get("signature")
	if signature == "" {
		return false
	}
	if expires := values.Get("expires"); expires != "" {
		deadline, err := strconv.ParseInt(expires, 10, 64)
		if err != nil || now.Unix() > deadline {
			return false
		}
	}
	original := strings.TrimRight("/"+strings.TrimPrefix(path, "/")+"?"+
		filteredQuery(rawQuery, append([]string{"signature"}, ignore...)), "?")
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(original))
	return hmac.Equal([]byte(signature), []byte(byteHex(mac.Sum(nil))))
}

func filteredQuery(rawQuery string, ignore []string) string {
	skip := map[string]bool{}
	for _, name := range ignore {
		skip[name] = true
	}
	var kept []string
	for _, part := range strings.Split(rawQuery, "&") {
		if part == "" {
			continue
		}
		name := part
		if index := strings.IndexByte(part, '='); index >= 0 {
			name = part[:index]
		}
		if skip[name] {
			continue
		}
		kept = append(kept, part)
	}
	return strings.Join(kept, "&")
}

type queryValues map[string][]string

func parseQuery(rawQuery string) queryValues {
	values := queryValues{}
	for _, part := range strings.Split(rawQuery, "&") {
		if part == "" {
			continue
		}
		name, value := part, ""
		if index := strings.IndexByte(part, '='); index >= 0 {
			name, value = part[:index], part[index+1:]
		}
		values[name] = append(values[name], value)
	}
	return values
}

func (v queryValues) Get(name string) string {
	if list := v[name]; len(list) > 0 {
		return list[0]
	}
	return ""
}

func base64URL(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func decodeBase64URL(value string) ([]byte, error) {
	if padding := len(value) % 4; padding > 0 {
		value += strings.Repeat("=", 4-padding)
	}
	return base64.URLEncoding.DecodeString(value)
}

func byteHex(data []byte) string {
	const alphabet = "0123456789abcdef"
	out := make([]byte, len(data)*2)
	for i, b := range data {
		out[i*2] = alphabet[b>>4]
		out[i*2+1] = alphabet[b&0x0f]
	}
	return string(out)
}
