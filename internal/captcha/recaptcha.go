package captcha

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// verifyURL 对应 PHP Rules\Captcha 使用的 reCAPTCHA 站点验证接口；
// 测试中可替换为本地 httptest 服务器。
var verifyURL = "https://www.recaptcha.net/recaptcha/api/siteverify"

// verifyClient 用于访问隐私校验接口（PHP 仅设置超时相关默认值，此处 10s）。
var verifyClient = &http.Client{Timeout: 10 * time.Second}

type recaptchaResponse struct {
	Success bool `json:"success"`
}

// VerifyRecaptcha 校验 reCAPTCHA v2 token（PHP Rules\Captcha：配置了
// recaptcha_secretkey 时走线上校验，否则走本地 session 验证码）。
// response 为表单 captcha 字段的值（前端由 g-recaptcha-response 回填）。
func VerifyRecaptcha(secret, response string) bool {
	if secret == "" || strings.TrimSpace(response) == "" {
		return false
	}
	form := url.Values{"secret": {secret}, "response": {strings.TrimSpace(response)}}
	request, err := http.NewRequest(http.MethodPost, verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return false
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := verifyClient.Do(request)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var parsed recaptchaResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return false
	}
	return parsed.Success
}
