package controllers

import (
	"path/filepath"
	"testing"

	"neoskin/internal/i18n"
)

// TestValidationMessageLocalized 验证 422 字段消息来自 resources/lang 的
// validation.yml，并按 validation.attributes 替换 :attribute（Laravel 语义）。
func TestValidationMessageLocalized(t *testing.T) {
	i18n.Root = filepath.Join("..", "..", "..", "resources", "lang")
	i18n.ResetCache()
	cases := []struct {
		name   string
		locale string
		rule   string
		field  string
		params []string
		want   string
	}{
		{
			name: "zh required with attribute", locale: "zh_CN",
			rule: "required", field: "identification",
			want: "邮箱或角色名 不能为空。",
		},
		{
			name: "en required with attribute", locale: "en",
			rule: "required", field: "identification",
			want: "The email or player name field is required.",
		},
		{
			name: "zh email", locale: "zh_CN",
			rule: "email", field: "email",
			want: "邮箱 不是一个合法的邮箱。",
		},
		{
			name: "en unique", locale: "en",
			rule: "unique", field: "email",
			want: "The email has already been taken.",
		},
		{
			name: "zh min string", locale: "zh_CN",
			rule: "min.string", field: "password", params: []string{"min", "8"},
			want: "密码 至少为 8 个字符。",
		},
		{
			name: "en max string", locale: "en",
			rule: "max.string", field: "title", params: []string{"max", "20"},
			want: "The title may not be greater than 20 characters.",
		},
		{
			name: "zh size string falls back to raw field", locale: "zh_CN",
			rule: "size.string", field: "id", params: []string{"size", "32"},
			want: "id 必须是 32 个字符。",
		},
		{
			name: "zh mimes", locale: "zh_CN",
			rule: "mimes", field: "file", params: []string{"values", "png"},
			want: "file 必须是一个 png 类型的文件。",
		},
		{
			name: "zh captcha", locale: "zh_CN",
			rule: "captcha", field: "captcha",
			want: "验证码不正确。",
		},
		{
			name: "en confirmed", locale: "en",
			rule: "confirmed", field: "password",
			want: "The password confirmation does not match.",
		},
		{
			name: "zh in falls back to raw field", locale: "zh_CN",
			rule: "in", field: "type",
			want: "已选的属性 type 非法。",
		},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			got := validationMessage(i18n.New(item.locale, nil), item.rule, item.field, item.params...)
			if got != item.want {
				t.Fatalf("message=%q want=%q", got, item.want)
			}
		})
	}
}

func TestPlayerNameRuleMatching(t *testing.T) {
	cases := []struct {
		name    string
		rule    string
		custom  string
		player  string
		allowed bool
	}{
		{name: "official ok", rule: "official", player: "Steve_01", allowed: true},
		{name: "official rejects cjk", rule: "official", player: "玩家", allowed: false},
		{name: "cjk ok", rule: "cjk", player: "玩家_01", allowed: true},
		{name: "cjk rejects space", rule: "cjk", player: "玩 家", allowed: false},
		{name: "utf8 ok", rule: "utf8", player: "玩家一", allowed: true},
		{name: "utf8 rejects space", rule: "utf8", player: "玩家 一", allowed: false},
		{name: "custom delimited with flag", rule: "custom", custom: "/^[a-z]+$/i", player: "Steve", allowed: true},
		{name: "custom delimited rejects digit", rule: "custom", custom: "/^[a-z]+$/i", player: "S1", allowed: false},
		{name: "custom bare pattern", rule: "custom", custom: "^[a-z]+$", player: "steve", allowed: true},
		{name: "custom empty allows all", rule: "custom", player: "any name!", allowed: true},
		{name: "unknown rule allows all", rule: "unknown", player: "any name!", allowed: true},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if got := playerNameMatches(item.rule, item.custom, item.player); got != item.allowed {
				t.Fatalf("allowed=%v want=%v", got, item.allowed)
			}
		})
	}
}
