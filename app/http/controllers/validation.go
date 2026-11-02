package controllers

import (
	"fmt"
	"net/http"
	"sort"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/i18n"
	"neoskin/internal/service"
)

// validationFailed 生成与 Laravel ValidationException 一致的 422 JSON：
//
//	{"message": "<首条错误>[ (and N more errors)]", "errors": {"field": ["msg", ...]}}
//
// message 取首条校验消息；多于一条时追加 "(and :count more errors)"，对应
// Illuminate\Validation\ValidationException::summarize（语言包未提供该文案的
// 翻译，PHP 同样保持英文）。PHP 的首条消息取规则声明顺序，Go 调用方以 map
// 传参，这里按字段名排序保证确定性——仅在多个字段同时失败时可能选中不同的
// 首条消息（每条字段消息本身与 PHP 一致）。
func validationFailed(ctx goravelhttp.Context, errors map[string][]string) goravelhttp.Response {
	fields := make([]string, 0, len(errors))
	total := 0
	for field, messages := range errors {
		if len(messages) == 0 {
			continue
		}
		fields = append(fields, field)
		total += len(messages)
	}
	sort.Strings(fields)

	message := ""
	if len(fields) > 0 {
		message = errors[fields[0]][0]
	}
	if total > 1 {
		plural := "errors"
		if total-1 == 1 {
			plural = "error"
		}
		message += fmt.Sprintf(" (and %d more %s)", total-1, plural)
	}

	return ctx.Response().Json(http.StatusUnprocessableEntity, goravelhttp.Json{
		"message": message,
		"errors":  errors,
	})
}

// vMsg 按 Laravel Validator::makeReplacements 的语义生成校验消息：
// 文案取自 resources/lang/<locale>/validation.yml，`:attribute` 使用
// validation.attributes 中的友好名称（未定义时保留原始字段名），其余占位符
// 由 params 以 key/value 交替传入（如 "min", "8", "max", "16"）。
func vMsg(ctx goravelhttp.Context, rule, field string, params ...string) string {
	return validationMessage(translatorFor(ctx), rule, field, params...)
}

// validationMessage 是 vMsg 的纯函数核心（便于单测）。
func validationMessage(translator *i18n.Translator, rule, field string, params ...string) string {
	attribute := translator.Trans("validation.attributes." + field)
	if attribute == "" {
		attribute = field
	}
	pairs := map[string]string{"attribute": attribute}
	for index := 0; index+1 < len(params); index += 2 {
		pairs[params[index]] = params[index+1]
	}
	return translator.Trans("validation."+rule, pairs)
}

func vRequired(ctx goravelhttp.Context, field string) string {
	return vMsg(ctx, "required", field)
}

func vEmail(ctx goravelhttp.Context, field string) string {
	return vMsg(ctx, "email", field)
}

func vUnique(ctx goravelhttp.Context, field string) string {
	return vMsg(ctx, "unique", field)
}

func vIn(ctx goravelhttp.Context, field string) string {
	return vMsg(ctx, "in", field)
}

func vExists(ctx goravelhttp.Context, field string) string {
	return vMsg(ctx, "exists", field)
}

func vInteger(ctx goravelhttp.Context, field string) string {
	return vMsg(ctx, "integer", field)
}

func vBoolean(ctx goravelhttp.Context, field string) string {
	return vMsg(ctx, "boolean", field)
}

func vURL(ctx goravelhttp.Context, field string) string {
	return vMsg(ctx, "url", field)
}

func vConfirmed(ctx goravelhttp.Context, field string) string {
	return vMsg(ctx, "confirmed", field)
}

func vMinString(ctx goravelhttp.Context, field, min string) string {
	return vMsg(ctx, "min.string", field, "min", min)
}

func vMaxString(ctx goravelhttp.Context, field, max string) string {
	return vMsg(ctx, "max.string", field, "max", max)
}

func vSizeString(ctx goravelhttp.Context, field, size string) string {
	return vMsg(ctx, "size.string", field, "size", size)
}

// vPasswordMessage 复刻 Laravel `required|min:N|max:M` 的字符串规则行为：
// 空值只报 required（required 失败后其余规则不再执行），过短报 min.string，
// 过长报 max.string。
func vPasswordMessage(ctx goravelhttp.Context, field, value string, min, max int) string {
	switch {
	case value == "":
		return vRequired(ctx, field)
	case len([]rune(value)) < min:
		return vMinString(ctx, field, fmt.Sprint(min))
	case len([]rune(value)) > max:
		return vMaxString(ctx, field, fmt.Sprint(max))
	}
	return ""
}

func vMimes(ctx goravelhttp.Context, field, values string) string {
	return vMsg(ctx, "mimes", field, "values", values)
}

// vCaptcha 对应 PHP Rules\Captcha::message：配置了 reCAPTCHA secret 时用
// validation.recaptcha，否则用 validation.captcha。
func vCaptcha(ctx goravelhttp.Context, options *service.Option) string {
	if options != nil && options.Get("recaptcha_secretkey") != "" {
		return vMsg(ctx, "recaptcha", "captcha")
	}
	return vMsg(ctx, "captcha", "captcha")
}
