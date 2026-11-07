package controllers

import (
	"encoding/json"
	nethttp "net/http"
	"strings"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/repo"
)

type TranslationsController struct{}

func NewTranslationsController() *TranslationsController {
	return &TranslationsController{}
}

type languageLineJSON struct {
	ID        int            `json:"id"`
	Group     string         `json:"group"`
	Key       string         `json:"key"`
	Text      map[string]any `json:"text"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

// List 处理 GET /admin/i18n/list。
func (r *TranslationsController) List(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	page := ctx.Request().InputInt("page", 1)
	if page < 1 {
		page = 1
	}
	const perPage = 10
	lines := repo.NewLanguageLineRepository(conn, dialect)
	total, err := lines.Count()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	rows, err := lines.Page(perPage, (page-1)*perPage)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	data := make([]languageLineJSON, 0, len(rows))
	for _, line := range rows {
		data = append(data, languageLineJSON{
			ID: line.ID, Group: line.Group, Key: line.Key,
			Text: parseTranslations(line.Text), CreatedAt: line.CreatedAt, UpdatedAt: line.UpdatedAt,
		})
	}
	return ctx.Response().Json(nethttp.StatusOK, paginatorJSON(ctx, data, total, page, perPage))
}

// Create 处理 POST /admin/i18n（成功后 302 到 /admin/i18n）。
func (r *TranslationsController) Create(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	group := ctx.Request().Input("group")
	key := ctx.Request().Input("key")
	text := ctx.Request().Input("text")
	errs := map[string][]string{}
	if group == "" {
		errs["group"] = []string{vRequired(ctx, "group")}
	}
	if key == "" {
		errs["key"] = []string{vRequired(ctx, "key")}
	}
	if text == "" {
		errs["text"] = []string{vRequired(ctx, "text")}
	}
	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}
	locale := ctx.Request().Input("lang", "zh_CN")
	if err := repo.NewLanguageLineRepository(conn, dialect).
		Create(group, key, locale, text); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	// 共享前端以 JSON 调用时不再 302（浏览器会跟随跳转并访问旧页面）。
	if strings.Contains(strings.ToLower(ctx.Request().Header("Accept")), "application/json") {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 0, "message": t(ctx, "admin.i18n.added"),
		})
	}
	ctx.Response().Header("Location", "/admin/i18n")
	return ctx.Response().String(nethttp.StatusFound, "")
}

// Update 处理 PUT /admin/i18n/{line}。
func (r *TranslationsController) Update(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	id := ctx.Request().RouteInt("line")
	text := ctx.Request().Input("text")
	if text == "" {
		return validationFailed(ctx, map[string][]string{"text": {vRequired(ctx, "text")}})
	}
	locale := ctx.Request().Input("lang", "zh_CN")
	lines := repo.NewLanguageLineRepository(conn, dialect)
	line, err := lines.Find(id)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if line == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	if err := lines.UpdateText(id, locale, text); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": t(ctx, "admin.i18n.updated")})
}

// Delete 处理 DELETE /admin/i18n/{line}。
func (r *TranslationsController) Delete(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	id := ctx.Request().RouteInt("line")
	lines := repo.NewLanguageLineRepository(conn, dialect)
	if line, _ := lines.Find(id); line == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	if err := lines.Delete(id); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": t(ctx, "admin.i18n.deleted")})
}

func parseTranslations(raw string) map[string]any {
	result := map[string]any{}
	if raw == "" {
		return result
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		result["zh_CN"] = raw
	}
	return result
}
