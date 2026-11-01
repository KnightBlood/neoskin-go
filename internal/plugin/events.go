package plugin

import "sync"

// 24 个插件事件名（与 codegraph/events.json 一致）。
const (
	EventConfigureAdminMenu   = "ConfigureAdminMenu"
	EventConfigureExploreMenu = "ConfigureExploreMenu"
	EventConfigureRoutes      = "ConfigureRoutes"
	EventConfigureUserMenu    = "ConfigureUserMenu"
	EventPlayerProfileUpdated = "PlayerProfileUpdated"
	EventPlayerRetrieved      = "PlayerRetrieved"
	EventPlayerWasAdded       = "PlayerWasAdded"
	EventPlayerWasDeleted     = "PlayerWasDeleted"
	EventPlayerWillBeAdded    = "PlayerWillBeAdded"
	EventPlayerWillBeDeleted  = "PlayerWillBeDeleted"
	EventPluginBootFailed     = "PluginBootFailed"
	EventPluginWasDeleted     = "PluginWasDeleted"
	EventPluginWasDisabled    = "PluginWasDisabled"
	EventPluginWasEnabled     = "PluginWasEnabled"
	EventRenderingBadges      = "RenderingBadges"
	EventRenderingFooter      = "RenderingFooter"
	EventRenderingHeader      = "RenderingHeader"
	EventTextureDeleting      = "TextureDeleting"
	EventUserAuthenticated    = "UserAuthenticated"
	EventUserLoggedIn         = "UserLoggedIn"
	EventUserProfileUpdated   = "UserProfileUpdated"
	EventUserRegistered       = "UserRegistered"
	EventUserTryToLogin       = "UserTryToLogin"
	EventPluginEvent          = "Event"
)

// Event 是插件事件负载。
type Event struct {
	Name    string
	Payload any
}

type EventHandler func(Event)

// Dispatcher 是最小事件总线（对应 Laravel Event）。
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string][]EventHandler
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{handlers: map[string][]EventHandler{}}
}

// Listen 注册事件监听器。
func (d *Dispatcher) Listen(name string, handler EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[name] = append(d.handlers[name], handler)
}

// Dispatch 同步派发事件。
func (d *Dispatcher) Dispatch(event Event) {
	d.mu.RLock()
	handlers := append([]EventHandler{}, d.handlers[event.Name]...)
	d.mu.RUnlock()
	for _, handler := range handlers {
		handler(event)
	}
}

// Count 返回某事件的监听器数量。
func (d *Dispatcher) Count(name string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.handlers[name])
}
