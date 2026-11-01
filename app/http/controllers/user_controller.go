package controllers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	nethttp "net/http"
	"strings"
	"time"

	goravelhttp "github.com/goravel/framework/contracts/http"

	localeMiddleware "neoskin/app/http/middleware"
	"neoskin/internal/auth"
	"neoskin/internal/i18n"
	"neoskin/internal/model"
	"neoskin/internal/php"
	"neoskin/internal/plugin"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
	"neoskin/internal/view"
)

type UserController struct{}

func NewUserController() *UserController {
	return &UserController{}
}

func (r *UserController) current(ctx goravelhttp.Context) (*model.User, *repo.UserRepository, error) {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return nil, nil, err
	}
	users := repo.NewUserRepository(conn, dialect)
	user, err := auth.CurrentUser(ctx, users)
	if err != nil {
		return nil, nil, err
	}

	return user, users, nil
}

// Profile 处理 GET /user/profile（当前返回 JSON；模板接入后替换）。
func (r *UserController) Profile(ctx goravelhttp.Context) goravelhttp.Response {
	user, _, err := r.current(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	return ctx.Response().Json(nethttp.StatusOK, toUserJSON(user))
}

// HandleProfile 处理 POST /user/profile（nickname/password/email/delete）。
func (r *UserController) HandleProfile(ctx goravelhttp.Context) goravelhttp.Response {
	user, users, err := r.current(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	action := ctx.Request().Input("action")
	switch action {
	case "nickname":
		nickname := strings.TrimSpace(ctx.Request().Input("new_nickname"))
		if nickname == "" {
			return validationFailed(ctx, map[string][]string{
				"new_nickname": {"The new nickname field is required."},
			})
		}
		if err := users.UpdateNickname(user.UID, nickname); err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventUserProfileUpdated,
			Payload: map[string]any{"uid": user.UID, "action": "nickname"}})
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 0, "message": "Nickname updated.",
		})

	case "password":
		current := ctx.Request().Input("current_password")
		newPassword := ctx.Request().Input("new_password")
		if len(current) < 6 || len(current) > 32 || len(newPassword) < 8 || len(newPassword) > 32 {
			return validationFailed(ctx, map[string][]string{
				"new_password": {"The password must be between 8 and 32 characters."},
			})
		}
		if !php.VerifyPassword(user.Password, current) {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": "Wrong password.",
			})
		}
		hashed, err := php.HashPassword(newPassword, 0)
		if err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		if err := users.UpdatePassword(user.UID, hashed); err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventUserProfileUpdated,
			Payload: map[string]any{"uid": user.UID, "action": "password"}})
		_ = auth.Logout(ctx)
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 0, "message": "Password updated, please login again.",
		})

	case "email":
		email := strings.TrimSpace(ctx.Request().Input("email"))
		password := ctx.Request().Input("password")
		if email == "" || !strings.Contains(email, "@") {
			return validationFailed(ctx, map[string][]string{
				"email": {"The email field must be a valid email address."},
			})
		}
		if count, _ := users.CountByEmail(email); count > 0 {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": "This email has already been taken.",
			})
		}
		if !php.VerifyPassword(user.Password, password) {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": "Wrong password.",
			})
		}
		if err := users.UpdateEmail(user.UID, email); err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventUserProfileUpdated,
			Payload: map[string]any{"uid": user.UID, "action": "email"}})
		_ = auth.Logout(ctx)
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 0, "message": "Email updated, please login again.",
		})

	case "delete":
		password := ctx.Request().Input("password")
		if user.IsAdmin() {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": "Administrators cannot delete their accounts.",
			})
		}
		if !php.VerifyPassword(user.Password, password) {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": "Wrong password.",
			})
		}
		if err := users.Delete(user.UID); err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		_ = auth.Logout(ctx)
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 0, "message": "Your account has been deleted.",
		})
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 1, "message": "Illegal parameters.",
	})
}

// SetAvatar 处理 POST /user/profile/avatar。
func (r *UserController) SetAvatar(ctx goravelhttp.Context) goravelhttp.Response {
	user, users, err := r.current(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	tid := ctx.Request().InputInt("tid")
	if tid < 0 {
		return validationFailed(ctx, map[string][]string{
			"tid": {"The tid field must be an integer."},
		})
	}

	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	textures := repo.NewTextureRepository(conn, dialect)
	if tid > 0 {
		texture, err := textures.FindByTid(tid)
		if err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		if texture == nil {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": "Texture not found.",
			})
		}
		if texture.Type == "cape" {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": "You can only use a skin as avatar.",
			})
		}
		if !texture.Public && texture.Uploader != user.UID && !user.IsAdmin() {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": "This texture is private.",
			})
		}
	}

	if err := users.UpdateAvatar(user.UID, tid); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": "Avatar updated.",
	})
}

