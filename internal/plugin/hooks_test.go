package plugin

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	contractshttp "github.com/goravel/framework/contracts/http"
	goravelroute "github.com/goravel/framework/contracts/route"
	mocksroute "github.com/goravel/framework/mocks/route"
	"github.com/stretchr/testify/mock"

	"neoskin/internal/db"
	"neoskin/internal/php"
	"neoskin/internal/repo"
)

func TestFilterPriorityAndRemove(t *testing.T) {
	filter := NewFilter()
	filter.Add("menu", func(value any, _ ...any) any {
		return value.(string) + "|second"
	}, 20)
	filter.Add("menu", func(value any, _ ...any) any {
		return value.(string) + "|first"
	}, 10)

	got := filter.Apply("menu", "start").(string)
	if got != "start|first|second" {
		t.Fatalf("apply=%q", got)
	}
	if filter.Listeners("menu") != 2 {
		t.Fatalf("listeners=%d", filter.Listeners("menu"))
	}
	filter.Remove("menu")
	if filter.Listeners("menu") != 0 {
		t.Fatal("remove failed")
	}
}

func TestHookMenuAndAssets(t *testing.T) {
	hook := NewHook()
	hook.AddMenuItem("user", 1, MenuItem{Title: "B", Link: "/b"})
	hook.AddMenuItem("user", 0, MenuItem{Title: "A", Link: "/a"})
	items := hook.MenuItems("user")
	if len(items) != 2 || items[0].Title != "A" || items[1].Title != "B" {
		t.Fatalf("items=%v", items)
	}

	hook.AddStyleFileToPage([]string{"/all.css"})
	hook.AddStyleFileToPage([]string{"/user.css"}, "user")
	styles := hook.StylesFor("user")
	if len(styles) != 2 || styles[0] != "/all.css" || styles[1] != "/user.css" {
		t.Fatalf("styles=%v", styles)
	}

	hook.AddScriptFileToPage([]string{"/app.js"}, "home")
	if len(hook.ScriptsFor("home")) != 1 {
		t.Fatal("scripts missing")
	}

	hook.AddUserBadge("STAFF", "primary")
	hook.AddUserBadge("VIP", "")
	badges := hook.UserBadges()
	if len(badges) != 2 || badges[1].Color != "primary" {
		t.Fatalf("badges=%v", badges)
	}
}

func TestDispatcher(t *testing.T) {
	dispatcher := NewDispatcher()
	events := []Event{}
	dispatcher.Listen(EventUserRegistered, func(event Event) {
		events = append(events, event)
	})
	dispatcher.Dispatch(Event{Name: EventUserRegistered, Payload: 1})
	dispatcher.Dispatch(Event{Name: EventTextureDeleting, Payload: 2})
	if len(events) != 1 || events[0].Payload != 1 {
		t.Fatalf("events=%v", events)
	}
	if dispatcher.Count(EventUserRegistered) != 1 {
		t.Fatal("count mismatch")
	}
}

func TestHookAddRouteAndApplyRoutes(t *testing.T) {
	// framework 自带的 Router/Action mockery mock：验证回调收到路由器，
	// 且注册顺序与 AddRoute 一致（等价 PHP Hook::addRoute 监听 ConfigureRoutes）。
	router := mocksroute.NewRouter(t)

	pluginFirst := func(_ contractshttp.Context) contractshttp.Response { return nil }
	pluginSecond := func(_ contractshttp.Context) contractshttp.Response { return nil }
	// HandlerFunc 是函数类型，mock 匹配用 Anything（路径已足够区分）。
	router.EXPECT().Post("/plugin/first", mock.Anything).Return(mocksroute.NewAction(t))
	router.EXPECT().Post("/plugin/second", mock.Anything).Return(mocksroute.NewAction(t))

	hook := NewHook()
	hook.AddRoute(nil) // nil 回调应被跳过

	var order []int
	hook.AddRoute(func(r goravelroute.Router) {
		order = append(order, 1)
		r.Post("/plugin/first", pluginFirst)
	})
	hook.AddRoute(func(r goravelroute.Router) {
		order = append(order, 2)
		r.Post("/plugin/second", pluginSecond)
	})

	// 未 ApplyRoutes 前不应注册任何路由（mock 未收到调用即隐式验证）。

	hook.ApplyRoutes(router)
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("order=%v", order)
	}

	// 再次 ApplyRoutes（每次路由注册周期）回调可重复执行，与 PHP
	// ConfigureRoutes 监听器对每次事件都会执行一致。
	hook.ApplyRoutes(router)

	// 派发 EventConfigureRoutes（routes/web.go 的时序：先 ApplyRoutes 再 Dispatch）。
	dispatcher := NewDispatcher()
	fired := 0
	dispatcher.Listen(EventConfigureRoutes, func(_ Event) { fired++ })
	dispatcher.Dispatch(Event{Name: EventConfigureRoutes, Payload: router})
	if fired != 1 {
		t.Fatalf("dispatch fired=%d", fired)
	}
}

