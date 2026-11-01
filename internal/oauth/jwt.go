package oauth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
)

func base64URL(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func decodeBase64URL(value string) ([]byte, error) {
	if padding := len(value) % 4; padding > 0 {
		value += strings.Repeat("=", 4-padding)
	}
	return base64.URLEncoding.DecodeString(value)
}

// SignRS256 生成与 league/oauth2-server 相同结构的最小 JWT。
func SignRS256(claims map[string]any, keys *Keys) (string, error) {
	header := map[string]any{"typ": "JWT", "alg": "RS256"}
	head, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	input := base64URL(head) + "." + base64URL(body)
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, keys.Private, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return input + "." + base64URL(signature), nil
}

// VerifyRS256 校验并返回 claims；失败返回 nil。
func VerifyRS256(token string, keys *Keys) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	input := parts[0] + "." + parts[1]
	signature, err := decodeBase64URL(parts[2])
	if err != nil {
		return nil
	}
	digest := sha256.Sum256([]byte(input))
	if rsa.VerifyPKCS1v15(keys.Public, crypto.SHA256, digest[:], signature) != nil {
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

func RandomToken(length int) string {
	buffer := make([]byte, (length+1)/2)
	_, _ = rand.Read(buffer)
	return base64URL(buffer)[:length]
}
