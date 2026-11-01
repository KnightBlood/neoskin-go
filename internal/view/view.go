// Package view 提供页面模板渲染与静态资源定位。
package view

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"neoskin/internal/i18n"
	"neoskin/internal/plugin"
)

// Language 是语言切换菜单项。
type Language struct {
	Name string
	URL  string
}

// BaseData 是页面公共 head/foot 数据。
type BaseData struct {
	Locale          string
	SiteName        string
	SiteDescription string
	CSRF            string
	ThemeColor      string
	Keywords        string
	Description     string
	MetaExtra       template.HTML
	BlessingJSON    template.JS
	StyleCSS        string
	HomeCSS         string
	SpectreCSS      string
	AppJS           string
	I18nURL         string
	Favicon         string
	CustomCSS       template.HTML
	CustomJS        template.HTML
	PluginStyles    []string
	PluginScripts   []string
	ExtraHead       template.HTML
	ExtraFoot       template.HTML

	CopyrightPreference int
	CustomCopyright     template.HTML
	SiteURL             string
	Year                int
}

// HomeData 是 home.tmpl 的渲染数据。
type HomeData struct {
	BaseData
	ExtraJSON         template.JS
	NavbarColor       string
	ColorMode         string
	TransparentNavbar bool
	FixedBG           bool
	HideIntro         bool
	HomePicURL        string
	LoggedIn          bool
	Nickname          string
	Email             string
	AvatarURL         string
	AvatarPNGURL      string
	IsAdmin           bool
	Languages         []Language
	CurrentLanguage   string
}

// AuthData 是 auth-* 页面（登录/注册/找回/绑定/验证）的数据。
type AuthData struct {
	BaseData
	Title     string
	Message   string
	ExtraJSON template.JS
	Error     string
	Nickname  string
}

// MenuItem 是侧边栏菜单项。
type MenuItem struct {
	Title    string
	Link     string
	Icon     string
	Active   bool
	Children []MenuItem
}

// Rate 是积分说明中的一行。
type Rate struct {
	Title string
	Value string
}

// Pagination 是列表页的分页信息。
type Pagination struct {
	Page    int
	Total   int
	PrevURL string
	NextURL string
}

// ReportItem 是"我的举报"表格行。
type ReportItem struct {
	TID        int
	Reason     string
	StatusText string
	Time       string
	LikeURL    string
}

// MessageItem 是站内信列表行。
type MessageItem struct {
	ID         string
	Title      string
	StatusText string
	Time       string
	URL        string
}

// CommunicationItem 是管理端通知/公告历史行。
type CommunicationItem struct {
	ID          int
	Title       string
	KindText    string
	StatusText  string
	ReadCount   int
	Delivery    int
	UpdatedAt   string
	EditURL     string
	WithdrawURL string
	CanWithdraw bool
}

// UserData 是用户中心页面（user.base）的数据。
type UserData struct {
	BaseData
	Title           string
	DarkMode        bool
	NavbarColor     string
	ColorMode       string
	SidebarColor    string
	Nickname        string
	Email           string
	AvatarURL       string
	AvatarPNGURL    string
	IsAdmin         bool
	MenuItems       []MenuItem
	ExploreItems    []MenuItem
	Badges          []plugin.Badge
	Languages       []Language
	CurrentLanguage string
	Announcement    template.HTML
	ScoreIntro      string
	Rates           []Rate
	ExtraJSON       template.JS

	Reports        []ReportItem
	Messages       []MessageItem
	Pagination     Pagination
	MessageTitle   string
	MessageContent template.HTML
	MessageTime    string

	StatsUsers    string
	StatsPlayers  string
	StatsTextures string
	StatsStorage  string
	Flash         string

	LatestVersion  string
	CurrentVersion string
	UpdateError    string
	UpdateNotice   string
	CanUpdate      bool
	HasPackage     bool
	PackageID      string
	PackageVersion string
	PackageSHA     string
	PackageCount   int
	UploadLimit    string

	Options       map[string]string
	NavbarColors  []string
	SidebarColors []string

	Communications    []CommunicationItem
	CommunicationPage Pagination
	CommunicationNote string

	EditorOpen       bool
	EditingID        int
	EditingStatus    string
	EditingKind      string
	EditorTitle      string
	EditorKind       string
	EditorLocale     string
	EditorSource     string
	EditorMode       string
	EditorReceiver   string
	EditorUID        string
	EditorEmail      string
	EditorBlocksJSON template.JS
}

