package controllers

import (
	"database/sql"
	"fmt"
	nethttp "net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/auth"
	"neoskin/internal/db"
	"neoskin/internal/model"
	"neoskin/internal/plugin"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
)

type PlayerController struct{}

func NewPlayerController() *PlayerController {
	return &PlayerController{}
}

type playerJSON struct {
	PID          int    `json:"pid"`
	UID          int    `json:"uid"`
	Name         string `json:"name"`
	TidCape      int    `json:"tid_cape"`
	LastModified string `json:"last_modified"`
	TidSkin      int    `json:"tid_skin"`
}

func toPlayerJSON(player model.Player) playerJSON {
	return playerJSON{
		PID: player.PID, UID: player.UID, Name: player.Name,
		TidCape: player.TidCape, LastModified: player.LastModified, TidSkin: player.TidSkin,
	}
}

// List 处理 GET /api/players、/user/player/list、/user/player。
func (r *PlayerController) List(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}
	if denied := requireVerifiedOnUserRoutes(ctx, user, service.NewOption(conn, dialect)); denied != nil {
		return denied
	}

	players, err := repo.NewPlayerRepository(conn, dialect).ListByUID(user.UID)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	result := make([]playerJSON, 0, len(players))
	for _, player := range players {
		result = append(result, toPlayerJSON(player))
	}

	return ctx.Response().Json(nethttp.StatusOK, result)
}

// Add 处理 POST /api/players、POST /user/player。
func (r *PlayerController) Add(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	options := service.NewOption(conn, dialect)
	if denied := requireVerifiedOnUserRoutes(ctx, user, options); denied != nil {
		return denied
	}
	players := repo.NewPlayerRepository(conn, dialect)
	users := repo.NewUserRepository(conn, dialect)

	name := strings.TrimSpace(ctx.Request().Input("name"))
	if message := validatePlayerName(ctx, options, "name", name); message != "" {
		return validationFailed(ctx, map[string][]string{"name": {message}})
	}
	if existing, _ := players.FindDetailByName(name); existing != nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.player.add.repeated"),
		})
	}

	cost := options.Int("score_per_player", 100)
	if user.Score < cost {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 7, "message": fmt.Sprintf("You need %d points to add a player.", cost),
		})
	}

	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventPlayerWillBeAdded,
		Payload: map[string]any{"uid": user.UID, "name": name}})
	pid, err := players.InsertWithSkin(user.UID, name, 0)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if err := users.UpdateScore(user.UID, user.Score-cost); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	player, _ := players.FindByID(pid)
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventPlayerWasAdded, Payload: pid})
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "user.player.add.success", "name", name), "data": toPlayerJSON(*player),
	})
}

// Delete 处理 DELETE /api/players/{player}、/user/player/{player}。
func (r *PlayerController) Delete(ctx goravelhttp.Context) goravelhttp.Response {
	player, user, conn, dialect, resp := r.owned(ctx)
	if resp != nil {
		return resp
	}

	options := service.NewOption(conn, dialect)
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventPlayerWillBeDeleted,
		Payload: map[string]any{"pid": player.PID}})
	if err := repo.NewPlayerRepository(conn, dialect).Delete(player.PID); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	if options.Bool("return_score", true) {
		cost := options.Int("score_per_player", 100)
		_ = repo.NewUserRepository(conn, dialect).UpdateScore(user.UID, user.Score+cost)
	}
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventPlayerWasDeleted, Payload: player.PID})

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "user.player.delete.success", "name", player.Name),
	})
}

// Rename 处理 PUT /api/players/{player}/name、/user/player/{player}/name。
func (r *PlayerController) Rename(ctx goravelhttp.Context) goravelhttp.Response {
	player, _, conn, dialect, resp := r.owned(ctx)
	if resp != nil {
		return resp
	}

	options := service.NewOption(conn, dialect)
	players := repo.NewPlayerRepository(conn, dialect)
	name := strings.TrimSpace(ctx.Request().Input("name"))
	if message := validatePlayerName(ctx, options, "name", name); message != "" {
		return validationFailed(ctx, map[string][]string{"name": {message}})
	}
	if existing, _ := players.FindDetailByName(name); existing != nil && existing.PID != player.PID {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.player.rename.repeated"),
		})
	}

	if err := players.UpdateName(player.PID, name); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	updated, _ := players.FindByID(player.PID)
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventPlayerProfileUpdated,
		Payload: map[string]any{"pid": player.PID, "action": "rename"}})
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "user.player.rename.success", "old", player.Name, "new", name), "data": toPlayerJSON(*updated),
	})
}

