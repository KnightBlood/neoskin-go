package controllers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	nethttp "net/http"
	"strconv"
	"strings"

	goravelhttp "github.com/goravel/framework/contracts/http"

	localeMiddleware "neoskin/app/http/middleware"
	"neoskin/internal/db"
	"neoskin/internal/php"
	"neoskin/internal/repo"
	"neoskin/internal/view"
)

type CommunicationsController struct{}

func NewCommunicationsController() *CommunicationsController {
	return &CommunicationsController{}
}

// Index 处理 GET /admin/communications（当前返回消息分页 JSON）。
func (r *CommunicationsController) Index(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	page := ctx.Request().InputInt("page", 1)
	if page < 1 {
		page = 1
	}
	const perPage = 15
	communications := repo.NewCommunicationRepository(conn, dialect)
	total, err := communications.CountMessages()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	rows, err := communications.PageMessages(perPage, (page-1)*perPage)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	data := make([]goravelhttp.Json, 0, len(rows))
	for _, row := range rows {
		data = append(data, goravelhttp.Json{
			"id": row.ID, "title": row.Title, "kind": row.Kind, "status": row.Status,
			"read_count": 0, "delivery_count": 0, "updated_at": row.UpdatedAt,
		})
	}
	return ctx.Response().Json(nethttp.StatusOK, paginatorJSON(ctx, data, total, page, perPage))
}

// CommunicationsPage 处理 GET /admin/communications（历史列表页）。
func (r *CommunicationsController) CommunicationsPage(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdminPage(ctx)
	if resp != nil {
		return resp
	}
	page := ctx.Request().InputInt("page", 1)
	if page < 1 {
		page = 1
	}
	const perPage = 15
	communications := repo.NewCommunicationRepository(conn, dialect)
	total, err := communications.CountMessages()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	rows, err := communications.PageMessages(perPage, (page-1)*perPage)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	data, translator, resp := adminPageData(ctx, "general.admin-panel", nil)
	if resp != nil {
		return resp
	}
	data.Title = "通知与公告"
	kindLabels := map[string]string{
		repo.CommunicationAnnouncement: "公告",
		repo.CommunicationMessage:      "站内信",
		repo.CommunicationBoth:         "公告与站内信",
	}
	statusLabels := map[string]string{
		repo.CommunicationDraft:     "草稿",
		repo.CommunicationPublished: "已发布",
		repo.CommunicationWithdrawn: "已撤回",
	}
	items := make([]view.CommunicationItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, view.CommunicationItem{
			ID: row.ID, Title: row.Title,
			KindText: kindLabels[row.Kind], StatusText: statusLabels[row.Status],
			ReadCount:   communications.ReadCount(row.ID),
			Delivery:    communications.DeliveryCount(row.ID),
			UpdatedAt:   row.UpdatedAt,
			EditURL:     "/admin/communications?edit=" + fmt.Sprint(row.ID),
			WithdrawURL: "/admin/communications/" + fmt.Sprint(row.ID) + "/withdraw",
			CanWithdraw: row.Status == repo.CommunicationPublished,
		})
	}
	data.Communications = items
	data.CommunicationNote = pullFlash(ctx, "communication_notice")
	data.CommunicationPage = pagination("/admin/communications", page, perPage, total)

	// 编辑器：?new=1 / ?announcement=1 / ?edit=<id>
	announcementParam := ctx.Request().Query("announcement") != ""
	newParam := ctx.Request().Query("new") != ""
	editRaw := ctx.Request().Query("edit")
	if announcementParam || newParam || editRaw != "" {
		data.EditorOpen = true
		data.EditorMode = "blocks"
		data.EditorKind = repo.CommunicationMessage
		data.EditorReceiver = "all"
		var row *repo.CommunicationRow
		if editRaw != "" {
			if id, convErr := strconv.Atoi(editRaw); convErr == nil {
				row, _ = communications.Find(id)
			}
		} else if announcementParam {
			row, _ = communications.FindLatestAnnouncement(localeMiddleware.LocaleOf(ctx))
			if row != nil && row.Kind == repo.CommunicationBoth {
				row = nil
			}
		}
		if row != nil {
			data.EditingID = row.ID
			data.EditingStatus = row.Status
			data.EditingKind = row.Kind
			data.EditorTitle = row.Title
			data.EditorKind = row.Kind
			data.EditorLocale = row.Locale
			data.EditorSource = row.Source
			if row.Editor != "" {
				data.EditorMode = row.Editor
			}
			audience := map[string]any{}
			if row.Audience != "" {
				_ = json.Unmarshal([]byte(row.Audience), &audience)
			}
			if receiver, ok := audience["receiver"].(string); ok && receiver != "" {
				data.EditorReceiver = receiver
			}
			data.EditorUID = fmt.Sprint(audience["uid"])
			data.EditorEmail = fmt.Sprint(audience["email"])
			if data.EditorUID == "<nil>" {
				data.EditorUID = ""
			}
			if data.EditorEmail == "<nil>" {
				data.EditorEmail = ""
			}
		} else if announcementParam {
			data.EditorTitle = "站点公告"
			data.EditorKind = repo.CommunicationAnnouncement
		}

		blocks := any([]any{})
		if row != nil && row.Blocks != "" {
			var parsed []any
			if json.Unmarshal([]byte(row.Blocks), &parsed) == nil {
				blocks = parsed
			}
		}
		initial, _ := json.Marshal(map[string]any{
			"blocks": blocks, "editor": data.EditorMode,
		})
		data.EditorBlocksJSON = template.JS(initial)
	}
	return renderAdminPage(ctx, "admin_communications", data, translator)
}

