// Package i18n 读取 resources/lang/<locale>/*.yml 并提供 trans()。
package i18n

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Locale 是语言菜单项（对应 config/locales.php）。
type Locale struct {
	Code  string
	Name  string
	Short string
}

// Locales 保持与 PHP config/locales.php 一致的顺序与显示名。
var Locales = []Locale{
	{Code: "zh_CN", Name: "中文 (简体)", Short: "zh-CN"},
	{Code: "zh_TW", Name: "中文 (正體)", Short: "zh-TW"},
	{Code: "en", Name: "English", Short: "en"},
	{Code: "es_ES", Name: "Español", Short: "es"},
	{Code: "ru_RU", Name: "Русский язык", Short: "ru"},
	{Code: "de_DE", Name: "Deutsch", Short: "de"},
}

var aliases = map[string]string{
	"zh_HANS_CN": "zh_CN",
	"en_US":      "en",
	"ru":         "ru_RU",
}

// Root 是语言文件根目录。
var Root = filepath.Join("resources", "lang")

// Canonical 把别名转换为真实 locale；未知值返回 zh_CN。
func Canonical(locale string) string {
	if mapped, ok := aliases[locale]; ok {
		return mapped
	}
	for _, item := range Locales {
		if item.Code == locale {
			return locale
		}
	}
	if _, err := os.Stat(filepath.Join(Root, locale)); err == nil {
		return locale
	}
	return "zh_CN"
}

// Name 返回语言显示名。
func Name(locale string) string {
	canonical := Canonical(locale)
	for _, item := range Locales {
		if item.Code == canonical {
			return item.Name
		}
	}
	return canonical
}

var (
	treeCache sync.Map // "locale/group" -> map[string]any
)

func loadTree(locale, group string) map[string]any {
	key := locale + "/" + group
	if cached, ok := treeCache.Load(key); ok {
		return cached.(map[string]any)
	}
	tree := map[string]any{}
	data, err := os.ReadFile(filepath.Join(Root, locale, group+".yml"))
	if err == nil {
		_ = yaml.Unmarshal(data, &tree)
	}
	if len(tree) == 0 && locale != "en" {
		if data, err := os.ReadFile(filepath.Join(Root, "en", group+".yml")); err == nil {
			_ = yaml.Unmarshal(data, &tree)
		}
	}
	treeCache.Store(key, tree)
	return tree
}

// ResetCache 清空 YAML 缓存（测试或语言文件更新后调用）。
func ResetCache() {
	treeCache = sync.Map{}
}

// Translator 是某个 locale 的翻译器；overrides 来自 language_lines 表。
type Translator struct {
	locale    string
	overrides map[string]string
}

func New(locale string, overrides map[string]string) *Translator {
	return &Translator{locale: Canonical(locale), overrides: overrides}
}

func (t *Translator) Locale() string {
	return t.locale
}

// Trans 解析 "group.key.path"，支持 :placeholder 替换。
func (t *Translator) Trans(key string, params ...map[string]string) string {
	group, rest := key, ""
	if index := strings.IndexByte(key, '.'); index >= 0 {
		group, rest = key[:index], key[index+1:]
	}
	if t.overrides != nil {
		if value, ok := t.overrides[group+"."+rest]; ok && value != "" {
			return applyParams(value, params...)
		}
	}
	tree := loadTree(t.locale, group)
	value, ok := lookup(tree, rest)
	if !ok || value == "" {
		if fallback := loadTree("en", group); t.locale != "en" {
			if fallbackValue, found := lookup(fallback, rest); found {
				value = fallbackValue
			}
		}
	}
	return applyParams(value, params...)
}

// Group 返回某个语言文件的完整树（供 front-end.js 使用）。
func (t *Translator) Group(group string) map[string]any {
	return loadTree(t.locale, group)
}

func lookup(tree map[string]any, path string) (string, bool) {
	if path == "" {
		return "", false
	}
	current := any(tree)
	for _, segment := range strings.Split(path, ".") {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return "", false
			}
			current = next
		case map[any]any:
			next, ok := typed[segment]
			if !ok {
				return "", false
			}
			current = next
		default:
			return "", false
		}
	}
	switch typed := current.(type) {
	case string:
		return typed, true
	case int, int64, float64, bool:
		return fmt.Sprint(typed), true
	}
	return "", false
}

func applyParams(value string, params ...map[string]string) string {
	if len(params) == 0 || value == "" {
		return value
	}
	replacements := make([]string, 0, len(params[0])*2)
	keys := make([]string, 0, len(params[0]))
	for key := range params[0] {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		replacements = append(replacements, ":"+key, params[0][key])
	}
	return strings.NewReplacer(replacements...).Replace(value)
}
