package controllers

import (
	"database/sql"
	"fmt"
	nethttp "net/http"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/db"
	"neoskin/internal/model"
	"neoskin/internal/repo"
	"neoskin/internal/service"
)

type AdminPlayersController struct{}

func NewAdminPlayersController() *AdminPlayersController {
	return &AdminPlayersController{}
}

// List 处理 GET /admin/players/list、/api/admin/players。
func (r *AdminPlayersController) List(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	q := ctx.Request().Input("q")
	page := ctx.Request().InputInt("page", 1)
	if page < 1 {
		page = 1
	}
	const perPage = 10
	players := repo.NewPlayerRepository(conn, dialect)
	total, err := players.AdminCount(q)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	rows, err := players.AdminPage(q, perPage, (page-1)*perPage)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	data := make([]playerJSON, 0, len(rows))
	for _, player := range rows {
		data = append(data, toPlayerJSON(player))
	}
	return ctx.Response().Json(nethttp.StatusOK, paginatorJSON(ctx, data, total, page, perPage))
}

// Name 处理 PUT /admin/players/{player}/name。
func (r *AdminPlayersController) Name(ctx goravelhttp.Context) goravelhttp.Response {
	player, _, conn, dialect, resp := r.target(ctx)
	if resp != nil {
		return resp
	}
	options := service.NewOption(conn, dialect)
	name := ctx.Request().Input("player_name")
	if message := validatePlayerName(ctx, options, "player_name", name); message != "" {
		return validationFailed(ctx, map[string][]string{"player_name": {message}})
	}
	players := repo.NewPlayerRepository(conn, dialect)
	if existing, _ := players.FindDetailByName(name); existing != nil && existing.PID != player.PID {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.player.rename.repeated"),
		})
	}
	if err := players.UpdateName(player.PID, name); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": t(ctx, "admin.players.name.success", "player", name)})
}

// Owner 处理 PUT /admin/players/{player}/owner。
func (r *AdminPlayersController) Owner(ctx goravelhttp.Context) goravelhttp.Response {
	player, _, conn, dialect, resp := r.target(ctx)
	if resp != nil {
		return resp
	}
	uid := ctx.Request().InputInt("uid")
	if uid <= 0 {
		return validationFailed(ctx, map[string][]string{"uid": {vRequired(ctx, "uid")}})
	}
	users := repo.NewUserRepository(conn, dialect)
	owner, err := users.FindByID(uid)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if owner == nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "admin.users.operations.non-existent"),
		})
	}
	if err := repo.NewPlayerRepository(conn, dialect).UpdateOwner(player.PID, uid); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "admin.players.owner.success", "player", player.Name, "user", owner.Nickname),
	})
}

// Texture 处理 PUT /admin/players/{player}/textures。
func (r *AdminPlayersController) Texture(ctx goravelhttp.Context) goravelhttp.Response {
	player, _, conn, dialect, resp := r.target(ctx)
	if resp != nil {
		return resp
	}
	tid := ctx.Request().InputInt("tid")
	textureType := ctx.Request().Input("type")
	if textureType != "skin" && textureType != "cape" {
		return validationFailed(ctx, map[string][]string{"type": {vIn(ctx, "type")}})
	}
	if tid > 0 {
		texture, err := repo.NewTextureRepository(conn, dialect).FindByTid(tid)
		if err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		if texture == nil {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": t(ctx, "admin.players.textures.non-existent", "tid", fmt.Sprint(tid)),
			})
		}
	}
	tidSkin, tidCape := player.TidSkin, player.TidCape
	if textureType == "skin" {
		tidSkin = tid
	} else {
		tidCape = tid
	}
	if err := repo.NewPlayerRepository(conn, dialect).
		UpdateTextures(player.PID, tidSkin, tidCape); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "admin.players.textures.success", "player", player.Name),
	})
}

// Delete 处理 DELETE /admin/players/{player}。
func (r *AdminPlayersController) Delete(ctx goravelhttp.Context) goravelhttp.Response {
	player, _, conn, dialect, resp := r.target(ctx)
	if resp != nil {
		return resp
	}
	if err := repo.NewPlayerRepository(conn, dialect).Delete(player.PID); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": t(ctx, "admin.players.delete.success")})
}

func (r *AdminPlayersController) target(ctx goravelhttp.Context) (*model.PlayerDetail,
	*model.User, *sql.DB, *db.Dialect, goravelhttp.Response) {
	admin, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return nil, nil, nil, nil, resp
	}
	pid := ctx.Request().RouteInt("player")
	players := repo.NewPlayerRepository(conn, dialect)
	player, err := players.FindDetailByID(pid)
	if err != nil {
		return nil, nil, nil, nil, ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if player == nil {
		return nil, nil, nil, nil, ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	if player.UID != admin.UID && player.Permission >= admin.Permission {
		return nil, nil, nil, nil, ctx.Response().String(nethttp.StatusForbidden, "Forbidden")
	}
	return player, admin, conn, dialect, nil
}
