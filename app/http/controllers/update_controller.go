package controllers

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/app/facades"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
	"neoskin/internal/update"
)

// UpdateController 对应源 UpdateController（在线更新）。
type UpdateController struct{}

func NewUpdateController() *UpdateController {
	return &UpdateController{}
}

type updateRemote struct {
	Spec    int    `json:"spec"`
	Latest  string `json:"latest"`
	PHP     string `json:"php"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Package string `json:"package"`
}

const sessionPackageKey = "system_update_package"

func (r *UpdateController) manager() *update.Manager {
	root, _ := os.Getwd()
	return update.NewManager(root, runtime.StorageRoot(), facades.Config().GetString("app.version", "6.0.11_patch"))
}

// Show 处理 GET /admin/update。
func (r *UpdateController) Show(ctx goravelhttp.Context) goravelhttp.Response {
	if _, _, _, resp := requireAdmin(ctx); resp != nil {
		return resp
	}
	current := facades.Config().GetString("app.version", "6.0.11_patch")
	info, reason := r.remoteInfo(ctx)
	latest := ""
	canUpdate := false
	if info != nil {
		latest = info.Latest
		canUpdate = update.Compare(latest, current) > 0
		if !canUpdate {
			reason = ""
		}
	}

	session := ctx.Request().Session()
	notice := ""
	updateError := reason
	if session != nil {
		if value := session.Get("update_notice"); value != nil {
			if text := fmt.Sprint(value); text != "<nil>" {
				notice = text
			}
		}
		if value := session.Get("update_error"); value != nil {
			if text := fmt.Sprint(value); text != "<nil>" && text != "" {
				updateError = text
			}
		}
		session.Forget("update_notice")
		session.Forget("update_error")
	}

	packageInfo := any(nil)
	if session != nil {
		if raw := session.Get(sessionPackageKey); raw != nil {
			var parsed update.Info
			if json.Unmarshal([]byte(fmt.Sprint(raw)), &parsed) == nil {
				packageInfo = parsed
			}
		}
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"latest":         latest,
		"current":        current,
		"error":          updateError,
		"can_update":     canUpdate,
		"package":        packageInfo,
		"notice":         notice,
		"patch_releases": []any{},
		"upload_limits": goravelhttp.Json{
			"application": update.MaxBytes, "upload": update.MaxBytes,
		},
	})
}

// UpdatePage 处理 GET /admin/update（HTML 页面）。
func (r *UpdateController) UpdatePage(ctx goravelhttp.Context) goravelhttp.Response {
	if _, _, _, resp := requireAdmin(ctx); resp != nil {
		return resp
	}
	current := facades.Config().GetString("app.version", "6.0.11_patch")
	info, reason := r.remoteInfo(ctx)
	latest, canUpdate := "", false
	if info != nil {
		latest = info.Latest
		canUpdate = update.Compare(latest, current) > 0
		if !canUpdate {
			reason = ""
		}
	}

	data, translator, resp := adminPageData(ctx, "general.check-update", nil)
	if resp != nil {
		return resp
	}
	data.LatestVersion = latest
	data.CurrentVersion = current
	data.CanUpdate = canUpdate
	data.UpdateError = reason
	data.UpdateNotice = pullFlash(ctx, "update_notice")
	if flash := pullFlash(ctx, "update_error"); flash != "" {
		data.UpdateError = flash
	}
	data.UploadLimit = "1 GiB"

	if session := ctx.Request().Session(); session != nil {
		if raw := session.Get(sessionPackageKey); raw != nil {
			var parsed update.Info
			if json.Unmarshal([]byte(fmt.Sprint(raw)), &parsed) == nil && parsed.ID != "" {
				data.HasPackage = true
				data.PackageID = parsed.ID
				data.PackageVersion = parsed.Version
				data.PackageSHA = parsed.SHA256
				data.PackageCount = parsed.Count
			}
		}
	}
	return renderAdminPage(ctx, "admin_update", data, translator)
}

// Download 处理 POST /admin/update/download。
func (r *UpdateController) Download(ctx goravelhttp.Context) goravelhttp.Response {
	if _, _, _, resp := requireAdmin(ctx); resp != nil {
		return resp
	}
	info, _ := r.remoteInfo(ctx)
	current := facades.Config().GetString("app.version", "6.0.11_patch")
	if info == nil || update.Compare(info.Latest, current) <= 0 || info.URL == "" {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": "Your system is up to date.",
		})
	}

	path, status, err := downloadToTemp(info.URL)
	if err != nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": fmt.Sprintf("Download failed (HTTP status code: %d).", status),
		})
	}
	defer os.Remove(path)

	manager := r.manager()
	if info.Package == "full" {
		if !validSHA256(info.SHA256) {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": "运行包校验值与更新信息不一致，请重新检查更新。",
			})
		}
		sum, err := fileSHA256(path)
		if err != nil || !strings.EqualFold(sum, info.SHA256) {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": "运行包校验值与更新信息不一致，请重新检查更新。",
			})
		}
		prepared, err := manager.Prepare(path)
		if err != nil {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": err.Error(),
			})
		}
		if prepared.Version != info.Latest {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": "运行包版本与更新信息不一致。",
			})
		}
		raw, _ := json.Marshal(prepared)
		if session := ctx.Request().Session(); session != nil {
			session.Put(sessionPackageKey, string(raw))
		}
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 0, "message": "运行包已下载并校验，请确认后安装。",
			"data": goravelhttp.Json{"review": true},
		})
	}

	if err := manager.Extract(path); err != nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": err.Error(),
		})
	}
	removeCaches()
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": "Update completed.",
	})
}

// Upload 处理 POST /admin/update/upload。
func (r *UpdateController) Upload(ctx goravelhttp.Context) goravelhttp.Response {
	if _, _, _, resp := requireAdmin(ctx); resp != nil {
		return resp
	}
	file, err := ctx.Request().File("package")
	if err != nil || file == nil || file.File() == "" {
		return r.redirectNotice(ctx, "update_error", "请选择 ZIP 运行包。")
	}
	if !strings.EqualFold(file.GetClientOriginalExtension(), "zip") {
		return r.redirectNotice(ctx, "update_error", "请选择 ZIP 运行包。")
	}
	info, err := r.manager().Prepare(file.File())
	if err != nil {
		return r.redirectNotice(ctx, "update_error", err.Error())
	}
	raw, _ := json.Marshal(info)
	if session := ctx.Request().Session(); session != nil {
		session.Put(sessionPackageKey, string(raw))
	}
	return r.redirectNotice(ctx, "update_notice", "运行包校验完成，请确认版本与校验值后安装。")
}

// Install 处理 POST /admin/update/install。
func (r *UpdateController) Install(ctx goravelhttp.Context) goravelhttp.Response {
	if _, _, _, resp := requireAdmin(ctx); resp != nil {
		return resp
	}
	id := strings.ToLower(strings.TrimSpace(ctx.Request().Input("id")))
	if len(id) != 32 {
		return validationFailed(ctx, map[string][]string{
			"id": {"The id field must be 32 characters."},
		})
	}
	session := ctx.Request().Session()
	raw := ""
	if session != nil {
		if value := session.Get(sessionPackageKey); value != nil {
			raw = fmt.Sprint(value)
		}
	}
	var pending update.Info
	if raw == "" || json.Unmarshal([]byte(raw), &pending) != nil ||
		subtle.ConstantTimeCompare([]byte(pending.ID), []byte(id)) != 1 {
		return ctx.Response().String(nethttp.StatusForbidden, "Forbidden")
	}
	if err := r.manager().Install(id); err != nil {
		return r.redirectNotice(ctx, "update_error", err.Error())
	}
	removeCaches()
	if session != nil {
		session.Forget(sessionPackageKey)
	}
	return r.redirectNotice(ctx, "update_notice",
		"系统程序已更新，版本检查与数据库升级将在后续请求中完成。")
}

func (r *UpdateController) redirectNotice(ctx goravelhttp.Context, key, message string) goravelhttp.Response {
	if session := ctx.Request().Session(); session != nil {
		session.Put(key, message)
	}
	return ctx.Response().Redirect(nethttp.StatusFound, "/admin/update")
}

func (r *UpdateController) remoteInfo(ctx goravelhttp.Context) (*updateRemote, string) {
	source := facades.Config().GetString("app.update_source")
	if _, conn, dialect, resp := requireAdmin(ctx); resp == nil {
		if value := service.NewOption(conn, dialect).Get("update_source"); value != "" {
			source = value
		}
	}
	if source == "" {
		return nil, "更新源连接失败"
	}
	client := &nethttp.Client{Timeout: 8 * time.Second}
	response, err := client.Get(source)
	if err != nil {
		return nil, "更新源连接失败"
	}
	defer response.Body.Close()
	if response.StatusCode != nethttp.StatusOK {
		return nil, fmt.Sprintf("HTTP status code: %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, "更新源连接失败"
	}
	var info updateRemote
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, "更新源返回格式错误"
	}
	if info.Spec != update.Spec {
		return nil, "更新源 spec 与当前版本不匹配"
	}
	if info.Latest == "" || info.PHP == "" {
		return nil, ""
	}
	return &info, ""
}

func downloadToTemp(url string) (string, int, error) {
	client := &nethttp.Client{Timeout: 5 * time.Minute}
	response, err := client.Get(url)
	if err != nil {
		return "", 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != nethttp.StatusOK {
		return "", response.StatusCode, errors.New("download failed")
	}
	file, err := os.CreateTemp("", "bs-update-*.zip")
	if err != nil {
		return "", response.StatusCode, err
	}
	name := file.Name()
	if _, err := io.Copy(file, response.Body); err != nil {
		_ = file.Close()
		_ = os.Remove(name)
		return "", response.StatusCode, err
	}
	_ = file.Close()
	return name, response.StatusCode, nil
}

func validSHA256(value string) bool {
	value = strings.ToLower(value)
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func removeCaches() {
	root, _ := os.Getwd()
	_ = os.Remove(filepath.Join(runtime.StorageRoot(), "options.php"))
	for _, name := range []string{"config.php", "routes.php", "routes-v7.php"} {
		_ = os.Remove(filepath.Join(root, "bootstrap", "cache", name))
	}
}