// Store 处理 POST /admin/communications。
func (r *CommunicationsController) Store(ctx goravelhttp.Context) goravelhttp.Response {
	admin, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	row, errs := buildCommunication(ctx)
	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}
	communications := repo.NewCommunicationRepository(conn, dialect)
	if row.Kind == repo.CommunicationAnnouncement {
		existing, _ := communications.FindLatestAnnouncement(row.Locale)
		if existing != nil {
			row.ID = existing.ID
			row.CreatedAt = existing.CreatedAt
			if err := communications.Update(row); err != nil {
				return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
			}
			return redirectCommunication(ctx, row, true)
		}
	}

	row.AuthorID = admin.UID
	row.CreatedAt = php.Now()
	id, err := communications.Insert(row)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	row.ID = id
	deliver(conn, dialect, row)
	return redirectCommunication(ctx, row, row.Kind == repo.CommunicationAnnouncement)
}

// Update 处理 PUT /admin/communications/{communication}。
func (r *CommunicationsController) Update(ctx goravelhttp.Context) goravelhttp.Response {
	admin, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	id := ctx.Request().RouteInt("communication")
	communications := repo.NewCommunicationRepository(conn, dialect)
	existing, err := communications.Find(id)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if existing == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	row, errs := buildCommunication(ctx)
	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}
	row.ID = id
	row.AuthorID = admin.UID
	row.CreatedAt = existing.CreatedAt
	row.DeliveryToken = existing.DeliveryToken
	if existing.Status == repo.CommunicationPublished {
		row.Audience = existing.Audience
	}
	if err := communications.Update(row); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	deliver(conn, dialect, row)
	return redirectCommunication(ctx, row, row.Kind == repo.CommunicationAnnouncement)
}

// Withdraw 处理 POST /admin/communications/{communication}/withdraw。
func (r *CommunicationsController) Withdraw(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	id := ctx.Request().RouteInt("communication")
	if err := repo.NewCommunicationRepository(conn, dialect).Withdraw(id); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	ctx.Response().Header("Location", "/admin/communications")
	return ctx.Response().String(nethttp.StatusFound, "")
}

// Preview 处理 POST /admin/communications/preview。
func (r *CommunicationsController) Preview(ctx goravelhttp.Context) goravelhttp.Response {
	_, _, _, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	title := ctx.Request().Input("title", "预览")
	_ = title
	source := ctx.Request().Input("source")
	blocksJSON := ctx.Request().Input("blocks_json")
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"html": renderCommunication(source, blocksJSON),
	})
}

// Import 处理 POST /admin/communications/import。
func (r *CommunicationsController) Import(ctx goravelhttp.Context) goravelhttp.Response {
	_, _, _, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	source := strings.TrimSpace(ctx.Request().Input("source"))
	blocks := []goravelhttp.Json{}
	for _, paragraph := range strings.Split(source, "\n\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph != "" {
			blocks = append(blocks, goravelhttp.Json{"type": "text", "content": paragraph})
		}
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"blocks": blocks})
}

