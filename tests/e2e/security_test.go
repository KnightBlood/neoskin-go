package e2e

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"neoskin/internal/db"
	"neoskin/internal/model"
	"neoskin/internal/php"
	"neoskin/internal/repo"
	"neoskin/tests/support"
)

const rememberCookieName = "remember_web_59ba36addc2b2f9401580f014c7f58ea4e30989d"

// TestSecurityFlows 覆盖认证/授权/CSRF/remember cookie 的关键安全语义。
func TestSecurityFlows(t *testing.T) {
	server := support.Start(t)
	base := server.BaseURL
	seedSecurityUsers(t, server.DBPath)

	t.Run("guest_access", func(t *testing.T) {
		client := newSession(t)

		// PHP authorize 中间件：未登录访问后台/用户页面重定向登录页。
		for _, path := range []string{"/admin", "/admin/users", "/admin/update", "/user", "/user/inbox"} {
			response, _ := get(t, client, base+path)
			if response.StatusCode != http.StatusFound ||
				response.Header.Get("Location") != "/auth/login" {
				t.Fatalf("%s status=%d location=%q want 302 /auth/login",
					path, response.StatusCode, response.Header.Get("Location"))
			}
		}

		// API 未登录保持 Laravel 的 JSON 401。
		response, body := get(t, client, base+"/api/user")
		if response.StatusCode != http.StatusUnauthorized ||
			!strings.Contains(body, `"Unauthenticated."`) {
			t.Fatalf("/api/user status=%d body=%s", response.StatusCode, body)
		}
	})

	t.Run("csrf_required", func(t *testing.T) {
		client := newSession(t)
		_ = csrfToken(t, client, base+"/auth/login")

		// 有 session 但不带 CSRF token：Laravel 返回 419。
		response, _ := postJSON(t, client, base+"/auth/login",
			`{"identification":"a@example.com","password":"secret123"}`, "")
		if response.StatusCode != 419 {
			t.Fatalf("missing csrf status=%d want 419", response.StatusCode)
		}

		// 携带 token 后进入正常校验流程（空字段 → 422）。
		token := csrfToken(t, client, base+"/auth/login")
		response, _ = postJSON(t, client, base+"/auth/login", `{}`, token)
		if response.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("with csrf status=%d want 422", response.StatusCode)
		}
	})

	t.Run("login_rotates_session_and_logout", func(t *testing.T) {
		client := newSession(t)
		token := csrfToken(t, client, base+"/auth/login")
		before := sessionCookieValue(t, client, base)

		response, body := postJSON(t, client, base+"/auth/login",
			`{"identification":"admin@example.com","password":"secret123","keep":false}`, token)
		if response.StatusCode != http.StatusOK || !strings.Contains(body, `"code":0`) {
			t.Fatalf("login status=%d body=%s", response.StatusCode, body)
		}
		after := sessionCookieValue(t, client, base)
		if after == "" || after == before {
			t.Fatalf("session id not rotated: before=%q after=%q", before, after)
		}

		if response, _ := get(t, client, base+"/user"); response.StatusCode != http.StatusOK {
			t.Fatalf("GET /user after login=%d", response.StatusCode)
		}
		if response, _ := get(t, client, base+"/admin"); response.StatusCode != http.StatusOK {
			t.Fatalf("GET /admin as admin=%d", response.StatusCode)
		}

		// 登出后会话失效。
		logoutToken := csrfToken(t, client, base+"/user")
		if response, body := postJSON(t, client, base+"/auth/logout", `{}`, logoutToken); response.StatusCode != http.StatusOK {
			t.Fatalf("logout status=%d body=%s", response.StatusCode, body)
		}
		if response, _ := get(t, client, base+"/user"); response.StatusCode != http.StatusFound {
			t.Fatalf("GET /user after logout=%d want 302", response.StatusCode)
		}
	})

	t.Run("remember_cookie_tampering", func(t *testing.T) {
		client := newSession(t)
		token := csrfToken(t, client, base+"/auth/login")
		response, body := postJSON(t, client, base+"/auth/login",
			`{"identification":"admin@example.com","password":"secret123","keep":true}`, token)
		if response.StatusCode != http.StatusOK || !strings.Contains(body, `"code":0`) {
			t.Fatalf("login keep status=%d body=%s", response.StatusCode, body)
		}

		var remember *http.Cookie
		for _, cookie := range response.Cookies() {
			if strings.HasPrefix(cookie.Name, "remember_web_") {
				remember = cookie
			}
		}
		if remember == nil {
			t.Fatal("remember cookie not issued")
		}
		if !remember.HttpOnly || remember.Value == "" {
			t.Fatalf("remember cookie attributes: HttpOnly=%v valueEmpty=%v",
				remember.HttpOnly, remember.Value == "")
		}

		// 仅凭有效 remember cookie（无 session）恢复登录态。
		recaller := newSession(t)
		baseURL, _ := url.Parse(base)
		recaller.Jar.SetCookies(baseURL, []*http.Cookie{{
			Name: rememberCookieName, Value: remember.Value, Path: "/",
		}})
		if response, _ := get(t, recaller, base+"/user"); response.StatusCode != http.StatusOK {
			t.Fatalf("remember restore status=%d want 200", response.StatusCode)
		}

		// 篡改 HMAC 段后必须被拒绝。
		last := remember.Value[len(remember.Value)-1]
		replacement := byte('A')
		if last == 'A' {
			replacement = 'B'
		}
		tampered := remember.Value[:len(remember.Value)-1] + string(replacement)
		forged := newSession(t)
		forged.Jar.SetCookies(baseURL, []*http.Cookie{{
			Name: rememberCookieName, Value: tampered, Path: "/",
		}})
		if response, _ := get(t, forged, base+"/user"); response.StatusCode != http.StatusFound {
			t.Fatalf("tampered remember status=%d want 302", response.StatusCode)
		}
	})

	t.Run("non_admin_forbidden", func(t *testing.T) {
		client := newSession(t)
		token := csrfToken(t, client, base+"/auth/login")
		response, body := postJSON(t, client, base+"/auth/login",
			`{"identification":"user@example.com","password":"secret123","keep":false}`, token)
		if response.StatusCode != http.StatusOK || !strings.Contains(body, `"code":0`) {
			t.Fatalf("login normal status=%d body=%s", response.StatusCode, body)
		}
		for _, path := range []string{"/admin", "/admin/users", "/admin/users/list"} {
			response, _ := get(t, client, base+path)
			if response.StatusCode != http.StatusForbidden {
				t.Fatalf("%s as normal user status=%d want 403", path, response.StatusCode)
			}
		}
	})
}

