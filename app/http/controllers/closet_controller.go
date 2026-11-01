package controllers

import (
	"fmt"
	nethttp "net/http"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/repo"
	"neoskin/internal/service"
)

type ClosetController struct{}

func NewClosetController() *ClosetController {
	return &ClosetController{}
}

type closetTextureJSON struct {
	TID      int    `json:"tid"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Hash     string `json:"hash"`
	Size     int    `json:"size"`
	Uploader int    `json:"uploader"`
	Public   bool   `json:"public"`
	UploadAt string `json:"upload_at"`
	Likes    int    `json:"likes"`
	Model    string `json:"model"`
	Pivot    struct {
		UserUID    int    `json:"user_uid"`
		TextureTID int    `json:"texture_tid"`
		ItemName   string `json:"item_name"`
	} `json:"pivot"`
}

// List 处理 GET /api/closet、/user/closet/list、/user/closet。
func (r *ClosetController) List(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	category := ctx.Request().Input("category", "skin")
	search := ctx.Request().Input("q")
	page := ctx.Request().InputInt("page", 1)
	if page < 1 {
		page = 1
	}
	perPage := ctx.Request().InputInt("perPage", 6)
	if perPage <= 0 {
		perPage = 6
	}

	closet := repo.NewClosetRepository(conn, dialect)
	total, err := closet.Count(user.UID, category, search)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	lastPage := (total + perPage - 1) / perPage
	if lastPage < 1 {
		lastPage = 1
	}
	offset := (page - 1) * perPage
	items, err := closet.Page(user.UID, category, search, perPage, offset)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	rows := make([]closetTextureJSON, 0, len(items))
	for _, item := range items {
		t := item.Texture
		row := closetTextureJSON{
			TID: t.Tid, Name: t.Name, Type: t.Type, Hash: t.Hash, Size: t.Size,
			Uploader: t.Uploader, Public: t.Public, UploadAt: t.UploadAt,
			Likes: t.Likes, Model: t.TextureModel(),
		}
		row.Pivot.UserUID = item.UserUID
		row.Pivot.TextureTID = item.TextureTID
		row.Pivot.ItemName = item.ItemName
		rows = append(rows, row)
	}

	path := "http://" + ctx.Request().Host() + ctx.Request().Path()
	var from, to any
	if len(rows) > 0 {
		from = offset + 1
		to = offset + len(rows)
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"current_page":   page,
		"data":           rows,
		"first_page_url": pageURL(path, 1),
		"from":           from,
		"last_page":      lastPage,
		"last_page_url":  pageURL(path, lastPage),
		"links":          paginationLinks(path, page, lastPage),
		"next_page_url":  nextPageURL(path, page, lastPage),
		"path":           path,
		"per_page":       perPage,
		"prev_page_url":  prevPageURL(path, page),
		"to":             to,
		"total":          total,
	})
}

// IDs 处理 GET /user/closet/ids。
func (r *ClosetController) IDs(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	ids, err := repo.NewClosetRepository(conn, dialect).AllIDs(user.UID)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if ids == nil {
		ids = []int{}
	}

	return ctx.Response().Json(nethttp.StatusOK, ids)
}

// Add 处理 POST /api/closet、POST /user/closet。
func (r *ClosetController) Add(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	tid := ctx.Request().InputInt("tid")
	name := ctx.Request().Input("name")
	if tid <= 0 || name == "" {
		errs := map[string][]string{}
		if tid <= 0 {
			errs["tid"] = []string{"The tid field is required."}
		}
		if name == "" {
			errs["name"] = []string{"The name field is required."}
		}
		return validationFailed(ctx, errs)
	}

	options := service.NewOption(conn, dialect)
	textures := repo.NewTextureRepository(conn, dialect)
	closet := repo.NewClosetRepository(conn, dialect)
	users := repo.NewUserRepository(conn, dialect)

	cost := options.Int("score_per_closet_item", 0)
	if user.Score < cost {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.closet.add.lack-score"),
		})
	}

	texture, err := textures.FindByTid(tid)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if texture == nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.closet.add.not-found"),
		})
	}
	if !texture.Public && texture.Uploader != user.UID && !user.IsAdmin() {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "skinlib.show.private"),
		})
	}
	if exists, _ := closet.InCloset(user.UID, tid); exists {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.closet.add.repeated"),
		})
	}

	if err := closet.Add(user.UID, tid, name); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	_ = users.UpdateScore(user.UID, user.Score-cost)
	_ = textures.AddLikes(tid, 1)
	if texture.Uploader != user.UID {
		if uploader, _ := users.FindByID(texture.Uploader); uploader != nil {
			award := options.Int("score_award_per_like", 0)
			_ = users.UpdateScore(uploader.UID, uploader.Score+award)
		}
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "user.closet.add.success", "name", name),
	})
}

// Rename 处理 PUT /api/closet/{tid}、/user/closet/{tid}。
func (r *ClosetController) Rename(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	tid := ctx.Request().RouteInt("tid")
	name := ctx.Request().Input("name")
	if name == "" {
		return validationFailed(ctx, map[string][]string{
			"name": {"The name field is required."},
		})
	}

	closet := repo.NewClosetRepository(conn, dialect)
	item, err := closet.FindItem(user.UID, tid)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if item == nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.closet.remove.non-existent"),
		})
	}
	if err := closet.RenameItem(user.UID, tid, name); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "user.closet.rename.success", "name", name),
	})
}

// Remove 处理 DELETE /api/closet/{tid}、/user/closet/{tid}。
func (r *ClosetController) Remove(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	tid := ctx.Request().RouteInt("tid")
	options := service.NewOption(conn, dialect)
	closet := repo.NewClosetRepository(conn, dialect)
	textures := repo.NewTextureRepository(conn, dialect)
	users := repo.NewUserRepository(conn, dialect)

	item, err := closet.FindItem(user.UID, tid)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if item == nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.closet.remove.non-existent"),
		})
	}

	if err := closet.Remove(user.UID, tid); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if options.Bool("return_score", true) {
		_ = users.UpdateScore(user.UID, user.Score+options.Int("score_per_closet_item", 0))
	}
	_ = textures.AddLikes(tid, -1)
	if item.Texture.Uploader != user.UID {
		if uploader, _ := users.FindByID(item.Texture.Uploader); uploader != nil {
			award := options.Int("score_award_per_like", 0)
			_ = users.UpdateScore(uploader.UID, uploader.Score-award)
		}
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "user.closet.remove.success"),
	})
}

func pageURL(path string, page int) string {
	return fmt.Sprintf("%s?page=%d", path, page)
}

func nextPageURL(path string, page, last int) any {
	if page < last {
		return pageURL(path, page+1)
	}
	return nil
}

func prevPageURL(path string, page int) any {
	if page > 1 {
		return pageURL(path, page-1)
	}
	return nil
}

func paginationLinks(path string, page, last int) []goravelhttp.Json {
	links := make([]goravelhttp.Json, 0, last+2)
	links = append(links, goravelhttp.Json{"url": prevPageURL(path, page),
		"label": "&laquo; Previous", "active": false})
	for i := 1; i <= last; i++ {
		links = append(links, goravelhttp.Json{
			"url": pageURL(path, i), "label": fmt.Sprint(i), "active": i == page,
		})
	}
	links = append(links, goravelhttp.Json{"url": nextPageURL(path, page, last),
		"label": "Next &raquo;", "active": false})
	return links
}
