package controllers

import (
	"encoding/json"
	"html"
	nethttp "net/http"
	"regexp"
	"strings"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/php"
	"neoskin/internal/repo"
)

type NotificationController struct{}

func NewNotificationController() *NotificationController {
	return &NotificationController{}
}

// Inbox 处理 GET /user/inbox、/api/user/inbox。
func (r *NotificationController) Inbox(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}
	page := ctx.Request().InputInt("page", 1)
	if page < 1 {
		page = 1
	}
	perPage := ctx.Request().InputInt("per_page", 10)
	if perPage <= 0 {
		perPage = 10
	}
	if perPage > 100 {
		perPage = 100
	}
	notifications := repo.NewNotificationRepository(conn, dialect)
	total, _ := notifications.CountByUser(user.UID)
	unread, _ := notifications.CountUnreadByUser(user.UID)
	rows, err := notifications.PageByUser(user.UID, perPage, (page-1)*perPage)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	data := make([]goravelhttp.Json, 0, len(rows))
	for _, row := range rows {
		title, _ := row.Data["title"].(string)
		content, _ := row.Data["content"].(string)
		format, _ := row.Data["format"].(string)
		data = append(data, goravelhttp.Json{
			"id": row.ID, "title": title, "preview": previewText(content, 120),
			"format": format, "read_at": nullIfEmpty(row.ReadAt),
			"created_at": row.CreatedAt, "communication_id": jsonInt(row.Data["communication_id"]),
		})
	}
	payload := paginatorJSON(ctx, data, total, page, perPage)
	payload["unread"] = unread
	return ctx.Response().Json(nethttp.StatusOK, payload)
}

// Message 处理 GET /user/inbox/{id}、/api/user/inbox/{id}：返回详情并标记已读。
func (r *NotificationController) Message(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
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
		row.ReadAt = php.Now()
	}
	title, _ := row.Data["title"].(string)
	content, _ := row.Data["content"].(string)
	format, _ := row.Data["format"].(string)
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": "",
		"data": goravelhttp.Json{
			"id": row.ID, "title": title, "content": content, "format": format,
			"read_at": nullIfEmpty(row.ReadAt), "created_at": row.CreatedAt,
			"communication_id": jsonInt(row.Data["communication_id"]),
		},
	})
}

// Read 处理 POST /user/notifications/{id}、/api/user/notifications/{id}。
func (r *NotificationController) Read(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
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
	_ = notifications.MarkRead(user.UID, id)
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"title": row.Data["title"], "content": row.Data["content"], "time": row.CreatedAt,
	})
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// Unread 处理 GET /api/user/notifications（PHP 返回未读列表）。
func (r *NotificationController) Unread(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}
	notifications := repo.NewNotificationRepository(conn, dialect)
	rows, err := notifications.PageByUser(user.UID, 100, 0)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	data := make([]goravelhttp.Json, 0, len(rows))
	for _, row := range rows {
		if row.ReadAt != "" {
			continue
		}
		data = append(data, goravelhttp.Json{"id": row.ID, "title": row.Data["title"]})
	}
	return ctx.Response().Json(nethttp.StatusOK, data)
}

// Send 处理 POST /admin/notifications/send、/api/admin/notifications。
func (r *NotificationController) Send(ctx goravelhttp.Context) goravelhttp.Response {
	admin, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	receiver := ctx.Request().Input("receiver")
	title := ctx.Request().Input("title")
	content := ctx.Request().Input("content")
	errs := map[string][]string{}
	switch receiver {
	case "":
		errs["receiver"] = []string{vRequired(ctx, "receiver")}
	case "all", "normal", "uid", "email":
	default:
		errs["receiver"] = []string{vIn(ctx, "receiver")}
	}
	// PHP：'title' => required|max:20。
	if title == "" {
		errs["title"] = []string{vRequired(ctx, "title")}
	} else if len([]rune(title)) > 20 {
		errs["title"] = []string{vMaxString(ctx, "title", "20")}
	}
	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}

	uid := ctx.Request().InputInt("uid")
	email := ctx.Request().Input("email")
	audience := map[string]any{"receiver": receiver}
	if uid > 0 {
		audience["uid"] = uid
	}
	if email != "" {
		audience["email"] = email
	}
	audienceJSON, _ := json.Marshal(audience)
	now := php.Now()
	communications := repo.NewCommunicationRepository(conn, dialect)
	row := repo.CommunicationRow{
		AuthorID: admin.UID, Title: title, Kind: repo.CommunicationMessage,
		Status: repo.CommunicationPublished, Editor: "source", Source: content,
		Audience: string(audienceJSON), DeliveryToken: repo.RandomHex(32),
		PublishedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	id, err := communications.Insert(row)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	row.ID = id

	users := repo.NewUserRepository(conn, dialect)
	notifications := repo.NewNotificationRepository(conn, dialect)
	ids, err := users.IdsForAudience(receiver, uid, email)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	payload := map[string]any{
		"title": title, "content": renderCommunication(content, ""),
		"format": "html", "communication_id": id, "delivery_token": row.DeliveryToken,
	}
	for _, notifiable := range ids {
		_ = notifications.Insert(notifiable, payload, now)
	}

	ctx.Request().Session().Flash("sentResult", t(ctx, "admin.notifications.send.success"))
	ctx.Response().Header("Location", "/admin")
	return ctx.Response().String(nethttp.StatusFound, "")
}

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

// previewText 把富文本压缩成纯文本预览（与契约「最多 120 字」一致）。
func previewText(content string, limit int) string {
	if content == "" {
		return ""
	}
	plain := html.UnescapeString(htmlTagPattern.ReplaceAllString(content, " "))
	plain = strings.Join(strings.Fields(plain), " ")
	runes := []rune(plain)
	if limit > 0 && len(runes) > limit {
		return string(runes[:limit])
	}
	return plain
}

// jsonInt 把 JSON 解码出的数值统一成 int（缺失时返回 nil）。
func jsonInt(value any) any {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
	}
	return nil
}
