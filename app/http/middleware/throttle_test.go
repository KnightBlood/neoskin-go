package middleware

import (
	"testing"
	"time"

	contractscache "github.com/goravel/framework/contracts/cache"
)

// fakeCache 只实现限流计数用到的缓存方法；嵌入接口补齐其余方法
// （运行期不会被限流逻辑调用，panic 即测试故障信号）。
type fakeCache struct {
	contractscache.Driver
	items map[string]any
}

func newFakeCache() *fakeCache {
	return &fakeCache{items: map[string]any{}}
}

func (c *fakeCache) Store(string) contractscache.Driver { return c }

func (c *fakeCache) Get(key string, _ ...any) any {
	return c.items[key]
}

func (c *fakeCache) GetInt(key string, _ ...int) int {
	return int(currentInt64(c.items[key]))
}

func (c *fakeCache) GetInt64(key string, _ ...int64) int64 {
	return currentInt64(c.items[key])
}

func (c *fakeCache) GetBool(string, ...bool) bool { return false }

func (c *fakeCache) GetString(string, ...string) string { return "" }

func (c *fakeCache) Has(key string) bool {
	_, ok := c.items[key]
	return ok
}

func (c *fakeCache) Add(key string, value any, _ time.Duration) bool {
	if _, ok := c.items[key]; ok {
		return false
	}
	c.items[key] = value
	return true
}

func (c *fakeCache) Put(key string, value any, _ time.Duration) error {
	c.items[key] = value
	return nil
}

func (c *fakeCache) Forever(string, any) bool { return false }

func (c *fakeCache) Forget(key string) bool {
	_, existed := c.items[key]
	delete(c.items, key)
	return existed
}

func (c *fakeCache) Increment(key string, value ...int64) (int64, error) {
	step := int64(1)
	if len(value) > 0 {
		step = value[0]
	}
	current := currentInt64(c.items[key])
	c.items[key] = newInt64(current + step)
	return current + step, nil
}

func (c *fakeCache) Decrement(key string, value ...int64) (int64, error) {
	step := int64(1)
	if len(value) > 0 {
		step = value[0]
	}
	return c.Increment(key, -step)
}

func (c *fakeCache) Flush() bool {
	c.items = map[string]any{}
	return true
}

func currentInt64(value any) int64 {
	switch number := value.(type) {
	case *int64:
		return *number
	case int64:
		return number
	case int:
		return int64(number)
	}
	return 0
}

func TestThrottleFixedWindow(t *testing.T) {
	m := Throttle(3, 60).(*ThrottleMiddleware)
	cache := newFakeCache()
	key := rateSignatureKey("POST", "/admin/update/install", "203.0.113.9")

	// Laravel 语义：前 3 次请求可放行；计入第 3 次后达到上限。
	for expected := 1; expected <= 3; expected++ {
		if m.tooManyAttempts(cache, key) {
			t.Fatalf("attempt %d should pass", expected)
		}
		m.hit(cache, key)
	}
	if got := cache.GetInt(key, 0); got != 3 {
		t.Fatalf("counter got=%d want=3", got)
	}
	if !m.tooManyAttempts(cache, key) {
		t.Fatal("window should be locked after reaching max attempts")
	}

	// 计时器消失（窗口过期）后重置计数放行。
	cache.Forget(key + ":timer")
	if m.tooManyAttempts(cache, key) {
		t.Fatal("expired window should unlock")
	}

	// 不同签名互不影响。
	other := rateSignatureKey("POST", "/admin/update/install", "203.0.113.10")
	if m.tooManyAttempts(cache, other) {
		t.Fatal("other key should be independent")
	}
}

func TestThrottleHitRebasesSingleDigitCounter(t *testing.T) {
	m := Throttle(10, 60).(*ThrottleMiddleware)

	// Add 因 key 已存在而失败但 Increment 结果为 1 时，回填基值并刷新 TTL。
	cache := newFakeCache()
	cache.items["throttle:preexisting"] = newInt64(0)
	key := "throttle:preexisting"
	hits := m.hit(cache, key)
	if hits != 1 {
		t.Fatalf("hits got=%d want=1", hits)
	}
	if got := cache.GetInt(key, 0); got != 1 {
		t.Fatalf("counter got=%d want=1", got)
	}
}

func TestAPIThrottlePathGate(t *testing.T) {
	m := APIThrottle(60).(*ThrottleMiddleware)
	if !m.OnlyAPI {
		t.Fatal("api throttle should be path-gated")
	}
	if m.DecaySeconds.Seconds() != 60 {
		t.Fatalf("decay got=%v want=60s", m.DecaySeconds)
	}
	plain := Throttle(30, 60)
	if plain.(*ThrottleMiddleware).OnlyAPI {
		t.Fatal("plain throttle should not be path-gated")
	}
}
