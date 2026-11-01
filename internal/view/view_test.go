package view

import (
	"html/template"
	"path/filepath"
	"strings"
	"testing"

	"neoskin/internal/i18n"
)

func TestRenderHome(t *testing.T) {
	Root = filepath.Join("..", "..", "resources", "pages")
	i18n.Root = filepath.Join("..", "..", "resources", "lang")
	i18n.ResetCache()

	translator := i18n.New("zh_CN", nil)
	body, err := Render("home", HomeData{
		BaseData: BaseData{
			Locale:          "zh_CN",
			SiteName:        "Demo Skin",
			SiteDescription: "Demo description",
			CSRF:            "csrf-token-value",
			ThemeColor:      "#17a2b8",
			BlessingJSON:    `{"version":"6.0.11_patch"}`,
			StyleCSS:        "/app/style.css",
			HomeCSS:         "/app/home.css",
			SpectreCSS:      "/app/spectre.css",
			AppJS:           "/app/app.js",
			I18nURL:         "/lang/zh_CN.js",
			Favicon:         "/app/favicon.ico",
		},
		ExtraJSON:       `{"transparent_navbar":false}`,
		NavbarColor:     "cyan",
		ColorMode:       "dark",
		HomePicURL:      "./app/bg.webp",
		Languages:       []Language{{Name: "English", URL: "/?lang=en"}},
		CurrentLanguage: "中文 (简体)",
	}, translator)
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	for _, needle := range []string{
		`<meta name="csrf-token" content="csrf-token-value">`,
		`window.blessing = {"version":"6.0.11_patch"}`,
		`id="blessing-extra"`,
		`Demo Skin`,
		`/lang/zh_CN.js`,
		`dropdown-item locale`,
		`id="intro"`,
	} {
		if !strings.Contains(html, needle) {
			t.Fatalf("home page missing %q", needle)
		}
	}
}

func TestRenderHomeLoggedIn(t *testing.T) {
	Root = filepath.Join("..", "..", "resources", "pages")
	i18n.Root = filepath.Join("..", "..", "resources", "lang")
	i18n.ResetCache()

	body, err := Render("home", HomeData{
		BaseData: BaseData{
			Locale: "zh_CN", SiteName: "Demo", CSRF: "x",
			BlessingJSON: `{}`,
		},
		ExtraJSON: `{}`,
		LoggedIn:  true, Nickname: "Steve", IsAdmin: true,
		AvatarURL: "/avatar/1?size=36", AvatarPNGURL: "/avatar/1?size=36&png=1",
		HideIntro: true,
	}, i18n.New("zh_CN", nil))
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	for _, needle := range []string{
		`data-mark="nickname">Steve`,
		`id="logout-button"`,
		`href="/admin/users"`,
		`class="without-intro"`,
	} {
		if !strings.Contains(html, needle) {
			t.Fatalf("logged-in home missing %q", needle)
		}
	}
}

func TestThemeColorAndAssets(t *testing.T) {
	if got := ThemeColor("cyan"); got != "#17a2b8" {
		t.Fatalf("theme=%q", got)
	}
	if ColorMode("light") != "light" || ColorMode("cyan") != "dark" {
		t.Fatal("color mode mismatch")
	}
	if got := AssetURL("", "app.js"); got != "/app/app.js" {
		t.Fatalf("asset url=%q", got)
	}
	if got := AssetURL("https://cdn.example.com/", "app.js"); got != "https://cdn.example.com/app/app.js" {
		t.Fatalf("cdn asset url=%q", got)
	}
}

func TestRenderAuthPages(t *testing.T) {
	Root = filepath.Join("..", "..", "resources", "pages")
	i18n.Root = filepath.Join("..", "..", "resources", "lang")
	i18n.ResetCache()

	base := BaseData{
		Locale: "zh_CN", SiteName: "Demo", CSRF: "csrf",
		BlessingJSON: `{}`, StyleCSS: "/app/style.css", SpectreCSS: "/app/spectre.css",
		AppJS: "/app/app.js", I18nURL: "/lang/zh_CN.js", Favicon: "/app/favicon.ico",
	}
	cases := []struct {
		page string
		data AuthData
		want []string
	}{
		{"auth_login", AuthData{BaseData: base, Title: "登录", ExtraJSON: `{"tooManyFails":false}`},
			[]string{`login-page`, `<main></main>`, `id="blessing-extra"`, `/auth/register`}},
		{"auth_register", AuthData{BaseData: base, Title: "注册", ExtraJSON: `{}`},
			[]string{`<main></main>`, `注册`}},
		{"auth_forgot", AuthData{BaseData: base, Title: "忘记密码", ExtraJSON: `{}`},
			[]string{`<main></main>`, `忘记密码`}},
		{"auth_reset", AuthData{BaseData: base, Title: "重置密码", ExtraJSON: `{}`, Nickname: "Steve"},
			[]string{`Steve`, `<main></main>`}},
		{"auth_bind", AuthData{BaseData: withCSRF(base, "csrf"), Title: "绑定邮箱", Error: "出错了"},
			[]string{`action="/auth/bind"`, `name="_token"`, `出错了`}},
		{"auth_verify", AuthData{BaseData: withCSRF(base, "csrf"), Title: "验证邮箱"},
			[]string{`name="email"`, `name="_token"`}},
	}
	for _, item := range cases {
		body, err := Render(item.page, item.data, i18n.New("zh_CN", nil))
		if err != nil {
			t.Fatalf("%s: %v", item.page, err)
		}
		for _, needle := range item.want {
			if !strings.Contains(string(body), needle) {
				t.Fatalf("%s missing %q", item.page, needle)
			}
		}
	}
}