// SetTextures 处理 PUT /api/players/{player}/textures、/user/player/{player}/textures。
func (r *PlayerController) SetTextures(ctx goravelhttp.Context) goravelhttp.Response {
	player, user, conn, dialect, resp := r.owned(ctx)
	if resp != nil {
		return resp
	}

	textures := repo.NewTextureRepository(conn, dialect)
	closet := repo.NewClosetRepository(conn, dialect)
	players := repo.NewPlayerRepository(conn, dialect)

	tidSkin, tidCape := player.TidSkin, player.TidCape
	for _, kind := range []string{"skin", "cape"} {
		tid := ctx.Request().InputInt(kind)
		if tid <= 0 {
			continue
		}
		texture, err := textures.FindByTid(tid)
		if err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		if texture == nil {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": t(ctx, "skinlib.non-existent"),
			})
		}
		ok, err := closet.InCloset(user.UID, tid)
		if err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		if !ok {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": t(ctx, "user.closet.remove.non-existent"),
			})
		}
		if kind == "skin" {
			tidSkin = tid
		} else {
			tidCape = tid
		}
	}

	if err := players.UpdateTextures(player.PID, tidSkin, tidCape); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	updated, _ := players.FindByID(player.PID)
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventPlayerProfileUpdated,
		Payload: map[string]any{"pid": player.PID, "action": "textures"}})
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "user.player.set.success", "name", player.Name), "data": toPlayerJSON(*updated),
	})
}

// ClearTextures 处理 DELETE /api/players/{player}/textures、/user/player/{player}/textures。
func (r *PlayerController) ClearTextures(ctx goravelhttp.Context) goravelhttp.Response {
	player, _, conn, dialect, resp := r.owned(ctx)
	if resp != nil {
		return resp
	}

	types := ctx.Request().InputArray("type")
	has := func(kind string) bool {
		for _, value := range types {
			if value == kind {
				return true
			}
		}
		return false
	}

	tidSkin, tidCape := player.TidSkin, player.TidCape
	if has("skin") || ctx.Request().Input("skin") != "" {
		tidSkin = 0
	}
	if has("cape") || ctx.Request().Input("cape") != "" {
		tidCape = 0
	}

	players := repo.NewPlayerRepository(conn, dialect)
	if err := players.UpdateTextures(player.PID, tidSkin, tidCape); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventPlayerProfileUpdated,
		Payload: map[string]any{"pid": player.PID, "action": "clear_textures"}})

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "user.player.clear.success", "name", player.Name),
	})
}

func (r *PlayerController) owned(ctx goravelhttp.Context) (*model.Player, *model.User,
	*sql.DB, *db.Dialect, goravelhttp.Response) {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return nil, nil, nil, nil, ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return nil, nil, nil, nil, ctx.Response().Json(nethttp.StatusUnauthorized,
			goravelhttp.Json{"message": "Unauthenticated."})
	}
	if denied := requireVerifiedOnUserRoutes(ctx, user, service.NewOption(conn, dialect)); denied != nil {
		return nil, nil, nil, nil, denied
	}

	pid := ctx.Request().RouteInt("player")
	player, err := repo.NewPlayerRepository(conn, dialect).FindByID(pid)
	if err != nil {
		return nil, nil, nil, nil, ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if player == nil {
		return nil, nil, nil, nil, ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	if player.UID != user.UID {
		return nil, nil, nil, nil, ctx.Response().Json(nethttp.StatusForbidden,
			goravelhttp.Json{"code": 1, "message": t(ctx, "skinlib.no-permission")})
	}

	return player, user, conn, dialect, nil
}

func currentUser(ctx goravelhttp.Context) (*model.User, *sql.DB, *db.Dialect, error) {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return nil, nil, nil, err
	}
	user, err := auth.CurrentUser(ctx, repo.NewUserRepository(conn, dialect))
	if err != nil {
		return nil, nil, nil, err
	}
	return user, conn, dialect, nil
}

