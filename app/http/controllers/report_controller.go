package controllers

import (
	"database/sql"
	nethttp "net/http"
	"reflect"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/db"
	"neoskin/internal/model"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
	"neoskin/internal/storage"
)

type ReportController struct{}

func NewReportController() *ReportController {
	return &ReportController{}
}

type reportJSON struct {
	ID       int    `json:"id"`
	TID      int    `json:"tid"`
	Uploader int    `json:"uploader"`
	Reporter int    `json:"reporter"`
	Reason   string `json:"reason"`
	Status   int    `json:"status"`
	ReportAt string `json:"report_at"`
}

func toReportJSON(row repo.ReportRow) reportJSON {
	return reportJSON{
		ID: row.ID, TID: row.TID, Uploader: row.Uploader, Reporter: row.Reporter,
		Reason: row.Reason, Status: row.Status, ReportAt: row.ReportAt,
	}
}

// Submit 处理 POST /skinlib/report。
func (r *ReportController) Submit(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}
	if denied := requireVerified(ctx, user, service.NewOption(conn, dialect)); denied != nil {
		return denied
	}

	tid := ctx.Request().InputInt("tid")
	reason := ctx.Request().Input("reason")
	errs := map[string][]string{}
	if tid <= 0 {
		errs["tid"] = []string{vRequired(ctx, "tid")}
	}
	if reason == "" {
		errs["reason"] = []string{vRequired(ctx, "reason")}
	}
	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}

	textures := repo.NewTextureRepository(conn, dialect)
	texture, err := textures.FindByTid(tid)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if texture == nil {
		return validationFailed(ctx, map[string][]string{"tid": {vExists(ctx, "tid")}})
	}

	reports := repo.NewReportRepository(conn, dialect)
	if exists, _ := reports.Exists(user.UID, tid); exists {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "skinlib.report.duplicate"),
		})
	}

	options := service.NewOption(conn, dialect)
	score := options.Int("reporter_score_modification", 0)
	if score < 0 && user.Score < -score {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "skinlib.upload.lack-score"),
		})
	}

	if _, err := reports.Insert(tid, texture.Uploader, user.UID, reason); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	_ = repo.NewUserRepository(conn, dialect).UpdateScore(user.UID, user.Score+score)

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "skinlib.report.success"),
	})
}

// Manage 处理 GET /admin/reports/list、/api/admin/reports。
func (r *ReportController) Manage(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	_ = user

	page := ctx.Request().InputInt("page", 1)
	if page < 1 {
		page = 1
	}
	const perPage = 9
	reports := repo.NewReportRepository(conn, dialect)
	total, _ := reports.CountAll()
	rows, err := reports.PageAll(perPage, (page-1)*perPage)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	data := make([]reportJSON, 0, len(rows))
	for _, row := range rows {
		data = append(data, toReportJSON(row))
	}
	return ctx.Response().Json(nethttp.StatusOK, paginatorJSON(ctx, data, total, page, perPage))
}

// Track 处理 GET /api/user/reports：当前用户提交的举报（分页）。
func (r *ReportController) Track(ctx goravelhttp.Context) goravelhttp.Response {
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

	reports := repo.NewReportRepository(conn, dialect)
	total, _ := reports.CountByReporter(user.UID)
	rows, err := reports.PageByReporter(user.UID, perPage, (page-1)*perPage)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	textures := repo.NewTextureRepository(conn, dialect)
	data := make([]goravelhttp.Json, 0, len(rows))
	for _, row := range rows {
		name := ""
		if texture, _ := textures.FindByTid(row.TID); texture != nil {
			name = texture.Name
		}
		data = append(data, goravelhttp.Json{
			"id": row.ID, "tid": row.TID, "texture_name": name, "reason": row.Reason,
			"status": row.Status, "report_at": row.ReportAt,
		})
	}
	return ctx.Response().Json(nethttp.StatusOK, paginatorJSON(ctx, data, total, page, perPage))
}