func buildCommunication(ctx goravelhttp.Context) (repo.CommunicationRow, map[string][]string) {
	title := strings.TrimSpace(ctx.Request().Input("title"))
	kind := ctx.Request().Input("kind")
	editor := ctx.Request().Input("editor")
	operation := ctx.Request().Input("operation")
	source := ctx.Request().Input("source")
	blocksJSON := ctx.Request().Input("blocks_json")
	locale := strings.TrimSpace(ctx.Request().Input("locale"))
	receiver := ctx.Request().Input("receiver")

	errs := map[string][]string{}
	if title == "" {
		errs["title"] = []string{vRequired(ctx, "title")}
	}
	if kind != repo.CommunicationAnnouncement && kind != repo.CommunicationMessage &&
		kind != repo.CommunicationBoth {
		errs["kind"] = []string{vIn(ctx, "kind")}
	}
	if editor != "source" && editor != "blocks" {
		errs["editor"] = []string{vIn(ctx, "editor")}
	}
	if operation != "draft" && operation != "publish" {
		errs["operation"] = []string{vIn(ctx, "operation")}
	}
	if receiver == "" {
		receiver = "all"
	}
	if receiver != "all" && receiver != "normal" && receiver != "uid" && receiver != "email" {
		errs["receiver"] = []string{vIn(ctx, "receiver")}
	}

	status := repo.CommunicationDraft
	if operation == "publish" {
		status = repo.CommunicationPublished
	}

	audience := map[string]any{"receiver": receiver}
	if uid := ctx.Request().InputInt("uid"); uid > 0 {
		audience["uid"] = uid
	}
	if email := ctx.Request().Input("email"); email != "" {
		audience["email"] = email
	}
	audienceJSON, _ := json.Marshal(audience)

	row := repo.CommunicationRow{
		Title: title, Kind: kind, Status: status, Locale: locale, Editor: editor,
		Source: source, Blocks: blocksJSON, Audience: string(audienceJSON),
		UpdatedAt: php.Now(),
	}
	if status == repo.CommunicationPublished {
		row.PublishedAt = php.Now()
	}
	if kind != repo.CommunicationAnnouncement && status == repo.CommunicationPublished {
		row.DeliveryToken = repo.RandomHex(32)
	}
	return row, errs
}

func deliver(conn *sql.DB, dialect *db.Dialect, row repo.CommunicationRow) {
	if row.Status != repo.CommunicationPublished ||
		(row.Kind != repo.CommunicationMessage && row.Kind != repo.CommunicationBoth) {
		return
	}

	var audience map[string]any
	_ = json.Unmarshal([]byte(row.Audience), &audience)
	receiver, _ := audience["receiver"].(string)
	uid := 0
	if value, ok := audience["uid"].(float64); ok {
		uid = int(value)
	}
	email, _ := audience["email"].(string)

	users := repo.NewUserRepository(conn, dialect)
	notifications := repo.NewNotificationRepository(conn, dialect)
	ids, err := users.IdsForAudience(receiver, uid, email)
	if err != nil {
		return
	}
	payload := map[string]any{
		"title": row.Title, "content": renderCommunication(row.Source, row.Blocks),
		"format": "html", "communication_id": row.ID, "delivery_token": row.DeliveryToken,
	}
	for _, id := range ids {
		_ = notifications.Insert(id, payload, php.Now())
	}
}

func redirectCommunication(ctx goravelhttp.Context, row repo.CommunicationRow,
	announcement bool) goravelhttp.Response {
	if session := ctx.Request().Session(); session != nil {
		session.Put("communication_notice", "内容已发布。")
		if row.Status == repo.CommunicationDraft {
			session.Put("communication_notice", "草稿已保存。")
		}
	}
	target := "/admin/communications?edit=" + fmt.Sprint(row.ID)
	if announcement {
		target = "/admin/communications?announcement=1"
	}
	ctx.Response().Header("Location", target)
	return ctx.Response().String(nethttp.StatusFound, "")
}

func renderCommunication(source, blocksJSON string) string {
	if strings.TrimSpace(blocksJSON) != "" {
		var blocks []map[string]any
		if json.Unmarshal([]byte(blocksJSON), &blocks) == nil && len(blocks) > 0 {
			var builder strings.Builder
			for _, block := range blocks {
				kind, _ := block["type"].(string)
				content, _ := block["content"].(string)
				title, _ := block["title"].(string)
				switch kind {
				case "heading":
					builder.WriteString("<h3>" + html.EscapeString(title) + "</h3>")
				case "divider":
					builder.WriteString("<hr>")
				case "alert":
					builder.WriteString("<div class=\"alert alert-info\">" +
						html.EscapeString(content) + "</div>")
				default:
					builder.WriteString("<p>" + html.EscapeString(content) + "</p>")
				}
			}
			return builder.String()
		}
	}

	var builder strings.Builder
	for _, paragraph := range strings.Split(source, "\n\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		escaped := html.EscapeString(paragraph)
		escaped = strings.ReplaceAll(escaped, "**", "")
		builder.WriteString("<p>" + strings.ReplaceAll(escaped, "\n", "<br>") + "</p>")
	}
	return builder.String()
}
