package captcha

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func withVerifyURL(url string, t *testing.T) func() {
	previous := verifyURL
	previousClient := verifyClient
	verifyURL = url
	verifyClient = &http.Client{Timeout: 2 * time.Second}
	return func() {
		verifyURL = previous
		verifyClient = previousClient
	}
}

func TestVerifyRecaptcha(t *testing.T) {
	var gotSecret, gotResponse, gotContentType string
	var calls int
	var mutex sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()
		calls++
		gotContentType = r.Header.Get("Content-Type")
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		gotSecret = r.FormValue("secret")
		gotResponse = r.FormValue("response")
		w.Header().Set("Content-Type", "application/json")
		if gotResponse == "valid-token" {
			_, _ = w.Write([]byte(`{"success":true}`))
		} else {
			_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
		}
	}))
	defer server.Close()
	restore := withVerifyURL(server.URL, t)
	defer restore()

	if VerifyRecaptcha("secret-key", "valid-token") != true {
		t.Fatal("expected success for valid token")
	}
	mutex.Lock()
	defer mutex.Unlock()
	if gotSecret != "secret-key" || gotResponse != "valid-token" {
		t.Fatalf("unexpected form values: secret=%q response=%q", gotSecret, gotResponse)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("unexpected content type: %q", gotContentType)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}

	// 失败 token / 空值 / 无 secret 时一律为 false。
	if VerifyRecaptcha("secret-key", "bad-token") {
		t.Fatal("expected failure for invalid token")
	}
	if VerifyRecaptcha("secret-key", "  ") {
		t.Fatal("expected failure for blank token")
	}
	if VerifyRecaptcha("", "valid-token") {
		t.Fatal("expected failure without secret")
	}
	// 不可达地址。
	restore2 := withVerifyURL("http://127.0.0.1:1/none", t)
	defer restore2()
	if VerifyRecaptcha("secret-key", "valid-token") {
		t.Fatal("expected failure on network error")
	}
}

func TestLoginFailTracker(t *testing.T) {
	tracker := newLoginFailTracker()
	previousNow := trackerNow
	trackerNow = func() time.Time { return time.Unix(nowUnix, 0) }
	defer func() { trackerNow = previousNow }()

	ip := "203.0.113.7"
	if tracker.Count(ip) != 0 {
		t.Fatal("expected zero failures for unknown IP")
	}
	if tracker.Count("") != 0 {
		t.Fatal("expected zero failures for blank IP")
	}

	for expected := 1; expected <= 5; expected++ {
		if got := tracker.Increment(ip); got != expected {
			t.Fatalf("increment got=%d want=%d", got, expected)
		}
		if got := tracker.Count(ip); got != expected {
			t.Fatalf("count got=%d want=%d", got, expected)
		}
	}

	// 计数不跨 IP。
	if got := tracker.Count("203.0.113.8"); got != 0 {
		t.Fatalf("expected other IP to be zero, got %d", got)
	}

	// 过期后归零（TTL 1 小时）。
	nowUnix += int64((time.Hour + time.Second).Seconds())
	if got := tracker.Count(ip); got != 0 {
		t.Fatalf("expected expired counter to be zero, got %d", got)
	}

	if got := tracker.Increment(ip); got != 1 {
		t.Fatalf("increment after expiry got=%d want=1", got)
	}

	tracker.Reset(ip)
	if got := tracker.Count(ip); got != 0 {
		t.Fatalf("expected reset counter to be zero, got %d", got)
	}
	tracker.Reset("255.255.255.254") // 不存在的键，不应 panic
}

var nowUnix int64 = 1730000000
