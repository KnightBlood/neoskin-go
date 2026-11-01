package controllers

import (
	"database/sql"
	nethttp "net/http"
	"strings"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/db"
	"neoskin/internal/minecraft"
	"neoskin/internal/model"
	"neoskin/internal/plugin"
	"neoskin/internal/repo"
	"neoskin/internal/resources"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
	"neoskin/internal/storage"
)

// TextureController 提供 Minecraft 客户端契约与原始纹理。
type TextureController struct{}

func NewTextureController() *TextureController {
	return &TextureController{}
}

// PlayerJSON 处理 GET /{player}.json 与 GET /csl/{player}.json。
func (r *TextureController) PlayerJSON(ctx goravelhttp.Context) goravelhttp.Response {
	raw := ctx.Request().Route("player")
	if !strings.HasSuffix(raw, ".json") {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	name := strings.TrimSuffix(raw, ".json")

	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	players := repo.NewPlayerRepository(conn, dialect)
	textures := repo.NewTextureRepository(conn, dialect)

	player, err := players.FindDetailByName(name)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if player == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	if player.Permission == model.PermissionBanned {
		return ctx.Response().String(nethttp.StatusForbidden, "Forbidden")
	}
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventPlayerRetrieved, Payload: player.Name})

	var skin *model.Texture
	if player.TidSkin > 0 {
		skin, _ = textures.FindByTid(player.TidSkin)
	}
	var cape *model.Texture
	if player.TidCape > 0 {
		cape, _ = textures.FindByTid(player.TidCape)
	}

	modelName := "default"
	var skinHash any
	if skin != nil {
		modelName = skin.TextureModel()
		skinHash = skin.Hash
	}

	var capeHash any
	if cape != nil {
		capeHash = cape.Hash
	}

	type playerJSON struct {
		Username string         `json:"username"`
		Skins    map[string]any `json:"skins"`
		Cape     any            `json:"cape"`
	}

	return ctx.Response().Json(nethttp.StatusOK, playerJSON{
		Username: player.Name,
		Skins:    map[string]any{modelName: skinHash},
		Cape:     capeHash,
	})
}

// Texture 处理 GET /textures/{hash} 与 GET /csl/textures/{hash}。
func (r *TextureController) Texture(ctx goravelhttp.Context) goravelhttp.Response {
	hash := ctx.Request().Route("hash")
	store := storage.NewLocal(runtime.StorageRoot())
	if !store.Exists(hash) {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}

	data, err := store.Read(hash)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	if modified, err := store.ModTime(hash); err == nil {
		ctx.Response().Header("Last-Modified", modified.UTC().Format(nethttp.TimeFormat))
	}
	return ctx.Response().Data(nethttp.StatusOK, "image/png", data)
}

// Raw 处理 GET /raw/{tid}：按 tid 下载原始纹理。
func (r *TextureController) Raw(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if !service.NewOption(conn, dialect).Bool("allow_downloading_texture", true) {
		return ctx.Response().String(nethttp.StatusForbidden, "Forbidden")
	}

	texture, err := repo.NewTextureRepository(conn, dialect).FindByTid(ctx.Request().RouteInt("tid"))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if texture == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}

	store := storage.NewLocal(runtime.StorageRoot())
	if !store.Exists(texture.Hash) {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	data, err := store.Read(texture.Hash)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if modified, err := store.ModTime(texture.Hash); err == nil {
		ctx.Response().Header("Last-Modified", modified.UTC().Format(nethttp.TimeFormat))
	}
	return ctx.Response().Data(nethttp.StatusOK, "image/png", data)
}

// avatar 共用的头像响应：?size=、?3d；无纹理时返回默认头像。
func avatarResponse(ctx goravelhttp.Context, texture *model.Texture) goravelhttp.Response {
	size := ctx.Request().QueryInt("size", 100)
	if size <= 0 {
		size = 100
	}
	_, threeD := ctx.Request().Queries()["3d"]

	var canvas *minecraft.Canvas
	store := storage.NewLocal(runtime.StorageRoot())
	if texture != nil && store.Exists(texture.Hash) {
		data, err := store.Read(texture.Hash)
		if err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		if threeD {
			canvas, err = minecraft.Render3dAvatar(data, 25)
		} else {
			canvas, err = minecraft.Render2dAvatar(data, 25)
		}
		if err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		if modified, err := store.ModTime(texture.Hash); err == nil {
			ctx.Response().Header("Last-Modified", modified.UTC().Format(nethttp.TimeFormat))
		}
	} else {
		fallback, err := minecraft.CanvasFromPNG(resources.AvatarFallback(threeD))
		if err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		canvas = fallback
	}

	body, err := encodeCanvas(ctx, canvas.Resize(size, size))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Data(nethttp.StatusOK, imageContentType(ctx), body)
}