// Review 处理 PUT /admin/reports/{report}、/api/admin/reports/{report}。
func (r *ReportController) Review(ctx goravelhttp.Context) goravelhttp.Response {
	admin, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}

	reportID := ctx.Request().RouteInt("report")
	reports := repo.NewReportRepository(conn, dialect)
	report, err := reports.Find(reportID)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if report == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}

	action := ctx.Request().Input("action")
	if action != "delete" && action != "ban" && action != "reject" {
		return validationFailed(ctx, map[string][]string{
			"action": {vIn(ctx, "action")},
		})
	}

	options := service.NewOption(conn, dialect)
	users := repo.NewUserRepository(conn, dialect)
	textures := repo.NewTextureRepository(conn, dialect)
	modification := options.Int("reporter_score_modification", 0)
	reward := options.Int("reporter_reward_score", 0)

	if action == "reject" {
		if reporter, _ := users.FindByID(report.Reporter); reporter != nil &&
			modification > 0 && report.Status == repo.ReportPending {
			_ = users.UpdateScore(reporter.UID, reporter.Score-modification)
		}
		_ = reports.UpdateStatus(report.ID, repo.ReportRejected)
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 0, "message": t(ctx, "general.op-success"), "data": goravelhttp.Json{"status": repo.ReportRejected},
		})
	}

	if action == "delete" {
		texture, _ := textures.FindByTid(report.TID)
		if texture != nil {
			if count, _ := textures.CountByHash(texture.Hash); count <= 1 {
				_ = storage.NewLocal(runtime.StorageRoot()).Delete(texture.Hash)
			}
			_ = textures.DeleteByTid(texture.Tid)
		} else {
			// 纹理已被上传者删除：只返还积分，不给举报奖励
			returnScore(users, options, report)
			_ = reports.UpdateStatus(report.ID, repo.ReportResolved)
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 0, "message": t(ctx, "general.texture-deleted"),
				"data": goravelhttp.Json{"status": repo.ReportResolved},
			})
		}
	} else if action == "ban" {
		uploader, _ := users.FindByID(report.Uploader)
		if uploader == nil {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": t(ctx, "admin.users.operations.non-existent"),
			})
		}
		if admin.Permission <= uploader.Permission {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": t(ctx, "admin.users.operations.no-permission"),
			})
		}
		if err := users.UpdatePermission(uploader.UID, model.PermissionBanned); err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
	}

	returnScore(users, options, report)
	if reporter, _ := users.FindByID(report.Reporter); reporter != nil &&
		report.Status == repo.ReportPending && reward != 0 {
		_ = users.UpdateScore(reporter.UID, reporter.Score+reward)
	}
	_ = reports.UpdateStatus(report.ID, repo.ReportResolved)

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "general.op-success"),
		"data": goravelhttp.Json{"status": repo.ReportResolved},
	})
}

func returnScore(users *repo.UserRepository,
	options *service.Option, report *repo.ReportRow) {
	modification := options.Int("reporter_score_modification", 0)
	if report.Status != repo.ReportPending || modification >= 0 {
		return
	}
	if reporter, _ := users.FindByID(report.Reporter); reporter != nil {
		_ = users.UpdateScore(reporter.UID, reporter.Score-modification)
	}
}

func paginatorJSON(ctx goravelhttp.Context, data any, total, page, perPage int) goravelhttp.Json {
	lastPage := (total + perPage - 1) / perPage
	if lastPage < 1 {
		lastPage = 1
	}
	path := "http://" + ctx.Request().Host() + ctx.Request().Path()
	var from, to any
	count := 0
	if value := reflect.ValueOf(data); value.Kind() == reflect.Slice {
		count = value.Len()
	}
	if count > 0 {
		fromValue := (page-1)*perPage + 1
		from = fromValue
		to = fromValue + count - 1
	}
	return goravelhttp.Json{
		"current_page": page, "data": data, "first_page_url": pageURL(path, 1),
		"from": from, "last_page": lastPage, "last_page_url": pageURL(path, lastPage),
		"links": paginationLinks(path, page, lastPage), "next_page_url": nextPageURL(path, page, lastPage),
		"path": path, "per_page": perPage, "prev_page_url": prevPageURL(path, page),
		"to": to, "total": total,
	}
}

func requireAdmin(ctx goravelhttp.Context) (*model.User, *sql.DB, *db.Dialect,
	goravelhttp.Response) {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return nil, nil, nil, ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return nil, nil, nil, ctx.Response().Json(nethttp.StatusUnauthorized,
			goravelhttp.Json{"message": "Unauthenticated."})
	}
	if !user.IsAdmin() {
		return nil, nil, nil, ctx.Response().String(nethttp.StatusForbidden, "Forbidden")
	}
	return user, conn, dialect, nil
}

// requireAdminPage 是后台 HTML 页面的守卫，对应 PHP 的
// `authorize` + `can:admin` 中间件：未登录的浏览器请求重定向到登录页
// （而不是返回 JSON 401），已登录但非管理员返回 403。
func requireAdminPage(ctx goravelhttp.Context) (*model.User, *sql.DB, *db.Dialect,
	goravelhttp.Response) {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return nil, nil, nil, ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return nil, nil, nil, ctx.Response().Redirect(nethttp.StatusFound, "/auth/login")
	}
	if !user.IsAdmin() {
		return nil, nil, nil, ctx.Response().String(nethttp.StatusForbidden, "Forbidden")
	}
	return user, conn, dialect, nil
}
