package controllers

import (
	"encoding/json"
	nethttp "net/http"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/php"
	"neoskin/internal/repo"
)

type NotificationController struct{}

func NewNotificationController() *NotificationController {
	return &NotificationController{}
}

// Inbox 处理 GET /user/inbox、/api/user/notifications。
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
	const perPage = 10
	notifications := repo.NewNotificationRepository(conn, dialect)
	total, _ := notifications.CountByUser(user.UID)
	rows, err := notifications.PageByUser(user.UID, perPage, (page-1)*perPage)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	data := make([]goravelhttp.Json, 0, len(rows))
	for _, row := range rows {
		data = append(data, goravelhttp.Json{
			"id": row.ID, "title": row.Data["title"], "read_at": nullIfEmpty(row.ReadAt),
			"created_at": row.CreatedAt,
		})
	}
	return ctx.Response().Json(nethttp.StatusOK, paginatorJSON(ctx, data, total, page, perPage))
}

// Message 处理 GET /user/inbox/{id}：返回详情并标记已读。
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
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"id": row.ID, "title": row.Data["title"], "content": row.Data["content"],
		"format": row.Data["format"], "read_at": nullIfEmpty(row.ReadAt),
		"created_at": row.CreatedAt,
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