func TestHookPushMiddleware(t *testing.T) {
	var dummy contractshttp.Middleware = &fakeMiddleware{signature: "m1"}
	var second contractshttp.Middleware = &fakeMiddleware{signature: "m2"}

	hook := NewHook()
	hook.PushMiddleware(nil) // nil 应被跳过
	hook.PushMiddleware(dummy)
	hook.PushMiddleware(second)

	got := hook.PushedMiddlewares()
	if len(got) != 2 || got[0] != dummy || got[1] != second {
		t.Fatalf("pushed=%v", got)
	}

	// 返回的是拷贝：外部改动不得影响内部状态。
	back := hook.PushedMiddlewares()
	back[0] = nil
	if again := hook.PushedMiddlewares(); again[0] != dummy {
		t.Fatal("PushedMiddlewares should return a copy")
	}
}

type fakeMiddleware struct{ signature string }

func (m *fakeMiddleware) Handle(contractshttp.Context) {}

func (m *fakeMiddleware) Signature() string { return m.signature }

func TestHookSendNotification(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.sqlite")
	conn, dialect, err := db.Open(db.Config{Connection: "sqlite", Database: path})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := db.NewInstaller(conn, dialect).Install(context.Background()); err != nil {
		t.Fatal(err)
	}

	users := repo.NewUserRepository(conn, dialect)
	password, err := php.HashPassword("secret123", 4)
	if err != nil {
		t.Fatal(err)
	}
	uid1, err := users.Insert("a@example.com", "A", password, "127.0.0.1", 1000)
	if err != nil {
		t.Fatal(err)
	}
	uid2, err := users.Insert("b@example.com", "B", password, "127.0.0.1", 1000)
	if err != nil {
		t.Fatal(err)
	}

	hook := NewHook()

	// 空入参 / nil 连接：无操作不报错，等价 PHP Notification::send 对空数组的短路。
	if err := hook.SendNotification(nil, "t", "c", conn, dialect); err != nil {
		t.Fatalf("empty uids: %v", err)
	}
	if err := hook.SendNotification([]int{uid1}, "t", "c", nil, dialect); err != nil {
		t.Fatalf("nil conn: %v", err)
	}

	// 非法 uid 跳过；合法 uid 各落一行 SiteMessage，data 仅含 title/content
	// （无 format/communication_id——对应 PHP Hook::sendNotification 的
	// SiteMessage 无 communication 关联场景）。
	if err := hook.SendNotification([]int{0, uid1, uid2}, "Title", "Content", conn, dialect); err != nil {
		t.Fatalf("send: %v", err)
	}

	notifications := repo.NewNotificationRepository(conn, dialect)
	total, err := notifications.CountByUser(uid1)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("count uid1=%d", total)
	}
	rows, err := notifications.PageByUser(uid2, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows uid2=%d", len(rows))
	}
	row := rows[0]
	if row.Type != repo.SiteMessageType {
		t.Fatalf("type=%q", row.Type)
	}
	if row.Data["title"] != "Title" || row.Data["content"] != "Content" {
		t.Fatalf("data=%v", row.Data)
	}
	if len(row.Data) != 2 {
		t.Fatalf("data should contain only title/content, got %v", row.Data)
	}
	// 未读：read_at 必须为 NULL（SQL 断言，不依赖 repo 对 NULL 的字符串映射）。
	var readAt sql.NullString
	if err := conn.QueryRow(`SELECT read_at FROM notifications WHERE notifiable_id = ?`, uid2).Scan(&readAt); err != nil {
		t.Fatal(err)
	}
	if readAt.Valid {
		t.Fatalf("read_at should be NULL, got %q", readAt.String)
	}
}
