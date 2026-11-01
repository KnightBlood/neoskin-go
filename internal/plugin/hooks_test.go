package plugin

import "testing"

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
