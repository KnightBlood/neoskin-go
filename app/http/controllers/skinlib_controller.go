package controllers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"image/png"
	nethttp "net/http"
	"os"
	"strings"

	goravelhttp "github.com/goravel/framework/contracts/http"

	localeMiddleware "neoskin/app/http/middleware"
	"neoskin/internal/auth"
	"neoskin/internal/db"
	"neoskin/internal/i18n"
	"neoskin/internal/model"
	"neoskin/internal/php"
	"neoskin/internal/plugin"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
	"neoskin/internal/storage"
	"neoskin/internal/view"
)

type SkinlibController struct{}

func NewSkinlibController() *SkinlibController {
	return &SkinlibController{}
}

type textureJSON struct {
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
}

func toTextureJSON(texture model.Texture) textureJSON {
	return textureJSON{
		TID: texture.Tid, Name: texture.Name, Type: texture.Type, Hash: texture.Hash,
		Size: texture.Size, Uploader: texture.Uploader, Public: texture.Public,
		UploadAt: texture.UploadAt, Likes: texture.Likes, Model: texture.TextureModel(),
	}
}

type libraryRowJSON struct {
	TID      int    `json:"tid"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Uploader int    `json:"uploader"`
	Public   bool   `json:"public"`
	Likes    int    `json:"likes"`
	Nickname string `json:"nickname"`
}

// Library 处理 GET /skinlib/list。
func (r *SkinlibController) Library(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	user, _, _, _ := currentUser(ctx)

	filter := ctx.Request().Input("filter", "skin")
	keyword := ctx.Request().Input("keyword")
	sortBy := ctx.Request().Input("sort", "time")
	uploader := ctx.Request().InputInt("uploader")
	page := ctx.Request().InputInt("page", 1)
	if page < 1 {
		page = 1
	}
	perPage := 20
	viewerUID := 0
	admin := false
	if user != nil {
		viewerUID = user.UID
		admin = user.IsAdmin()
	}

	textures := repo.NewTextureRepository(conn, dialect)
	total, err := textures.LibraryCount(filter, keyword, uploader, viewerUID, admin)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	lastPage := (total + perPage - 1) / perPage
	if lastPage < 1 {
		lastPage = 1
	}
	offset := (page - 1) * perPage
	rows, err := textures.LibraryPage(filter, keyword, sortBy, uploader, viewerUID, admin,
		perPage, offset)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	data := make([]libraryRowJSON, 0, len(rows))
	for _, row := range rows {
		data = append(data, libraryRowJSON{
			TID: row.TID, Name: row.Name, Type: row.Type, Uploader: row.Uploader,
			Public: row.Public, Likes: row.Likes, Nickname: row.Nickname,
		})
	}

	path := "http://" + ctx.Request().Host() + ctx.Request().Path()
	var from, to any
	if len(data) > 0 {
		from = offset + 1
		to = offset + len(data)
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"current_page":   page,
		"data":           data,
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

// Info 处理 GET /skinlib/info/{texture}、/texture/{texture}。
func (r *SkinlibController) Info(ctx goravelhttp.Context) goravelhttp.Response {
	texture, user, conn, dialect, resp := r.textureForView(ctx)
	if resp != nil {
		return resp
	}
	if !texture.Public && (user == nil ||
		(user.UID != texture.Uploader && !user.IsAdmin())) {
		return ctx.Response().String(nethttp.StatusForbidden, "This texture is private.")
	}
	_ = conn
	_ = dialect

	return ctx.Response().Json(nethttp.StatusOK, toTextureJSON(*texture))
}

// Show 处理 GET /skinlib/show/{texture}（当前返回详情 JSON，模板后替换）。
func (r *SkinlibController) Show(ctx goravelhttp.Context) goravelhttp.Response {
	return r.Info(ctx)
}

// skinlibShell 构建皮肤库页面公共数据（顶部导航 + 页脚）。
func (r *SkinlibController) skinlibShell(ctx goravelhttp.Context, titleKey string,
	extra map[string]any) (*view.SkinlibData, *i18n.Translator, goravelhttp.Response) {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return nil, nil, ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	options := service.NewOption(conn, dialect)
	locale := localeMiddleware.LocaleOf(ctx)
	translator := i18n.New(locale, repo.NewLanguageLineRepository(conn, dialect).Overrides(locale))
	siteName := localizedOption(options, "site_name", locale, "Blessing Skin")
	navbarColor := options.Get("navbar_color")
	if navbarColor == "" {
		navbarColor = "cyan"
	}
	user, _ := auth.CurrentUser(ctx, repo.NewUserRepository(conn, dialect))

	extraJSON, _ := json.Marshal(extra)
	data := &view.SkinlibData{
		BaseData:        buildBaseData(ctx, options, locale, siteName),
		Title:           translator.Trans(titleKey),
		NavbarColor:     navbarColor,
		ColorMode:       view.ColorMode(navbarColor),
		Languages:       languageOptions(ctx, locale),
		CurrentLanguage: i18n.Name(locale),
		ExtraJSON:       template.JS(extraJSON),
	}
	if user != nil {
		data.LoggedIn = true
		data.Nickname = user.Nickname
		if data.Nickname == "" {
			data.Nickname = user.Email
		}
		data.IsAdmin = user.IsAdmin()
		data.AvatarURL = fmt.Sprintf("/avatar/%d?size=36", user.Avatar)
		data.AvatarPNGURL = fmt.Sprintf("/avatar/%d?size=36&png=1", user.Avatar)
	}
	return data, translator, nil
}

func renderSkinlibPage(ctx goravelhttp.Context, name string, data *view.SkinlibData,
	translator *i18n.Translator) goravelhttp.Response {
	body, err := view.Render(name, data, translator)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Data(nethttp.StatusOK, "text/html; charset=utf-8", body)
}

// IndexPage 处理 GET /skinlib（浏览页，数据由前端调用 /skinlib/list）。
func (r *SkinlibController) IndexPage(ctx goravelhttp.Context) goravelhttp.Response {
	user, _, _, _ := currentUser(ctx)
	uid := 0
	if user != nil {
		uid = user.UID
	}
	data, translator, resp := r.skinlibShell(ctx, "general.skinlib",
		map[string]any{"currentUid": uid})
	if resp != nil {
		return resp
	}
	return renderSkinlibPage(ctx, "skinlib_index", data, translator)
}

// ShowPage 处理 GET /skinlib/show/{texture}。
func (r *SkinlibController) ShowPage(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	textures := repo.NewTextureRepository(conn, dialect)
	texture, err := textures.FindByTid(ctx.Request().RouteInt("texture"))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if texture == nil || !storage.NewLocal(runtime.StorageRoot()).Exists(texture.Hash) {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}

	user, _, _, _ := currentUser(ctx)
	uid, admin := 0, false
	if user != nil {
		uid, admin = user.UID, user.IsAdmin()
	}
	options := service.NewOption(conn, dialect)
	data, translator, resp := r.skinlibShell(ctx, "skinlib.show.title", map[string]any{
		"download":   options.Bool("allow_downloading_texture", true),
		"currentUid": uid,
		"admin":      admin,
	})
	if resp != nil {
		return resp
	}
	data.Title = texture.Name
	return renderSkinlibPage(ctx, "skinlib_show", data, translator)
}

// UploadPage 处理 GET /skinlib/upload（需要登录且邮箱已验证）。
func (r *SkinlibController) UploadPage(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Redirect(nethttp.StatusFound, "/auth/login")
	}
	options := service.NewOption(conn, dialect)
	if options.Bool("require_verification", false) && !user.Verified {
		return ctx.Response().String(nethttp.StatusForbidden, "You must verify your email first.")
	}

	locale := localeMiddleware.LocaleOf(ctx)
	translator := i18n.New(locale, repo.NewLanguageLineRepository(conn, dialect).Overrides(locale))
	rule := translator.Trans("skinlib.upload.name-rule")
	if regexpValue := options.Get("texture_name_regexp"); regexpValue != "" {
		rule = translator.Trans("skinlib.upload.name-rule-regexp",
			map[string]string{"regexp": regexpValue})
	}
	policy := localizedOption(options, "content_policy", locale, "")
	data, translator, resp := r.skinlibShell(ctx, "skinlib.upload.title", map[string]any{
		"rule": rule,
		"privacyNotice": translator.Trans("skinlib.upload.private-score-notice",
			map[string]string{"score": options.Get("private_score_per_storage")}),
		"score":          user.Score,
		"scorePublic":    options.Int("score_per_storage", 0),
		"scorePrivate":   options.Int("private_score_per_storage", 0),
		"closetItemCost": options.Int("score_per_closet_item", 0),
		"award":          options.Int("score_award_per_texture", 0),
		"contentPolicy":  renderCommunication(policy, ""),
	})
	if resp != nil {
		return resp
	}
	return renderSkinlibPage(ctx, "skinlib_upload", data, translator)
} // Delete 处理 DELETE /texture/{texture}。
func (r *SkinlibController) Delete(ctx goravelhttp.Context) goravelhttp.Response {
	texture, user, conn, dialect, resp := r.textureForManage(ctx)
	if resp != nil {
		return resp
	}
	_ = user

	textures := repo.NewTextureRepository(conn, dialect)
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventTextureDeleting, Payload: texture.Tid})
	count, _ := textures.CountByHash(texture.Hash)
	if count <= 1 {
		_ = storage.NewLocal(runtime.StorageRoot()).Delete(texture.Hash)
	}
	if err := textures.DeleteByTid(texture.Tid); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "skinlib.delete.success"),
	})
}

// Privacy 处理 PUT /texture/{texture}/privacy。
func (r *SkinlibController) Privacy(ctx goravelhttp.Context) goravelhttp.Response {
	texture, _, conn, dialect, resp := r.textureForManage(ctx)
	if resp != nil {
		return resp
	}
	options := service.NewOption(conn, dialect)
	users := repo.NewUserRepository(conn, dialect)
	textures := repo.NewTextureRepository(conn, dialect)

	uploader, err := users.FindByID(texture.Uploader)
	if err != nil || uploader == nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, "uploader not found")
	}
	publicCost := options.Int("score_per_storage", 0)
	privateCost := options.Int("private_score_per_storage", 10)
	diff := texture.Size * (privateCost - publicCost)
	if texture.Public {
		diff = -diff
		if options.Bool("take_back_scores_after_deletion", true) {
			diff -= options.Int("score_award_per_texture", 0)
		}
	}
	if uploader.Score+diff < 0 {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "skinlib.upload.lack-score"),
		})
	}

	if !texture.Public {
		if duplicated, _ := textures.FindByHash(texture.Hash); duplicated != nil && duplicated.Public {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 2, "message": t(ctx, "skinlib.upload.repeated"),
				"data": goravelhttp.Json{"tid": duplicated.Tid},
			})
		}
	}

	privacyLabel := t(ctx, "general.private")
	if !texture.Public {
		privacyLabel = t(ctx, "general.public")
	}
	if err := users.UpdateScore(uploader.UID, uploader.Score+diff); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if err := textures.UpdatePublic(texture.Tid, !texture.Public); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "skinlib.privacy.success", "privacy", privacyLabel),
	})
}

// Rename 处理 PUT /texture/{texture}/name。
func (r *SkinlibController) Rename(ctx goravelhttp.Context) goravelhttp.Response {
	texture, _, conn, dialect, resp := r.textureForManage(ctx)
	if resp != nil {
		return resp
	}
	name := ctx.Request().Input("name")
	if name == "" {
		return validationFailed(ctx, map[string][]string{"name": {"The name field is required."}})
	}
	if err := repo.NewTextureRepository(conn, dialect).UpdateName(texture.Tid, name); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "skinlib.rename.success", "name", name),
	})
}

// Type 处理 PUT /texture/{texture}/type。
func (r *SkinlibController) Type(ctx goravelhttp.Context) goravelhttp.Response {
	texture, _, conn, dialect, resp := r.textureForManage(ctx)
	if resp != nil {
		return resp
	}
	textureType := ctx.Request().Input("type")
	if textureType != "steve" && textureType != "alex" && textureType != "cape" {
		return validationFailed(ctx, map[string][]string{
			"type": {"The selected type is invalid."},
		})
	}
	if err := repo.NewTextureRepository(conn, dialect).UpdateType(texture.Tid, textureType); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "skinlib.model.success", "model", textureType),
	})
}

func (r *SkinlibController) textureForView(ctx goravelhttp.Context) (*model.Texture,
	*model.User, *sql.DB, *db.Dialect, goravelhttp.Response) {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return nil, nil, nil, nil, ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	tid := ctx.Request().RouteInt("texture")
	texture, err := repo.NewTextureRepository(conn, dialect).FindByTid(tid)
	if err != nil {
		return nil, nil, nil, nil, ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if texture == nil {
		return nil, nil, nil, nil, ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	return texture, user, conn, dialect, nil
}

func (r *SkinlibController) textureForManage(ctx goravelhttp.Context) (*model.Texture,
	*model.User, *sql.DB, *db.Dialect, goravelhttp.Response) {
	texture, user, conn, dialect, resp := r.textureForView(ctx)
	if resp != nil {
		return nil, nil, nil, nil, resp
	}
	if user == nil {
		return nil, nil, nil, nil, ctx.Response().Json(nethttp.StatusUnauthorized,
			goravelhttp.Json{"message": "Unauthenticated."})
	}
	if user.UID != texture.Uploader && !user.IsAdmin() {
		return nil, nil, nil, nil, ctx.Response().Json(nethttp.StatusForbidden,
			goravelhttp.Json{"code": 1, "message": t(ctx, "skinlib.no-permission")})
	}
	return texture, user, conn, dialect, nil
}

// Upload 处理 POST /texture（multipart：file/name/type/public）。
func (r *SkinlibController) Upload(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	name := strings.TrimSpace(ctx.Request().Input("name"))
	textureType := ctx.Request().Input("type")
	publicValue := ctx.Request().Input("public")
	errs := map[string][]string{}
	if name == "" {
		errs["name"] = []string{"The name field is required."}
	}
	if textureType != "steve" && textureType != "alex" && textureType != "cape" {
		errs["type"] = []string{"The selected type is invalid."}
	}

	upload, fileErr := ctx.Request().File("file")
	if fileErr != nil || upload == nil {
		errs["file"] = []string{"The file field is required."}
	}
	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}

	data, err := os.ReadFile(upload.File())
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	options := service.NewOption(conn, dialect)
	maxSize := options.Int("max_upload_file_size", 1024) * 1024
	if maxSize > 0 && len(data) > maxSize {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": fmt.Sprintf("File is too large (max %d KB).", maxSize/1024),
		})
	}

	image, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return validationFailed(ctx, map[string][]string{"file": {"The file must be a PNG image."}})
	}
	width := image.Bounds().Dx()
	height := image.Bounds().Dy()
	maxWidth := options.Int("max_texture_width", 8192)
	if width > maxWidth {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code":    1,
			"message": t(ctx, "skinlib.upload.too-wide", "width", fmt.Sprint(width), "maxWidth", fmt.Sprint(maxWidth)),
		})
	}
	if width%64 != 0 || height%32 != 0 {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "skinlib.upload.invalid-size", "type", textureType, "width", fmt.Sprint(width), "height", fmt.Sprint(height)),
		})
	}
	ratio := float64(width) / float64(height)
	if textureType == "cape" {
		if ratio != 2 {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": t(ctx, "skinlib.upload.invalid-size", "type", textureType, "width", fmt.Sprint(width), "height", fmt.Sprint(height)),
			})
		}
	} else if (ratio != 2 && ratio != 1) || (textureType == "alex" && ratio == 2) {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "skinlib.upload.invalid-size", "type", textureType, "width", fmt.Sprint(width), "height", fmt.Sprint(height)),
		})
	}

	var sanitized bytes.Buffer
	if err := png.Encode(&sanitized, image); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	body := sanitized.Bytes()
	hash := php.Sha256Hex(body)

	textures := repo.NewTextureRepository(conn, dialect)
	if duplicated, _ := textures.FindPublicOrMine(hash, user.UID); duplicated != nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 2, "message": t(ctx, "skinlib.upload.repeated"),
			"data": goravelhttp.Json{"tid": duplicated.Tid},
		})
	}

	isPublic := publicValue == "1" || strings.EqualFold(publicValue, "true") ||
		strings.EqualFold(publicValue, "on")
	fileSize := (len(body) + 1023) / 1024
	cost := fileSize
	if isPublic {
		cost *= options.Int("score_per_storage", 0)
	} else {
		cost *= options.Int("private_score_per_storage", 10)
	}
	cost += options.Int("score_per_closet_item", 0)
	cost -= options.Int("score_award_per_texture", 0)
	if user.Score < cost {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "skinlib.upload.lack-score"),
		})
	}

	tid, err := textures.Insert(name, textureType, hash, fileSize, user.UID, isPublic, php.Now())
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	store := storage.NewLocal(runtime.StorageRoot())
	if !store.Exists(hash) {
		if err := store.Write(hash, body); err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
	}
	if err := repo.NewClosetRepository(conn, dialect).Add(user.UID, tid, name); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if err := repo.NewUserRepository(conn, dialect).UpdateScore(user.UID, user.Score-cost); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "skinlib.upload.success", "name", name),
		"data": goravelhttp.Json{"tid": tid},
	})
}
