package middleware

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	nethttp "net/http"
	"strconv"
	"strings"
	"time"

	contractscache "github.com/goravel/framework/contracts/cache"
	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/app/facades"
)

// ThrottleMiddleware 复刻 Laravel 路由注解 `throttle:N,M`
// （Illuminate\Routing\Middleware\ThrottleRequests + Illuminate\Cache\RateLimiter）：
// 以「方法|路径|IP」为签名做固定窗口计数，decaySeconds 秒内最多
// maxAttempts 次，超限返回 429（响应体 "Too Many Attempts."，带
// Retry-After 与 X-RateLimit-* 头）。
type ThrottleMiddleware struct {
	MaxAttempts  int
	DecaySeconds time.Duration
	// OnlyAPI 对应 PHP api 中间件组的全局 throttle:60,1：仅作用于 /api
	// 前缀，且 APP_ENV=testing 时跳过（与 PHP RouteServiceProvider 一致）。
	OnlyAPI bool
}

// Throttle 复刻单条路由的 `throttle:N,M`（N 次/decaySeconds 秒）。
func Throttle(maxAttempts, decaySeconds int) goravelhttp.Middleware {
	return &ThrottleMiddleware{
		MaxAttempts:  maxAttempts,
		DecaySeconds: time.Duration(decaySeconds) * time.Second,
	}
}

// APIThrottle 对应 api 中间件组的 throttle:60,1（每分钟 60 次/IP）。
func APIThrottle(maxAttempts int) goravelhttp.Middleware {
	return &ThrottleMiddleware{
		MaxAttempts:  maxAttempts,
		DecaySeconds: time.Minute,
		OnlyAPI:      true,
	}
}

func (m *ThrottleMiddleware) Signature() string {
	return fmt.Sprintf("neoskin.throttle:%d:%d:%v", m.MaxAttempts, int(m.DecaySeconds.Seconds()), m.OnlyAPI)
}

func (m *ThrottleMiddleware) Handle(ctx goravelhttp.Context) {
	if m.OnlyAPI {
		if facades.Config().GetString("app.env") == "testing" ||
			!strings.HasPrefix(ctx.Request().Path(), "/api") {
			ctx.Request().Next()
			return
		}
	}

	cache := facades.Cache()
	if cache == nil {
		ctx.Request().Next()
		return
	}

	key := rateSignatureKey(ctx.Request().Method(), ctx.Request().Path(), ctx.Request().Ip())

	// Laravel ThrottleRequests::handleRequest：先检查后计数。
	if m.tooManyAttempts(cache, key) {
		m.writeTooManyAttempts(ctx, cache, key)
		return
	}

	attempts := m.hit(cache, key)

	// Laravel addHeaders：成功响应也携带 X-RateLimit-* 头。
	reset := now().Add(m.DecaySeconds).Unix()
	if expiry, ok := m.timerExpiry(cache, key); ok {
		reset = expiry
	}
	ctx.Response().Header("X-RateLimit-Limit", strconv.Itoa(m.MaxAttempts))
	ctx.Response().Header("X-RateLimit-Remaining",
		strconv.Itoa(max(0, m.MaxAttempts-int(attempts))))
	ctx.Response().Header("X-RateLimit-Reset", strconv.FormatInt(reset, 10))

	ctx.Request().Next()
}

// tooManyAttempts 对应 Illuminate\Cache\RateLimiter::tooManyAttempts：
// 计数已达上限且锁定计时器仍存在；计时器不存在说明窗口已过，重置计数放行。
func (m *ThrottleMiddleware) tooManyAttempts(cache contractscache.Driver, key string) bool {
	if cache.GetInt(key, 0) < m.MaxAttempts {
		return false
	}

	if cache.Has(key + ":timer") {
		return true
	}
	cache.Forget(key)

	return false
}

// hit 对应 RateLimiter::increment：Add 计时器（含过期时间戳）→ Add 计数
// 基值 → Increment。Goravel memory 驱动的 Increment 只支持 *int64 指针类型，
// 因此基值与回填值都用指针存放（等价 Add(key, 0, ttl) 的 Laravel 语义）。
func (m *ThrottleMiddleware) hit(cache contractscache.Driver, key string) int64 {
	expiry := now().Add(m.DecaySeconds).Unix()
	cache.Add(key+":timer", expiry, m.DecaySeconds)

	added := cache.Add(key, newInt64(0), m.DecaySeconds)
	hits, err := cache.Increment(key, 1)
	if err == nil && !added && hits <= 1 {
		if putErr := cache.Put(key, newInt64(1), m.DecaySeconds); putErr != nil {
			return hits
		}
	}
	return hits
}

func newInt64(value int64) *int64 {
	pointer := new(int64)
	*pointer = value
	return pointer
}

// writeTooManyAttempts 对应 Laravel buildException 的 429 响应：
// Retry-After 与 X-RateLimit-Reset 取锁定剩余秒数，Remaining 恒 0。
func (m *ThrottleMiddleware) writeTooManyAttempts(ctx goravelhttp.Context, cache contractscache.Driver, key string) {
	retryAfter := m.DecaySeconds
	if expiry, ok := m.timerExpiry(cache, key); ok {
		if remain := expiry - now().Unix(); remain > 0 {
			retryAfter = time.Duration(remain) * time.Second
		} else {
			retryAfter = 0
		}
	}

	ctx.Response().Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
	ctx.Response().Header("X-RateLimit-Limit", strconv.Itoa(m.MaxAttempts))
	ctx.Response().Header("X-RateLimit-Remaining", "0")
	ctx.Response().Header("X-RateLimit-Reset",
		strconv.FormatInt(now().Add(retryAfter).Unix(), 10))
	// Laravel ThrottleRequestsException 的响应体。
	_ = ctx.Response().String(nethttp.StatusTooManyRequests, "Too Many Attempts.").Abort()
}

func (m *ThrottleMiddleware) timerExpiry(cache contractscache.Driver, key string) (int64, bool) {
	expiry := cache.GetInt64(key+":timer", 0)
	if expiry <= 0 {
		return 0, false
	}
	return expiry, true
}

// rateSignatureKey 对应 Laravel Request::fingerprint（路由方法+URI+IP）。
func rateSignatureKey(method, path, ip string) string {
	sum := sha1.Sum([]byte(method + "|" + path + "|" + ip))
	return "throttle:" + hex.EncodeToString(sum[:])
}

func now() time.Time {
	return time.Now()
}