func textureStore() *storage.Local {
	return storage.NewLocal(runtime.StorageRoot())
}

func (r *TextureController) textureByTid(conn *sql.DB, dialect *db.Dialect, tid int) (*model.Texture, error) {
	if tid <= 0 {
		return nil, nil
	}
	return repo.NewTextureRepository(conn, dialect).FindByTid(tid)
}

// AvatarByTexture 处理 GET /avatar/{tid}。
func (r *TextureController) AvatarByTexture(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	texture, err := r.textureByTid(conn, dialect, ctx.Request().RouteInt("tid"))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if texture != nil && texture.Type != "steve" && texture.Type != "alex" {
		return ctx.Response().String(nethttp.StatusUnprocessableEntity, "")
	}
	return avatarResponse(ctx, texture)
}

// AvatarByHash 处理 GET /avatar/hash/{hash}。
func (r *TextureController) AvatarByHash(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	texture, err := repo.NewTextureRepository(conn, dialect).FindByHash(ctx.Request().Route("hash"))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if texture != nil && texture.Type != "steve" && texture.Type != "alex" {
		return ctx.Response().String(nethttp.StatusUnprocessableEntity, "")
	}
	return avatarResponse(ctx, texture)
}

// AvatarByPlayer 处理 GET /avatar/player/{name}。
func (r *TextureController) AvatarByPlayer(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	player, err := repo.NewPlayerRepository(conn, dialect).
		FindDetailByName(ctx.Request().Route("name"))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if player == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	texture, err := r.textureByTid(conn, dialect, player.TidSkin)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return avatarResponse(ctx, texture)
}

// AvatarByUser 处理 GET /avatar/user/{uid}。
func (r *TextureController) AvatarByUser(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	user, err := repo.NewUserRepository(conn, dialect).FindByID(ctx.Request().RouteInt("uid"))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return avatarResponse(ctx, nil)
	}
	texture, err := r.textureByTid(conn, dialect, user.Avatar)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return avatarResponse(ctx, texture)
}

// Preview 处理 GET /preview/{texture}。
func (r *TextureController) Preview(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	texture, err := r.textureByTid(conn, dialect, ctx.Request().RouteInt("texture"))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if texture == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	return previewResponse(ctx, texture)
}

// PreviewByHash 处理 GET /preview/hash/{hash}。
func (r *TextureController) PreviewByHash(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	texture, err := repo.NewTextureRepository(conn, dialect).FindByHash(ctx.Request().Route("hash"))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if texture == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	return previewResponse(ctx, texture)
}

func previewResponse(ctx goravelhttp.Context, texture *model.Texture) goravelhttp.Response {
	store := textureStore()
	if !store.Exists(texture.Hash) {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	data, err := store.Read(texture.Hash)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	height := ctx.Request().QueryInt("height", 200)
	var canvas *minecraft.Canvas
	if texture.Type == "cape" {
		canvas, err = minecraft.RenderCape(data, height)
	} else {
		canvas, err = minecraft.RenderSkin(data, 12, texture.Type == "alex")
	}
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if modified, err := store.ModTime(texture.Hash); err == nil {
		ctx.Response().Header("Last-Modified", modified.UTC().Format(nethttp.TimeFormat))
	}
	body, err := encodeCanvas(ctx, canvas)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Data(nethttp.StatusOK, imageContentType(ctx), body)
}

// imageContentType 与 PHP 的 ?png 语义一致：默认 webp，带 png 参数时输出 PNG。
func imageContentType(ctx goravelhttp.Context) string {
	if _, ok := ctx.Request().Queries()["png"]; ok {
		return "image/png"
	}
	return "image/webp"
}

func encodeCanvas(ctx goravelhttp.Context, canvas *minecraft.Canvas) ([]byte, error) {
	if _, ok := ctx.Request().Queries()["png"]; ok {
		return canvas.PNG()
	}
	return canvas.WebP()
}
