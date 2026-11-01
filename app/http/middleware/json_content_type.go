package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/goravel/framework/contracts/http"
	gonig "github.com/goravel/gin"
)

// JSONContentType 把 Goravel/Gin 的 "application/json; charset=utf-8"
// 规范为 "application/json"。
//
// 前端 app.js 的 fetch 封装使用严格相等判断 Content-Type，
// 带 charset 时会被当作纯文本返回，导致所有 AJAX 页面报错。
type JSONContentType struct{}

func NormalizeJSONContentType() http.Middleware {
	return &JSONContentType{}
}

func (m *JSONContentType) Handle(ctx http.Context) {
	if gonigCtx, ok := ctx.(*gonig.Context); ok {
		if ginCtx := gonigCtx.Instance(); ginCtx != nil && ginCtx.Writer != nil {
			ginCtx.Writer = &jsonWriter{ResponseWriter: ginCtx.Writer}
		}
	}
	ctx.Request().Next()
}

func (m *JSONContentType) Signature() string {
	return "neoskin:json_content_type"
}

type jsonWriter struct {
	gin.ResponseWriter
}

func (w *jsonWriter) normalize() {
	header := w.Header()
	if strings.HasPrefix(header.Get("Content-Type"), "application/json") {
		header.Set("Content-Type", "application/json")
	}
}

func (w *jsonWriter) WriteHeader(code int) {
	w.normalize()
	w.ResponseWriter.WriteHeader(code)
}

func (w *jsonWriter) WriteHeaderNow() {
	w.normalize()
	w.ResponseWriter.WriteHeaderNow()
}

func (w *jsonWriter) Write(data []byte) (int, error) {
	w.normalize()
	return w.ResponseWriter.Write(data)
}

func (w *jsonWriter) WriteString(value string) (int, error) {
	w.normalize()
	return w.ResponseWriter.WriteString(value)
}
