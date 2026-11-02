package plugin

import (
	"database/sql"
	"sort"
	"sync"

	contractshttp "github.com/goravel/framework/contracts/http"
	goravelroute "github.com/goravel/framework/contracts/route"

	"neoskin/internal/db"
	"neoskin/internal/php"
	"neoskin/internal/repo"
)

// Filter 对应 Blessing\Filter：按优先级执行过滤器。
type FilterFunc func(value any, args ...any) any

type filterListener struct {
	priority int
	order    int
	fn       FilterFunc
}

type Filter struct {
	mu        sync.RWMutex
	listeners map[string][]filterListener
	sequence  int
}

func NewFilter() *Filter {
	return &Filter{listeners: map[string][]filterListener{}}
}

// Add 注册过滤器；priority 越小越先执行（与 PHP 默认 20 一致）。
func (f *Filter) Add(name string, fn FilterFunc, priority int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sequence++
	f.listeners[name] = append(f.listeners[name], filterListener{
		priority: priority, order: f.sequence, fn: fn,
	})
	sort.SliceStable(f.listeners[name], func(i, j int) bool {
		a, b := f.listeners[name][i], f.listeners[name][j]
		if a.priority != b.priority {
			return a.priority < b.priority
		}
		return a.order < b.order
	})
}

// Apply 依次执行过滤器；value 为初始值，args 为附加参数。
func (f *Filter) Apply(name string, value any, args ...any) any {
	f.mu.RLock()
	listeners := append([]filterListener{}, f.listeners[name]...)
	f.mu.RUnlock()
	for _, listener := range listeners {
		value = listener.fn(value, args...)
	}
	return value
}

// Remove 移除某个钩子的全部过滤器。
func (f *Filter) Remove(name string) {
	f.mu.Lock()
	delete(f.listeners, name)
	f.mu.Unlock()
}

// Listeners 返回某个钩子的过滤器数量。
func (f *Filter) Listeners(name string) int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.listeners[name])
}

// MenuItem 是插件添加的菜单项。
type MenuItem struct {
	Title  string
	Link   string
	Icon   string
	NewTab bool
}

type positionedItem struct {
	position int
	order    int
	item     MenuItem
}

// Badge 是用户徽章（Hook::addUserBadge）。
type Badge struct {
	Text  string
	Color string
}

// Hook 对应 App\Services\Hook 的 7 个方法。
type Hook struct {
	mu       sync.Mutex
	menus    map[string][]positionedItem
	styles   map[string][]string
	scripts  map[string][]string
	badges   []Badge
	sequence int
	// routeHooks 对应 Hook::addRoute（监听 ConfigureRoutes）注册的回调。
	routeHooks []func(r goravelroute.Router)
	// pushedMiddlewares 对应 Hook::pushMiddleware 追加的全局中间件。
	pushedMiddlewares []contractshttp.Middleware
}

func NewHook() *Hook {
	return &Hook{
		menus:   map[string][]positionedItem{},
		styles:  map[string][]string{},
		scripts: map[string][]string{},
	}
}

// AddMenuItem 在指定菜单位置插入项（category = user/admin/explore）。
func (h *Hook) AddMenuItem(category string, position int, item MenuItem) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sequence++
	items := append(h.menus[category], positionedItem{
		position: position, order: h.sequence, item: item,
	})
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].position != items[j].position {
			return items[i].position < items[j].position
		}
		return items[i].order < items[j].order
	})
	h.menus[category] = items
}

// MenuItems 返回某个分类的菜单项。
func (h *Hook) MenuItems(category string) []MenuItem {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]MenuItem, 0, len(h.menus[category]))
	for _, item := range h.menus[category] {
		result = append(result, item.item)
	}
	return result
}

// AddStyleFileToPage 为指定页面追加样式文件（pages 为空表示所有页面）。
func (h *Hook) AddStyleFileToPage(urls []string, pages ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(pages) == 0 {
		pages = []string{"*"}
	}
	for _, page := range pages {
		h.styles[page] = append(h.styles[page], urls...)
	}
}