func withCSRF(base BaseData, token string) BaseData {
	base.CSRF = token
	return base
}

func TestRenderUserProfile(t *testing.T) {
	Root = filepath.Join("..", "..", "resources", "pages")
	i18n.Root = filepath.Join("..", "..", "resources", "lang")
	i18n.ResetCache()

	body, err := Render("user_profile", UserData{
		BaseData: BaseData{
			Locale: "zh_CN", SiteName: "Demo", CSRF: "x", BlessingJSON: `{}`,
			StyleCSS: "/app/style.css", SpectreCSS: "/app/spectre.css",
			AppJS: "/app/app.js", I18nURL: "/lang/zh_CN.js", Favicon: "/app/favicon.ico",
		},
		Title: "个人资料", NavbarColor: "cyan", ColorMode: "dark",
		SidebarColor: "dark-maroon", Nickname: "Steve", Email: "steve@example.com",
		AvatarURL: "/avatar/1?size=45", AvatarPNGURL: "/avatar/1?size=45&png=1",
	}, i18n.New("zh_CN", nil))
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	for _, needle := range []string{
		`id="change-nickname"`, `value="Steve"`, `id="change-password"`,
		`id="change-email"`, `value="steve@example.com"`, `reset-avatar`,
		`id="modal-delete-account"`, `data-mark="nickname"`,
	} {
		if !strings.Contains(html, needle) {
			t.Fatalf("profile page missing %q", needle)
		}
	}
}

func TestRenderUserDashboard(t *testing.T) {
	Root = filepath.Join("..", "..", "resources", "pages")
	i18n.Root = filepath.Join("..", "..", "resources", "lang")
	i18n.ResetCache()

	body, err := Render("user_index", UserData{
		BaseData: BaseData{
			Locale: "zh_CN", SiteName: "Demo", CSRF: "x", BlessingJSON: `{}`,
			StyleCSS: "/app/style.css", SpectreCSS: "/app/spectre.css",
			AppJS: "/app/app.js", I18nURL: "/lang/zh_CN.js", Favicon: "/app/favicon.ico",
		},
		Title: "仪表盘", NavbarColor: "cyan", ColorMode: "dark",
		SidebarColor: "dark-maroon", Nickname: "Steve", IsAdmin: true,
		AvatarURL: "/avatar/1?size=45", AvatarPNGURL: "/avatar/1?size=45&png=1",
		MenuItems: []MenuItem{
			{Title: "仪表盘", Link: "/user", Icon: "fa-tachometer-alt", Active: true},
			{Title: "站内信", Link: "/user/inbox", Icon: "fa-envelope"},
			{Title: "高级功能", Icon: "fa-code-branch", Children: []MenuItem{
				{Title: "OAuth2 应用", Link: "/user/oauth/manage", Icon: "fa-feather-alt"},
			}},
		},
		ExploreItems: []MenuItem{{Title: "皮肤库", Link: "/skinlib", Icon: "fa-archive"}},
		Announcement: template.HTML("<p>Hello announcement body.</p>"),
		ScoreIntro:   "积分说明",
		Rates:        []Rate{{Title: "1 积分 = 1 KB"}},
		ExtraJSON:    `{"unverified":false}`,
	}, i18n.New("zh_CN", nil))
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	for _, needle := range []string{
		`main-sidebar`, `sidebar-dark-maroon`, `data-mark="nickname">Steve`,
		`id="usage-box"`, `id="email-verification"`, `modal-score-instruction`,
		`id="blessing-extra"`, `Hello announcement body.`, `href="/admin"`,
		`nav-treeview`, `积分说明`,
	} {
		if !strings.Contains(html, needle) {
			t.Fatalf("user dashboard missing %q", needle)
		}
	}
}
