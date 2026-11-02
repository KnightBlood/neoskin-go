package captcha

import (
	"sync"
	"time"
)

// loginFailTTL 对应 PHP `Cache::put(key, $loginFails, 3600)`：每次失败都会
// 刷新 1 小时的过期时间。
const loginFailTTL = time.Hour

// loginFailPruneThreshold 超过该数量时清理过期项，防止无限增长的 map。
const loginFailPruneThreshold = 10000

// loginFailEntry 是单个 IP 的失败计数（等价 PHP `sha1('login_fails_'.$ip)` 缓存项）。
type loginFailEntry struct {
	count     int
	expiresAt time.Time
}

// trackerNow 便于测试注入时间。
var trackerNow = time.Now

// LoginFailTracker 维护「登录失败次数」（PHP 在 Laravel Cache 中按 IP 计数），
// 触发条件 `loginFails > 3` 时要求验证码。
type LoginFailTracker struct {
	mu      sync.Mutex
	entries map[string]*loginFailEntry
}

func newLoginFailTracker() *LoginFailTracker {
	return &LoginFailTracker{entries: map[string]*loginFailEntry{}}
}

var defaultLoginFails = newLoginFailTracker()

// LoginFails 返回进程级单例；重启后计数清零（比 PHP 文件缓存收敛，安全性等价）。
func LoginFails() *LoginFailTracker {
	return defaultLoginFails
}

func validIP(ip string) bool {
	return ip != ""
}

// Count 返回该 IP 当前失败次数（过期视为 0）。
func (t *LoginFailTracker) Count(ip string) int {
	if t == nil || !validIP(ip) {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, ok := t.entries[ip]
	if !ok || !entry.expiresAt.After(trackerNow()) {
		if ok {
			delete(t.entries, ip)
		}
		return 0
	}
	return entry.count
}

// Increment 使失败次数加一并刷新 TTL（`Cache::put(key, $loginFails, 3600)`）。
func (t *LoginFailTracker) Increment(ip string) int {
	if t == nil || !validIP(ip) {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := trackerNow()
	entry, ok := t.entries[ip]
	if !ok || !entry.expiresAt.After(now) {
		entry = &loginFailEntry{count: 0, expiresAt: now.Add(loginFailTTL)}
		t.entries[ip] = entry
	}
	entry.count++
	entry.expiresAt = now.Add(loginFailTTL)
	if entry.count > 3 && len(t.entries) > loginFailPruneThreshold {
		t.pruneLocked(now)
	}
	return entry.count
}

// Reset 在登录成功后清零（`Cache::forget($loginFailsCacheKey)`）。
func (t *LoginFailTracker) Reset(ip string) {
	if t == nil || !validIP(ip) {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, ip)
}

func (t *LoginFailTracker) pruneLocked(now time.Time) {
	for key := range t.entries {
		if entry := t.entries[key]; entry.expiresAt.Before(now) {
			delete(t.entries, key)
		}
	}
}
