package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"testing"
	"time"

	"neoskin/tests/support"
)

func TestHTTPEndToEnd(t *testing.T) {
	server := support.Start(t)
	base := server.BaseURL

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	request := func(path string) (*http.Response, string) {
		response, err := client.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		return response, string(body)
	}

	cases := []struct {
		path        string
		status      int
		contains    string
		contentType string
	}{
		{"/", 200, "window.blessing", "text/html"},
		{"/auth/login", 200, "csrf-token", "text/html"},
		{"/auth/register", 200, "csrf-token", "text/html"},
		{"/setup", 200, "NeoSkin", "text/html"},
		{"/lang/zh_CN.js", 200, "blessing.i18n =", "application/javascript"},
		{"/api", 200, "blessing_skin", "application/json"},
		{"/skinlib/list", 200, "[", "application/json"},
		{"/avatar/0?png=1", 200, "", "image/png"},
		{"/avatar/0", 200, "", "image/webp"},
		{"/user", 302, "", ""},
	}
	for _, item := range cases {
		response, body := request(item.path)
		if response.StatusCode != item.status {
			t.Fatalf("%s status=%d want %d", item.path, response.StatusCode, item.status)
		}
		if item.contains != "" && !strings.Contains(body, item.contains) {
			t.Fatalf("%s missing %q", item.path, item.contains)
		}
		if item.contentType != "" {
			contentType := response.Header.Get("Content-Type")
			if !strings.HasPrefix(contentType, item.contentType) {
				t.Fatalf("%s content-type=%q want %q", item.path, contentType, item.contentType)
			}
		}
	}

	assertValidationMessages(t, base)
}

// message 为首条字段消息（多错时追加 "(and N more errors)"），字段消息按
// resources/lang/<locale>/validation.yml + attributes 本地化。
func assertValidationMessages(t *testing.T, base string) {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	session := &http.Client{
		Timeout: 10 * time.Second,
		Jar:     jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	page, err := session.Get(base + "/auth/login")
	if err != nil {
		t.Fatalf("GET /auth/login: %v", err)
	}
	pageBody, _ := io.ReadAll(page.Body)
	_ = page.Body.Close()
	match := regexp.MustCompile(`name="csrf-token" content="([^"]+)"`).FindStringSubmatch(string(pageBody))
	if len(match) != 2 {
		t.Fatal("csrf token not found on login page")
	}
	token := match[1]

	cases := []struct {
		name        string
		query       string
		message     string
		fieldErrors string
	}{
		{
			name:  "zh_CN",
			query: "?lang=zh_CN",
			// PHP ValidationException::summarize：首条消息 + "(and 1 more error)"。
			message:     "邮箱或角色名 不能为空。 (and 1 more error)",
			fieldErrors: "邮箱或角色名 不能为空。",
		},
		{
			name:        "en",
			query:       "?lang=en",
			message:     "The email or player name field is required. (and 1 more error)",
			fieldErrors: "The email or player name field is required.",
		},
	}
	for _, item := range cases {
		t.Run("validation_"+item.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, base+"/auth/login"+item.query,
				strings.NewReader("{}"))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json")
			request.Header.Set("X-CSRF-TOKEN", token)
			response, err := session.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if response.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", response.StatusCode, body)
			}
			var payload struct {
				Message string              `json:"message"`
				Errors  map[string][]string `json:"errors"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode: %v body=%s", err, body)
			}
			if payload.Message != item.message {
				t.Fatalf("message=%q want=%q", payload.Message, item.message)
			}
			messages := payload.Errors["identification"]
			if len(messages) == 0 || messages[0] != item.fieldErrors {
				t.Fatalf("errors.identification=%v want=%q", messages, item.fieldErrors)
			}
		})
	}
}