// ScoreInfo 处理 GET /user/score-info。
func (r *UserController) ScoreInfo(ctx goravelhttp.Context) goravelhttp.Response {
	user, _, err := r.current(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	options := service.NewOption(conn, dialect)
	players := repo.NewPlayerRepository(conn, dialect)
	textures := repo.NewTextureRepository(conn, dialect)
	playerCount, _ := players.CountByUID(user.UID)
	storage, _ := textures.SumSizeByUploader(user.UID)

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"user": goravelhttp.Json{
			"score":      user.Score,
			"lastSignAt": user.LastSignAt,
		},
		"rate": goravelhttp.Json{
			"storage": options.Int("score_per_storage", 0),
			"players": options.Int("score_per_player", 0),
		},
		"usage": goravelhttp.Json{
			"players": playerCount,
			"storage": storage,
		},
		"signAfterZero": options.Bool("sign_after_zero", false),
		"signGapTime":   options.Int("sign_gap_time", 24),
	})
}

// Sign 处理 POST /user/sign。
func (r *UserController) Sign(ctx goravelhttp.Context) goravelhttp.Response {
	user, users, err := r.current(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	options := service.NewOption(conn, dialect)

	location, _ := time.LoadLocation("Asia/Shanghai")
	layout := "2006-01-02 15:04:05"
	now := time.Now().In(location)
	last, _ := time.ParseInLocation(layout, user.LastSignAt, location)
	canSign := last.IsZero()
	if !canSign {
		if options.Bool("sign_after_zero", false) {
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
			canSign = last.Before(today)
		} else {
			gap := time.Duration(options.Int("sign_gap_time", 24)) * time.Hour
			canSign = !now.Before(last.Add(gap))
		}
	}
	if !canSign {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": "",
		})
	}

	minScore, maxScore := 10, 100
	if parts := strings.Split(options.Get("sign_score"), ","); len(parts) == 2 {
		_, _ = fmt.Sscan(parts[0], &minScore)
		_, _ = fmt.Sscan(parts[1], &maxScore)
	}
	if maxScore < minScore {
		maxScore = minScore
	}
	acquired := minScore + rand.Intn(maxScore-minScore+1)
	score := user.Score + acquired
	if err := users.UpdateScoreAndSign(user.UID, score, now.Format(layout)); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": fmt.Sprintf("Signed in and got %d points.", acquired),
		"data": goravelhttp.Json{"score": score},
	})
}

