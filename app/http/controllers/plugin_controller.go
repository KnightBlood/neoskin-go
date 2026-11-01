package controllers

import (
	"html"
	nethttp "net/http"
	"os"
	"strings"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/plugin"
	"neoskin/internal/service"
)

type PluginController struct{}

func NewPluginController() *PluginController {
	return &PluginController{}
}

func (r *PluginController) Data(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	manager := plugin.NewManager("", service.NewOption(conn, dialect))
	plugins, err := manager.All()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	data := make([]goravelhttp.Json, 0, len(plugins))
	for _, item := range plugins {
		icon := goravelhttp.Json{"fa": "plug", "faType": "fas", "bg": "navy"}
		if item.Manifest.Enchants != nil {
			if custom, ok := item.Manifest.Enchants["icon"].(map[string]any); ok {
				for key, value := range custom {
					icon[key] = value
				}
			}
		}
		data = append(data, goravelhttp.Json{
			"name": item.Manifest.Name, "title": item.Manifest.Title,
			"description": item.Manifest.Description, "version": item.Manifest.Version,
			"enabled": item.Enabled, "readme": hasReadme(manager, item),
			"config": false, "icon": icon,
		})
	}
	return ctx.Response().Json(nethttp.StatusOK, data)
}

func (r *PluginController) Manage(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	manager := plugin.NewManager("", service.NewOption(conn, dialect))
	name := ctx.Request().Input("name")
	item, err := manager.Find(name)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if item == nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 1, "message": "Plugin not found."})
	}
	switch ctx.Request().Input("action") {
	case "enable":
		if err := manager.SetEnabled(name, true); err != nil {
			plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventPluginBootFailed,
				Payload: map[string]any{"name": name, "error": err.Error()}})
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventPluginWasEnabled, Payload: name})
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": "Plugin enabled."})
	case "disable":
		if err := manager.SetEnabled(name, false); err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventPluginWasDisabled, Payload: name})
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": "Plugin disabled."})
	case "delete":
		if err := manager.Delete(name); err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
		plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventPluginWasDeleted, Payload: name})
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": "Plugin deleted."})
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 1, "message": "Invalid action."})
}

func (r *PluginController) Readme(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	manager := plugin.NewManager("", service.NewOption(conn, dialect))
	content, err := manager.Readme(ctx.Request().Route("name"))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if content == "" {
		return ctx.Response().String(nethttp.StatusNotFound, "No readme.")
	}
	body := "<!DOCTYPE html><html><body><pre>" + html.EscapeString(content) + "</pre></body></html>"
	return ctx.Response().String(nethttp.StatusOK, body)
}

func (r *PluginController) Config(ctx goravelhttp.Context) goravelhttp.Response {
	return ctx.Response().String(nethttp.StatusNotFound, "No config.")
}

func (r *PluginController) Upload(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	upload, err := ctx.Request().File("file")
	if err != nil || upload == nil {
		return validationFailed(ctx, map[string][]string{"file": {"The file field is required."}})
	}
	manager := plugin.NewManager("", service.NewOption(conn, dialect))
	if err := manager.ExtractZip(upload.File()); err != nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 1, "message": err.Error()})
	}
	_ = os.Remove(upload.File())
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": "Plugin installed."})
}

func (r *PluginController) Wget(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	url := strings.TrimSpace(ctx.Request().Input("url"))
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return validationFailed(ctx, map[string][]string{"url": {"The url field must be a valid URL."}})
	}
	file, err := os.CreateTemp("", "neoskin-plugin-*.zip")
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	path := file.Name()
	_ = file.Close()
	defer os.Remove(path)

	manager := plugin.NewManager("", service.NewOption(conn, dialect))
	if err := manager.Download(url, path); err != nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 1, "message": err.Error()})
	}
	if err := manager.ExtractZip(path); err != nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 1, "message": err.Error()})
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{"code": 0, "message": "Plugin installed."})
}

func hasReadme(manager *plugin.Manager, item plugin.Plugin) bool {
	content, _ := manager.Readme(item.Manifest.Name)
	return content != ""
}
