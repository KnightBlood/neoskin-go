package laravel

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SignedPath 生成 Laravel 相对签名 URL（对应 URL::signedRoute / temporarySignedRoute(..., false)）。
// 返回值形如 /auth/reset/1?expires=...&signature=...
func SignedPath(path string, params map[string]string, expires *time.Time) (string, error) {
	raw, err := RawKey()
	if err != nil {
		return "", err
	}
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	if expires != nil {
		values.Set("expires", strconv.FormatInt(expires.Unix(), 10))
	}
	query := values.Encode()
	original := strings.TrimRight(path+"?"+query, "?")
	mac := hmac.New(sha256.New, []byte(raw))
	mac.Write([]byte(original))
	signature := hex.EncodeToString(mac.Sum(nil))
	if query == "" {
		return path + "?signature=" + signature, nil
	}
	return path + "?" + query + "&signature=" + signature, nil
}
