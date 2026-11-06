package bootstrap

import (
	contractsfoundation "github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/contracts/foundation/configuration"
	"github.com/goravel/framework/foundation"

	appmiddleware "neoskin/app/http/middleware"
	"neoskin/config"
	"neoskin/routes"
)

func Boot() contractsfoundation.Application {
	return foundation.Setup().
		WithCommands(Commands).
		// PHP `App\Http\Middleware\RedirectToSetup` 是全局中间件：未安装跳 /setup，
		// 已安装但版本落后时在请求期执行 update。
		WithMiddleware(func(middleware configuration.Middleware) {
			middleware.Append(appmiddleware.NewRedirectToSetup())
		}).
		WithRouting(func() {
			routes.Web()
			routes.Grpc()
		}).
		WithProviders(Providers).
		WithConfig(config.Boot).
		Create()
}