// DarkMode 处理 PUT /user/dark-mode（切换）。
func (r *UserController) DarkMode(ctx goravelhttp.Context) goravelhttp.Response {
	user, users, err := r.current(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	if err := users.UpdateDarkMode(user.UID, !user.DarkMode); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	return ctx.Response().String(nethttp.StatusNoContent, "")
}

func toUserJSON(user *model.User) userJSON {
	return userJSON{
		UID: user.UID, Email: user.Email, Nickname: user.Nickname, Locale: user.Locale,
		Score: user.Score, Avatar: user.Avatar, IP: user.IP, DarkMode: user.DarkMode,
		Permission: user.Permission, LastSignAt: user.LastSignAt,
		RegisterAt: user.RegisterAt, Verified: user.Verified, Verification: user.Verification,
	}
}

// Index 处理 GET /user：用户中心仪表盘。
func (r *UserController) Index(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Redirect(nethttp.StatusFound, "/auth/login")
	}

	options := service.NewOption(conn, dialect)
	locale := localeMiddleware.LocaleOf(ctx)
	translator := i18n.New(locale, repo.NewLanguageLineRepository(conn, dialect).Overrides(locale))
	siteName := localizedOption(options, "site_name", locale, "Blessing Skin")

	navbarColor := options.Get("navbar_color")
	if navbarColor == "" {
		navbarColor = "cyan"
	}
	sidebarColor := options.Get("sidebar_color")
	if sidebarColor == "" {
		sidebarColor = "dark-maroon"
	}
	if user.DarkMode {
		sidebarColor = strings.ReplaceAll(sidebarColor, "light", "dark")
	}

	current := strings.Trim(ctx.Request().Path(), "/")
	item := func(titleKey, link, icon string) view.MenuItem {
		return view.MenuItem{
			Title: translator.Trans(titleKey), Link: "/" + link, Icon: icon,
			Active: current == link || strings.HasPrefix(current, link+"/"),
		}
	}
	menu := []view.MenuItem{
		item("general.dashboard", "user", "fa-tachometer-alt"),
		{Title: "站内信", Link: "/user/inbox", Icon: "fa-envelope",
			Active: strings.HasPrefix(current, "user/inbox")},
		item("general.my-closet", "user/closet", "fa-star"),
		item("general.player-manage", "user/player", "fa-users"),
		item("general.my-reports", "user/reports", "fa-flag"),
		item("general.profile", "user/profile", "fa-user"),
		{
			Title: translator.Trans("general.developer"), Icon: "fa-code-branch",
			Active: strings.HasPrefix(current, "user/oauth"),
			Children: []view.MenuItem{
				item("general.oauth-manage", "user/oauth/manage", "fa-feather-alt"),
			},
		},
	}

	minimum, maximum := "0", "0"
	if parts := strings.Split(options.Get("sign_score"), ","); len(parts) == 2 {
		minimum, maximum = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	returnScore := "no-return-score"
	if options.Bool("return_score", true) {
		returnScore = "will-return-score"
	}
	scoreIntro := translator.Trans("user.score-intro.introduction", map[string]string{
		"initial_score": options.Get("user_initial_score"),
		"score-from":    minimum,
		"score-to":      maximum,
		"return-score":  translator.Trans("user.score-intro." + returnScore),
	})
	rates := []view.Rate{
		{Title: translator.Trans("user.score-intro.rates.storage",
			map[string]string{"score": options.Get("score_per_storage")})},
		{Title: translator.Trans("user.score-intro.rates.player",
			map[string]string{"score": options.Get("score_per_player")})},
		{Title: translator.Trans("user.score-intro.rates.closet",
			map[string]string{"score": options.Get("score_per_closet_item")})},
	}

	announcement := template.HTML("")
	if row, err := repo.NewCommunicationRepository(conn, dialect).
		FindLatestAnnouncement(locale); err == nil && row != nil &&
		row.Status == repo.CommunicationPublished {
		announcement = template.HTML(renderCommunication(row.Source, row.Blocks))
	}

	extra, _ := json.Marshal(map[string]any{
		"unverified": options.Bool("require_verification", false) && !user.Verified,
	})

	data := view.UserData{
		BaseData:        buildBaseData(ctx, options, locale, siteName),
		Title:           translator.Trans("general.dashboard"),
		DarkMode:        user.DarkMode,
		NavbarColor:     navbarColor,
		ColorMode:       view.ColorMode(navbarColor),
		SidebarColor:    sidebarColor,
		Nickname:        user.Nickname,
		AvatarURL:       fmt.Sprintf("/avatar/%d?size=45", user.Avatar),
		AvatarPNGURL:    fmt.Sprintf("/avatar/%d?size=45&png=1", user.Avatar),
		IsAdmin:         user.IsAdmin(),
		MenuItems:       menu,
		ExploreItems:    append([]view.MenuItem{item("general.skinlib", "skinlib", "fa-archive")}, pluginMenuItems("explore")...),
		Languages:       languageOptions(ctx, locale),
		CurrentLanguage: i18n.Name(locale),
		Announcement:    announcement,
		ScoreIntro:      scoreIntro,
		Rates:           rates,
		ExtraJSON:       template.JS(extra),
	}
	if data.Nickname == "" {
		data.Nickname = user.Email
	}

	body, err := view.Render("user_index", data, translator)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Data(nethttp.StatusOK, "text/html; charset=utf-8", body)
}

// userPageShell 构建用户中心最外层布局（header/sidebar/user-panel）。
func (r *UserController) userPageShell(ctx goravelhttp.Context, titleKey string) (*view.UserData,
	*i18n.Translator, goravelhttp.Response) {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return nil, nil, ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return nil, nil, ctx.Response().Redirect(nethttp.StatusFound, "/auth/login")
	}

	options := service.NewOption(conn, dialect)
	locale := localeMiddleware.LocaleOf(ctx)
	translator := i18n.New(locale, repo.NewLanguageLineRepository(conn, dialect).Overrides(locale))
	siteName := localizedOption(options, "site_name", locale, "Blessing Skin")

	navbarColor := options.Get("navbar_color")
	if navbarColor == "" {
		navbarColor = "cyan"
	}
	sidebarColor := options.Get("sidebar_color")
	if sidebarColor == "" {
		sidebarColor = "dark-maroon"
	}
	if user.DarkMode {
		sidebarColor = strings.ReplaceAll(sidebarColor, "light", "dark")
	}

	current := strings.Trim(ctx.Request().Path(), "/")
	item := func(key, link, icon string) view.MenuItem {
		return view.MenuItem{
			Title: translator.Trans(key), Link: "/" + link, Icon: icon,
			Active: current == link || strings.HasPrefix(current, link+"/"),
		}
	}
	data := view.UserData{
		BaseData:        buildBaseData(ctx, options, locale, siteName),
		Title:           translator.Trans(titleKey),
		DarkMode:        user.DarkMode,
		NavbarColor:     navbarColor,
		ColorMode:       view.ColorMode(navbarColor),
		SidebarColor:    sidebarColor,
		Nickname:        user.Nickname,
		Email:           user.Email,
		AvatarURL:       fmt.Sprintf("/avatar/%d?size=45", user.Avatar),
		AvatarPNGURL:    fmt.Sprintf("/avatar/%d?size=45&png=1", user.Avatar),
		IsAdmin:         user.IsAdmin(),
		Languages:       languageOptions(ctx, locale),
		CurrentLanguage: i18n.Name(locale),
		MenuItems: []view.MenuItem{
			item("general.dashboard", "user", "fa-tachometer-alt"),
			{Title: "站内信", Link: "/user/inbox", Icon: "fa-envelope",
				Active: strings.HasPrefix(current, "user/inbox")},
			item("general.my-closet", "user/closet", "fa-star"),
			item("general.player-manage", "user/player", "fa-users"),
			item("general.my-reports", "user/reports", "fa-flag"),
			item("general.profile", "user/profile", "fa-user"),
			{
				Title: translator.Trans("general.developer"), Icon: "fa-code-branch",
				Active: strings.HasPrefix(current, "user/oauth"),
				Children: []view.MenuItem{
					item("general.oauth-manage", "user/oauth/manage", "fa-feather-alt"),
				},
			},
		},
		ExploreItems: []view.MenuItem{item("general.skinlib", "skinlib", "fa-archive")},
		ExtraJSON:    "{}",
	}
	data.MenuItems = append(data.MenuItems, pluginMenuItems("user")...)
	badges := plugin.DefaultHook.UserBadges()
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventRenderingBadges, Payload: &badges})
	data.Badges = badges
	if data.Nickname == "" {
		data.Nickname = user.Email
	}
	return &data, translator, nil
}