func newSession(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{
		Timeout: 15 * time.Second,
		Jar:     jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func get(t *testing.T, client *http.Client, target string) (*http.Response, string) {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	return response, string(body)
}

func postJSON(t *testing.T, client *http.Client, target, body, token string) (*http.Response, string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if token != "" {
		request.Header.Set("X-CSRF-TOKEN", token)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}
	payload, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	return response, string(payload)
}

// csrfToken 打开页面并从 <meta name="csrf-token"> 提取 token。
func csrfToken(t *testing.T, client *http.Client, target string) string {
	t.Helper()
	_, body := get(t, client, target)
	token := metaCSRFToken(body)
	if token == "" {
		t.Fatalf("csrf token missing on %s", target)
	}
	return token
}

func sessionCookieValue(t *testing.T, client *http.Client, base string) string {
	t.Helper()
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range client.Jar.Cookies(parsed) {
		if strings.HasSuffix(cookie.Name, "_session") {
			return cookie.Value
		}
	}
	return ""
}

// seedSecurityUsers 直接写 SQLite 建两个账号（管理员 / 普通用户）。
func seedSecurityUsers(t *testing.T, dbPath string) {
	t.Helper()
	conn, dialect, err := db.Open(db.Config{Connection: "sqlite", Database: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	users := repo.NewUserRepository(conn, dialect)
	password, err := php.HashPassword("secret123", 4)
	if err != nil {
		t.Fatal(err)
	}
	adminUID, err := users.Insert("admin@example.com", "Admin", password, "127.0.0.1", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if err := users.UpdatePermission(adminUID, model.PermissionAdmin); err != nil {
		t.Fatal(err)
	}
	if err := users.UpdateVerified(adminUID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := users.Insert("user@example.com", "User", password, "127.0.0.1", 1000); err != nil {
		t.Fatal(err)
	}
}

// metaCSRFToken 与 assertValidationMessages 共用同一段 meta 解析逻辑。
func metaCSRFToken(body string) string {
	const prefix = `name="csrf-token" content="`
	index := strings.Index(body, prefix)
	if index < 0 {
		return ""
	}
	rest := body[index+len(prefix):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}
