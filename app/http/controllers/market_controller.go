package controllers

import (
	nethttp "net/http"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/plugin"
	"neoskin/internal/service"
)

type MarketController struct{}

func NewMarketController() *MarketController {
	return &MarketController{}
}

const defaultRegistry = "https://plugin.blessing.dreamcraft.eu.org/registry_{lang}.json"

func (r *MarketController) manager(ctx goravelhttp.Context) (*plugin.Manager,
	*service.Option, goravelhttp.Response) {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return nil, nil, resp
	}
	options := service.NewOption(conn, dialect)
	return plugin.NewManager("", options), options, nil
}

// List 处理 GET /admin/plugins/market/list。
func (r *MarketController) List(ctx goravelhttp.Context) goravelhttp.Response {
	manager, options, resp := r.manager(ctx)
	if resp != nil {
		return resp
	}
	registry := options.Get("plugins_registry")
	if registry == "" {
		registry = defaultRegistry
	}
	packages, err := plugin.FetchRegistry(registry, "zh_CN")
	if err != nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "admin.plugins.market.connection-error", "error", err.Error()),
		})
	}

	data := make([]goravelhttp.Json, 0, len(packages))
	for _, item := range packages {
		installed := any(false)
		canUpdate := false
		if current, _ := manager.Find(item.Name); current != nil {
			installed = current.Manifest.Version
			canUpdate = plugin.VersionGreater(item.Version, current.Manifest.Version)
		}
		data = append(data, goravelhttp.Json{
			"name": item.Name, "title": item.Title, "description": item.Description,
			"version": item.Version, "readme": item.Readme,
			"dist":    goravelhttp.Json{"url": item.Dist.URL},
			"require": item.Require,
			"dependencies": goravelhttp.Json{
				"all": item.Require, "unsatisfied": []any{},
			},
			"installed": installed, "can_update": canUpdate,
		})
	}
	return ctx.Response().Json(nethttp.StatusOK, data)
}

// Download 处理 POST /admin/plugins/market/download。
func (r *MarketController) Download(ctx goravelhttp.Context) goravelhttp.Response {
	manager, options, resp := r.manager(ctx)
	if resp != nil {
		return resp
	}
	name := ctx.Request().Input("name")
	registry := options.Get("plugins_registry")
	if registry == "" {
		registry = defaultRegistry
	}
	packages, err := plugin.FetchRegistry(registry, "zh_CN")
	if err != nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "admin.plugins.market.connection-error", "error", err.Error()),
		})
	}
	var target *plugin.MarketPackage
	for i := range packages {
		if packages[i].Name == name {
			target = &packages[i]
			break
		}
	}
	if target == nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "admin.plugins.market.non-existent", "plugin", name),
		})
	}
	if err := manager.DownloadPackage(target.Dist.URL); err != nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": err.Error(),
		})
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "admin.plugins.market.install-success"),
	})
}