// SkinlibData 是皮肤库页面（skinlib.base）的数据。
type SkinlibData struct {
	BaseData
	Title           string
	NavbarColor     string
	ColorMode       string
	LoggedIn        bool
	Nickname        string
	AvatarURL       string
	AvatarPNGURL    string
	IsAdmin         bool
	Languages       []Language
	CurrentLanguage string
	ExtraJSON       template.JS
}

var (
	// Root 是页面模板目录（测试可覆盖）。
	Root      = filepath.Join("resources", "pages")
	parseOnce sync.Once
	templates *template.Template
	parseErr  error
)

func load(translator *i18n.Translator) (*template.Template, error) {
	parseOnce.Do(func() {
		files, err := filepath.Glob(filepath.Join(Root, "*.tmpl"))
		if err != nil {
			parseErr = err
			return
		}
		funcs := template.FuncMap{
			"t": func(key string, params ...string) string {
				return translator.Trans(key, pairs(params))
			},
			"nonce": func() string { return "" },
		}
		templates, parseErr = template.New("views").Funcs(funcs).ParseFiles(files...)
	})
	if parseErr != nil {
		return nil, parseErr
	}
	// 每次渲染时更新 t 的绑定 translator（ParseFiles 只做一次）。
	return templates.Clone()
}

// Render 渲染指定模板（例如 "home"）。
func Render(name string, data any, translator *i18n.Translator) ([]byte, error) {
	set, err := load(translator)
	if err != nil {
		return nil, err
	}
	set = set.Funcs(template.FuncMap{
		"t": func(key string, params ...string) string {
			return translator.Trans(key, pairs(params))
		},
	})
	var buffer bytes.Buffer
	if err := set.ExecuteTemplate(&buffer, name, data); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// Asset 优先返回 public/app 下匹配前缀/后缀的真实文件，否则使用 fallback。
func Asset(prefix, suffix, fallback string) string {
	entries, err := os.ReadDir(filepath.Join("public", "app"))
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
				return name
			}
		}
	}
	return fallback
}

// AssetURL 拼接 cdn_base 与 /app/<name>。
func AssetURL(cdnBase, name string) string {
	base := strings.TrimRight(cdnBase, "/")
	if base == "" {
		return "/app/" + name
	}
	return base + "/app/" + name
}

// ThemeColor 与 HeadComposer::applyThemeColor 的色表一致。
func ThemeColor(navbarColor string) string {
	colors := map[string]string{
		"primary": "#007bff", "secondary": "#6c757d", "success": "#28a745",
		"warning": "#ffc107", "danger": "#dc3545", "navy": "#001f3f",
		"olive": "#3d9970", "lime": "#01ff70", "fuchsia": "#f012be",
		"maroon": "#d81b60", "indigo": "#6610f2", "purple": "#6f42c1",
		"pink": "#e83e8c", "orange": "#fd7e14", "teal": "#20c997",
		"cyan": "#17a2b8", "gray": "#6c757d", "light": "#f8f9fa",
		"white": "#ffffff", "dark": "#343a40",
	}
	if color, ok := colors[navbarColor]; ok {
		return color
	}
	return colors["primary"]
}

// ColorMode 判断导航栏配色是 light 还是 dark。
func ColorMode(navbarColor string) string {
	switch navbarColor {
	case "light", "warning", "white", "orange":
		return "light"
	}
	return "dark"
}

func pairs(values []string) map[string]string {
	result := map[string]string{}
	for index := 0; index+1 < len(values); index += 2 {
		result[values[index]] = values[index+1]
	}
	return result
}

// SetupData 是安装向导页面（setup.*）的数据。
type SetupData struct {
	Locale  string
	Title   string
	CSRF    string
	Version string
	Error   string
	Values  map[string]string
}
