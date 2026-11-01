package i18n

import (
	"path/filepath"
	"testing"
)

func TestTrans(t *testing.T) {
	Root = filepath.Join("..", "..", "resources", "lang")
	ResetCache()

	translator := New("zh_CN", nil)
	if got := translator.Trans("general.skinlib"); got != "皮肤库" {
		t.Fatalf("zh_CN general.skinlib=%q", got)
	}
	if got := translator.Trans("general.user-center"); got != "用户中心" {
		t.Fatalf("zh_CN general.user-center=%q", got)
	}

	english := New("en", nil)
	if got := english.Trans("general.skinlib"); got != "Skin Library" {
		t.Fatalf("en general.skinlib=%q", got)
	}

	// 未知 locale 回退到 zh_CN。
	if got := New("xx_XX", nil).Trans("general.login"); got != "登录" {
		t.Fatalf("fallback login=%q", got)
	}

	// language_lines 覆盖优先。
	override := New("zh_CN", map[string]string{"general.skinlib": "纹理库"})
	if got := override.Trans("general.skinlib"); got != "纹理库" {
		t.Fatalf("override=%q", got)
	}

	// :placeholder 替换。
	if got := New("en", nil).Trans("index.introduction", map[string]string{"sitename": "Demo"}); got == "" {
		t.Fatal("index.introduction is empty")
	}
}

func TestCanonical(t *testing.T) {
	cases := map[string]string{
		"zh_HANS_CN": "zh_CN",
		"en_US":      "en",
		"ru":         "ru_RU",
		"en":         "en",
		"unknown":    "zh_CN",
	}
	for input, want := range cases {
		if got := Canonical(input); got != want {
			t.Fatalf("Canonical(%q)=%q want %q", input, got, want)
		}
	}
}