// validatePlayerName 复刻 PHP `Rules\PlayerName` + `min/max` 规则：
// 空值 → required；规则不符 → user.player.player-name-rule.<rule>；
// 长度按字符计（等价 PHP mb_strlen）→ min/max.string。返回空串表示通过。
func validatePlayerName(ctx goravelhttp.Context, options *service.Option, field, name string) string {
	minLength := options.Int("player_name_length_min", 3)
	maxLength := options.Int("player_name_length_max", 16)
	if name == "" {
		return vRequired(ctx, field)
	}
	if !playerNameAllowed(options, name) {
		rule := options.Get("player_name_rule")
		if rule == "" {
			rule = "official"
		}
		return t(ctx, "user.player.player-name-rule."+rule)
	}
	length := utf8.RuneCountInString(name)
	if length < minLength {
		return vMinString(ctx, field, strconv.Itoa(minLength))
	}
	if length > maxLength {
		return vMaxString(ctx, field, strconv.Itoa(maxLength))
	}
	return ""
}

var (
	officialPlayerName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	cjkPlayerName      = regexp.MustCompile(`^[A-Za-z0-9_§\x{4e00}-\x{9fff}]+$`)
)

// playerNameAllowed 对应 PHP Rules\PlayerName::passes 的四种规则。
func playerNameAllowed(options *service.Option, name string) bool {
	return playerNameMatches(options.Get("player_name_rule"),
		options.Get("custom_player_name_regexp"), name)
}

// playerNameMatches 是 playerNameAllowed 的纯函数核心（便于单测）。
func playerNameMatches(rule, customPattern, name string) bool {
	switch rule {
	case "", "official":
		return officialPlayerName.MatchString(name)
	case "cjk":
		return cjkPlayerName.MatchString(name)
	case "utf8":
		if !utf8.ValidString(name) {
			return false
		}
		return !strings.ContainsFunc(name, unicode.IsSpace)
	case "custom":
		if customPattern == "" {
			return true
		}
		re, ok := compilePHPPattern(customPattern)
		if !ok {
			return false
		}
		return re.MatchString(name)
	}
	return true
}

// compilePHPPattern 把 PHP preg 风格 "/pattern/flags" 转成 Go regexp；
// 不含分隔符时按原样编译（兼容管理员填写裸正则的情况）。
func compilePHPPattern(pattern string) (re *regexp.Regexp, ok bool) {
	if body, flags, delimited := splitPHPPattern(pattern); delimited {
		var prefix string
		if strings.ContainsRune(flags, 'i') {
			prefix += "(?i)"
		}
		if strings.ContainsRune(flags, 'm') {
			prefix += "(?m)"
		}
		if strings.ContainsRune(flags, 's') {
			prefix += "(?s)"
		}
		if compiled, err := regexp.Compile(prefix + body); err == nil {
			return compiled, true
		}
	}
	compiled, err := regexp.Compile(pattern)
	return compiled, err == nil
}

func splitPHPPattern(pattern string) (string, string, bool) {
	if len(pattern) < 3 {
		return "", "", false
	}
	delim := pattern[0]
	if (delim >= 'a' && delim <= 'z') || (delim >= 'A' && delim <= 'Z') ||
		(delim >= '0' && delim <= '9') || delim == '\\' || delim == ' ' {
		return "", "", false
	}
	index := -1
	for cursor := len(pattern) - 1; cursor > 0; cursor-- {
		if pattern[cursor] == delim && pattern[cursor-1] != '\\' {
			index = cursor
			break
		}
	}
	if index <= 0 {
		return "", "", false
	}
	flags := pattern[index+1:]
	if strings.Trim(flags, "imsxuADU") != "" {
		return "", "", false
	}
	return pattern[1:index], flags, true
}