// AddScriptFileToPage 为指定页面追加脚本文件。
func (h *Hook) AddScriptFileToPage(urls []string, pages ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(pages) == 0 {
		pages = []string{"*"}
	}
	for _, page := range pages {
		h.scripts[page] = append(h.scripts[page], urls...)
	}
}

// StylesFor 返回某页面（含 * 通配）的样式文件。
func (h *Hook) StylesFor(page string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append(append([]string{}, h.styles["*"]...), h.styles[page]...)
}

// ScriptsFor 返回某页面的脚本文件。
func (h *Hook) ScriptsFor(page string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append(append([]string{}, h.scripts["*"]...), h.scripts[page]...)
}

// AddUserBadge 注册用户徽章。
func (h *Hook) AddUserBadge(text, color string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if color == "" {
		color = "primary"
	}
	h.badges = append(h.badges, Badge{Text: text, Color: color})
}

// UserBadges 返回已注册徽章。
func (h *Hook) UserBadges() []Badge {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]Badge{}, h.badges...)
}

// AddRoute 对应 Hook::addRoute：注册回调监听 ConfigureRoutes 事件，回调在
// 核心路由注册完成后收到全局路由器（goravel contracts/route.Router），
// 可用 Register/Router 原生 API 注册新路由（与 PHP 对 $event->router 的操作一致）。
func (h *Hook) AddRoute(fn func(r goravelroute.Router)) {
	if fn == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.routeHooks = append(h.routeHooks, fn)
}

// ApplyRoutes 在核心路由注册完成后执行 AddRoute 收集的全部回调
// （与 PHP RouteServiceProvider::map 末尾 event(new ConfigureRoutes) 的
// 时序一致：先注册回调再统一执行）。之后由调用方派发 EventConfigureRoutes。
func (h *Hook) ApplyRoutes(router goravelroute.Router) {
	h.mu.Lock()
	hooks := append([]func(r goravelroute.Router){}, h.routeHooks...)
	h.mu.Unlock()
	for _, fn := range hooks {
		fn(router)
	}
}

// PushMiddleware 对应 Hook::pushMiddleware：向运行期全局中间件链追加条目。
// 偏差：Go 侧中间件链在路由注册时一次性建立，因此 push 需在 routes.Web()
// 注册之前完成（PHP 的运行期 push 同样只对后续请求生效）。
func (h *Hook) PushMiddleware(middleware contractshttp.Middleware) {
	if middleware == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pushedMiddlewares = append(h.pushedMiddlewares, middleware)
}

// PushedMiddlewares 返回已 push 的全局中间件（由路由注册方挂入链尾，对应
// Laravel Kernel::pushMiddleware 追加到全局栈末尾的顺序）。
func (h *Hook) PushedMiddlewares() []contractshttp.Middleware {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]contractshttp.Middleware{}, h.pushedMiddlewares...)
}

// SendNotification 等价 PHP Hook::sendNotification：$users 经
// Notification::send 以 SiteMessage 写入 notifications 表（database channel）；
// 无 communication 关联时 data 仅含 title/content。uids 与 PHP Arr::wrap($users)
// 的 User 模型数组对应（Go 插件以 uid 指定通知对象）。
func (h *Hook) SendNotification(uids []int, title, content string, conn *sql.DB, dialect *db.Dialect) error {
	if len(uids) == 0 || conn == nil {
		return nil
	}
	notifications := repo.NewNotificationRepository(conn, dialect)
	now := php.Now()
	payload := map[string]any{"title": title, "content": content}
	for _, uid := range uids {
		if uid <= 0 {
			continue
		}
		if err := notifications.Insert(uid, payload, now); err != nil {
			return err
		}
	}
	return nil
}

// 默认注册表：Go 插件/内置扩展通过这三个单例接入。
var (
	DefaultHook       = NewHook()
	DefaultFilter     = NewFilter()
	DefaultDispatcher = NewDispatcher()
)
