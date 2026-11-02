package controllers

import (
	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/captcha"
	"neoskin/internal/service"
)

// captchaRequiredFails 对应 PHP `login_fails > 3` 的阈值。
const captchaRequiredFails = 3

// verifyCaptcha 实现 PHP `Rules\Captcha`：配置了 recaptcha_secretkey 时
// 走 reCAPTCHA 线上校验，否则消费 session 中的本地验证码。
func verifyCaptcha(ctx goravelhttp.Context, options *service.Option, input string) bool {
	if secret := options.Get("recaptcha_secretkey"); secret != "" {
		return captcha.VerifyRecaptcha(secret, input)
	}
	return consumeCaptcha(ctx, input)
}
