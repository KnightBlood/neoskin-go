package controllers

import (
	nethttp "net/http"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/app/facades"
	"neoskin/internal/service"
)

// ApiController 对应 HomeController@apiRoot。
type ApiController struct{}

func NewApiController() *ApiController {
	return &ApiController{}
}

// Root 处理 ANY /api。
func (r *ApiController) Root(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	options := service.NewOption(conn, dialect)
	locale := facades.Config().GetString("app.locale", "zh_CN")
	index := options.Int("copyright_prefer_"+locale,
		options.Int("copyright_prefer", 0))
	copyrights := []string{
		"Powered with ❤ by Blessing Skin Server.",
		"Powered by Blessing Skin Server.",
		"Proudly powered by Blessing Skin Server.",
		"由 Blessing Skin Server 强力驱动。",
		"采用 Blessing Skin Server 搭建。",
		"使用 Blessing Skin Server 稳定运行。",
		"自豪地采用 Blessing Skin Server。",
	}
	if index < 0 || index >= len(copyrights) {
		index = 0
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"blessing_skin": facades.Config().GetString("app.version", "6.0.11_patch"),
		"spec":          0,
		"copyright":     copyrights[index],
		"site_name":     options.Get("site_name"),
	})
}
