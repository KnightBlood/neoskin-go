package controllers

import (
	"database/sql"
	"fmt"
	nethttp "net/http"
	"regexp"
	"strings"

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
	players := repo.NewPlayerRepository(conn, dialect)
	users := repo.NewUserRepository(conn, dialect)

	name := strings.TrimSpace(ctx.Request().Input("name"))
	if message := validatePlayerName(options, name); message != "" {
		return validationFailed(ctx, map[string][]string{"name": {message}})
	}
	if existing, _ := players.FindDetailByName(name); existing != nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": "This player name is already taken.",
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
		"code": 0, "message": "Player added.", "data": toPlayerJSON(*player),
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
		"code": 0, "message": "Player deleted.",
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
	if message := validatePlayerName(options, name); message != "" {
		return validationFailed(ctx, map[string][]string{"name": {message}})
	}
	if existing, _ := players.FindDetailByName(name); existing != nil && existing.PID != player.PID {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": "This player name is already taken.",
		})
	}

	if err := players.UpdateName(player.PID, name); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	updated, _ := players.FindByID(player.PID)
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventPlayerProfileUpdated,
		Payload: map[string]any{"pid": player.PID, "action": "rename"}})
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": "Player renamed.", "data": toPlayerJSON(*updated),
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
				"code": 1, "message": "Texture not found.",
			})
		}
		ok, err := closet.InCloset(user.UID, tid)
		if err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		if !ok {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": "Texture is not in your closet.",
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
		"code": 0, "message": "Texture applied.", "data": toPlayerJSON(*updated),
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
		"code": 0, "message": "Texture cleared.",
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
			goravelhttp.Json{"code": 1, "message": "No permission."})
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

func validatePlayerName(options *service.Option, name string) string {
	minLength := options.Int("player_name_length_min", 3)
	maxLength := options.Int("player_name_length_max", 16)
	if name == "" || len(name) < minLength || len(name) > maxLength {
		return fmt.Sprintf("The player name must be between %d and %d characters.",
			minLength, maxLength)
	}
	if options.Get("player_name_rule") == "official" &&
		!regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(name) {
		return "Player name may only contain letters, numbers and underscores."
	}
	return ""
}