func renderUserPage(ctx goravelhttp.Context, templateName string, data *view.UserData,
	translator *i18n.Translator) goravelhttp.Response {
	body, err := view.Render(templateName, data, translator)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Data(nethttp.StatusOK, "text/html; charset=utf-8", body)
}

// ProfilePage 处理 GET /user/profile。
func (r *UserController) ProfilePage(ctx goravelhttp.Context) goravelhttp.Response {
	data, translator, resp := r.userPageShell(ctx, "general.profile")
	if resp != nil {
		return resp
	}
	return renderUserPage(ctx, "user_profile", data, translator)
}

// PlayerPage 处理 GET /user/player。
func (r *UserController) PlayerPage(ctx goravelhttp.Context) goravelhttp.Response {
	data, translator, resp := r.userPageShell(ctx, "general.player-manage")
	if resp != nil {
		return resp
	}
	return renderUserPage(ctx, "user_player", data, translator)
}

// ClosetPage 处理 GET /user/closet。
func (r *UserController) ClosetPage(ctx goravelhttp.Context) goravelhttp.Response {
	data, translator, resp := r.userPageShell(ctx, "general.my-closet")
	if resp != nil {
		return resp
	}
	return renderUserPage(ctx, "user_closet", data, translator)
}

// OAuthPage 处理 GET /user/oauth/manage。
func (r *UserController) OAuthPage(ctx goravelhttp.Context) goravelhttp.Response {
	data, translator, resp := r.userPageShell(ctx, "general.oauth-manage")
	if resp != nil {
		return resp
	}
	return renderUserPage(ctx, "user_oauth", data, translator)
}

func reportStatusText(status int) string {
	switch status {
	case 1:
		return "处理完成"
	case 2:
		return "已被拒绝"
	default:
		return "正在处理"
	}
}

func pagination(base string, page, perPage, total int) view.Pagination {
	result := view.Pagination{Page: page, Total: total}
	if page > 1 {
		result.PrevURL = fmt.Sprintf("%s?page=%d", base, page-1)
	}
	if page*perPage < total {
		result.NextURL = fmt.Sprintf("%s?page=%d", base, page+1)
	}
	return result
}

