package controllers

import (
	nethttp "net/http"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/model"
	"neoskin/internal/repo"
)

// AdminClosetController 对应源 ClosetManagementController：
// /admin/closet/{user} 与 /api/admin/closet/{user}。
type AdminClosetController struct{}

func NewAdminClosetController() *AdminClosetController {
	return &AdminClosetController{}
}

func (r *AdminClosetController) target(ctx goravelhttp.Context) (*model.User,
	*repo.UserRepository, goravelhttp.Response) {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return nil, nil, resp
	}
	users := repo.NewUserRepository(conn, dialect)
	target, err := users.FindByID(ctx.Request().RouteInt("user"))
	if err != nil {
		return nil, nil, ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if target == nil {
		return nil, nil, ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	return target, users, nil
}

// List 处理 GET /admin/closet/{user}。
func (r *AdminClosetController) List(ctx goravelhttp.Context) goravelhttp.Response {
	target, users, resp := r.target(ctx)
	if resp != nil {
		return resp
	}
	items, err := repo.NewClosetRepository(users.DB, users.Dialect).AllForUser(target.UID)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	rows := make([]goravelhttp.Json, 0, len(items))
	for _, item := range items {
		rows = append(rows, closetItemJSON(item))
	}
	return ctx.Response().Json(nethttp.StatusOK, rows)
}

// Add 处理 POST /admin/closet/{user}。
func (r *AdminClosetController) Add(ctx goravelhttp.Context) goravelhttp.Response {
	target, users, resp := r.target(ctx)
	if resp != nil {
		return resp
	}
	tid := ctx.Request().InputInt("tid")
	textures := repo.NewTextureRepository(users.DB, users.Dialect)
	texture, err := textures.FindByTid(tid)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if texture == nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.closet.add.not-found"),
		})
	}

	closet := repo.NewClosetRepository(users.DB, users.Dialect)
	if exists, _ := closet.InCloset(target.UID, tid); exists {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.closet.add.repeated"),
		})
	}
	if err := closet.Add(target.UID, tid, texture.Name); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": "",
		"data": goravelhttp.Json{
			"user":    userToJSON(target),
			"texture": textureToJSON(*texture),
		},
	})
}

// Remove 处理 DELETE /admin/closet/{user}。
func (r *AdminClosetController) Remove(ctx goravelhttp.Context) goravelhttp.Response {
	target, users, resp := r.target(ctx)
	if resp != nil {
		return resp
	}
	tid := ctx.Request().InputInt("tid")
	closet := repo.NewClosetRepository(users.DB, users.Dialect)
	item, err := closet.FindItem(target.UID, tid)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if item == nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.closet.remove.non-existent"),
		})
	}
	if err := closet.Remove(target.UID, tid); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": "",
		"data": goravelhttp.Json{
			"user":    userToJSON(target),
			"texture": textureToJSON(item.Texture),
		},
	})
}

func closetItemJSON(item repo.Item) goravelhttp.Json {
	texture := textureToJSON(item.Texture)
	texture["pivot"] = goravelhttp.Json{
		"user_uid":    item.UserUID,
		"texture_tid": item.TextureTID,
		"item_name":   item.ItemName,
	}
	return texture
}

func textureToJSON(texture model.Texture) goravelhttp.Json {
	return goravelhttp.Json{
		"tid": texture.Tid, "name": texture.Name, "type": texture.Type,
		"hash": texture.Hash, "size": texture.Size, "uploader": texture.Uploader,
		"public": texture.Public, "upload_at": texture.UploadAt,
		"likes": texture.Likes, "model": texture.TextureModel(),
	}
}

func userToJSON(user *model.User) userJSON {
	return userJSON{
		UID: user.UID, Email: user.Email, Nickname: user.Nickname, Locale: user.Locale,
		Score: user.Score, Avatar: user.Avatar, IP: user.IP, DarkMode: user.DarkMode,
		Permission: user.Permission, LastSignAt: user.LastSignAt,
		RegisterAt: user.RegisterAt, Verified: user.Verified,
		Verification: user.Verification,
	}
}
