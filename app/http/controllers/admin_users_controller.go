package controllers

import (
	"database/sql"
	nethttp "net/http"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/db"
	"neoskin/internal/model"
	"neoskin/internal/php"
	"neoskin/internal/repo"
)

type AdminUsersController struct{}

func NewAdminUsersController() *AdminUsersController {
	return &AdminUsersController{}
}

// List 处理 GET /admin/users/list、/api/admin/users。
func (r *AdminUsersController) List(ctx goravelhttp.Context) goravelhttp.Response {
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
	users := repo.NewUserRepository(conn, dialect)
	total, err := users.AdminCount(q)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	rows, err := users.AdminPage(q, perPage, (page-1)*perPage)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	data := make([]userJSON, 0, len(rows))
	for _, user := range rows {
		data = append(data, toUserJSON(user))
	}

	return ctx.Response().Json(nethttp.StatusOK, paginatorJSON(ctx, data, total, page, perPage))
}

// Email 处理 PUT /admin/users/{user}/email。
func (r *AdminUsersController) Email(ctx goravelhttp.Context) goravelhttp.Response {
	target, _, conn, dialect, resp := r.target(ctx)
	if resp != nil {
		return resp
	}

	email := ctx.Request().Input("email")
	if email == "" {
		return validationFailed(ctx, map[string][]string{"email": {"The email field is required."}})
	}
	users := repo.NewUserRepository(conn, dialect)
	if count, _ := users.CountByEmail(email); count > 0 &&
		(email != target.Email) {
		return validationFailed(ctx, map[string][]string{
			"email": {"The email has already been taken."},
		})
	}
	if err := users.UpdateEmail(target.UID, email); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": t(ctx, "admin.users.operations.email.success")})
}

// Nickname 处理 PUT /admin/users/{user}/nickname。
func (r *AdminUsersController) Nickname(ctx goravelhttp.Context) goravelhttp.Response {
	target, _, conn, dialect, resp := r.target(ctx)
	if resp != nil {
		return resp
	}
	nickname := ctx.Request().Input("nickname")
	if nickname == "" {
		return validationFailed(ctx, map[string][]string{
			"nickname": {"The nickname field is required."},
		})
	}
	if err := repo.NewUserRepository(conn, dialect).UpdateNickname(target.UID, nickname); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": t(ctx, "admin.users.operations.nickname.success", "new", nickname)})
}

// Password 处理 PUT /admin/users/{user}/password。
func (r *AdminUsersController) Password(ctx goravelhttp.Context) goravelhttp.Response {
	target, _, conn, dialect, resp := r.target(ctx)
	if resp != nil {
		return resp
	}
	password := ctx.Request().Input("password")
	if len(password) < 8 || len(password) > 16 {
		return validationFailed(ctx, map[string][]string{
			"password": {"The password must be between 8 and 16 characters."},
		})
	}
	hashed, err := php.HashPassword(password, 0)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if err := repo.NewUserRepository(conn, dialect).UpdatePassword(target.UID, hashed); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": t(ctx, "admin.users.operations.password.success")})
}

// Permission 处理 PUT /admin/users/{user}/permission。
func (r *AdminUsersController) Permission(ctx goravelhttp.Context) goravelhttp.Response {
	target, admin, conn, dialect, resp := r.target(ctx)
	if resp != nil {
		return resp
	}
	permission := ctx.Request().InputInt("permission", 99)
	if permission != model.PermissionBanned && permission != model.PermissionNormal &&
		permission != model.PermissionAdmin {
		return validationFailed(ctx, map[string][]string{
			"permission": {"The selected permission is invalid."},
		})
	}
	if permission == model.PermissionAdmin && admin.Permission < model.PermissionSuper {
		return ctx.Response().String(nethttp.StatusForbidden, "Forbidden")
	}
	if target.UID == admin.UID {
		return ctx.Response().String(nethttp.StatusForbidden, "Forbidden")
	}
	if err := repo.NewUserRepository(conn, dialect).UpdatePermission(target.UID, permission); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": t(ctx, "admin.users.operations.permission")})
}

// Score 处理 PUT /admin/users/{user}/score。
func (r *AdminUsersController) Score(ctx goravelhttp.Context) goravelhttp.Response {
	target, _, conn, dialect, resp := r.target(ctx)
	if resp != nil {
		return resp
	}
	score := ctx.Request().InputInt("score", 0)
	if err := repo.NewUserRepository(conn, dialect).UpdateScore(target.UID, score); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": t(ctx, "admin.users.operations.score.success")})
}

// Verification 处理 PUT /admin/users/{user}/verification（切换）。
func (r *AdminUsersController) Verification(ctx goravelhttp.Context) goravelhttp.Response {
	target, _, conn, dialect, resp := r.target(ctx)
	if resp != nil {
		return resp
	}
	if err := repo.NewUserRepository(conn, dialect).UpdateVerified(target.UID, !target.Verified); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": t(ctx, "admin.users.operations.verification.success")})
}

// Delete 处理 DELETE /admin/users/{user}。
func (r *AdminUsersController) Delete(ctx goravelhttp.Context) goravelhttp.Response {
	target, _, conn, dialect, resp := r.target(ctx)
	if resp != nil {
		return resp
	}
	if err := repo.NewUserRepository(conn, dialect).Delete(target.UID); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": t(ctx, "admin.users.operations.delete.success")})
}

func (r *AdminUsersController) target(ctx goravelhttp.Context) (*model.User, *model.User,
	*sql.DB, *db.Dialect, goravelhttp.Response) {
	admin, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return nil, nil, nil, nil, resp
	}

	uid := ctx.Request().RouteInt("user")
	target, err := repo.NewUserRepository(conn, dialect).FindByID(uid)
	if err != nil {
		return nil, nil, nil, nil, ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if target == nil {
		return nil, nil, nil, nil, ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	if target.UID != admin.UID && target.Permission >= admin.Permission {
		return nil, nil, nil, nil, ctx.Response().String(nethttp.StatusForbidden, "Forbidden")
	}
	return target, admin, conn, dialect, nil
}

var _ = nethttp.StatusOK