// ReportsPage 处理 GET /user/reports。
func (r *UserController) ReportsPage(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Redirect(nethttp.StatusFound, "/auth/login")
	}
	page := ctx.Request().InputInt("page", 1)
	if page < 1 {
		page = 1
	}
	const perPage = 10
	reports := repo.NewReportRepository(conn, dialect)
	total, _ := reports.CountByReporter(user.UID)
	rows, err := reports.PageByReporter(user.UID, perPage, (page-1)*perPage)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	data, translator, resp := r.userPageShell(ctx, "general.my-reports")
	if resp != nil {
		return resp
	}
	items := make([]view.ReportItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, view.ReportItem{
			TID: row.TID, Reason: row.Reason,
			StatusText: reportStatusText(row.Status), Time: row.ReportAt,
		})
	}
	data.Reports = items
	data.Pagination = pagination("/user/reports", page, perPage, total)
	return renderUserPage(ctx, "user_reports", data, translator)
}

// InboxPage 处理 GET /user/inbox。
func (r *UserController) InboxPage(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Redirect(nethttp.StatusFound, "/auth/login")
	}
	page := ctx.Request().InputInt("page", 1)
	if page < 1 {
		page = 1
	}
	const perPage = 10
	notifications := repo.NewNotificationRepository(conn, dialect)
	total, _ := notifications.CountByUser(user.UID)
	rows, err := notifications.PageByUser(user.UID, perPage, (page-1)*perPage)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	data, translator, resp := r.userPageShell(ctx, "user/inbox")
	if resp != nil {
		return resp
	}
	items := make([]view.MessageItem, 0, len(rows))
	for _, row := range rows {
		title, _ := row.Data["title"].(string)
		status := "未读"
		if row.ReadAt != "" {
			status = "已读"
		}
		items = append(items, view.MessageItem{
			ID: row.ID, Title: title, StatusText: status,
			Time: row.CreatedAt, URL: "/user/inbox/" + row.ID,
		})
	}
	data.Messages = items
	data.Title = "站内信"
	data.Pagination = pagination("/user/inbox", page, perPage, total)
	return renderUserPage(ctx, "user_inbox", data, translator)
}

// MessagePage 处理 GET /user/inbox/{id}。
func (r *UserController) MessagePage(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Redirect(nethttp.StatusFound, "/auth/login")
	}
	id := ctx.Request().Route("id")
	notifications := repo.NewNotificationRepository(conn, dialect)
	row, err := notifications.FindByUser(user.UID, id)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if row == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	if row.ReadAt == "" {
		_ = notifications.MarkRead(user.UID, id)
	}

	data, translator, resp := r.userPageShell(ctx, "user/inbox")
	if resp != nil {
		return resp
	}
	title, _ := row.Data["title"].(string)
	content, _ := row.Data["content"].(string)
	data.Title = title
	data.MessageTitle = title
	data.MessageContent = template.HTML(content)
	data.MessageTime = row.CreatedAt
	return renderUserPage(ctx, "user_message", data, translator)
}

// ruleAndLength 返回角色名规则与长度提示（供玩家/衣柜页面 extra 使用）。
func ruleAndLength(ctx goravelhttp.Context) (string, string) {
	_, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return "", ""
	}
	options := service.NewOption(conn, dialect)
	translator := i18n.New(localeMiddleware.LocaleOf(ctx), nil)
	rule := options.Get("player_name_rule")
	if rule == "" {
		rule = "official"
	}
	min := options.Int("player_name_length_min", 3)
	max := options.Int("player_name_length_max", 16)
	return translator.Trans("user.player.player-name-rule." + rule),
		translator.Trans("user.player.player-name-length",
			map[string]string{"min": fmt.Sprint(min), "max": fmt.Sprint(max)})
}

func playerPageExtra(ctx goravelhttp.Context) template.JS {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil || user == nil {
		return ""
	}
	rule, length := ruleAndLength(ctx)
	count, _ := repo.NewPlayerRepository(conn, dialect).CountByUID(user.UID)
	options := service.NewOption(conn, dialect)
	encoded, _ := json.Marshal(map[string]any{
		"count": count, "rule": rule, "length": length,
		"unverified": options.Bool("require_verification", false) && !user.Verified,
	})
	return template.JS(encoded)
}

func closetPageExtra(ctx goravelhttp.Context) template.JS {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil || user == nil {
		return ""
	}
	rule, length := ruleAndLength(ctx)
	options := service.NewOption(conn, dialect)
	encoded, _ := json.Marshal(map[string]any{
		"unverified": options.Bool("require_verification", false) && !user.Verified,
		"rule":       rule,
		"length":     length,
	})
	return template.JS(encoded)
}
