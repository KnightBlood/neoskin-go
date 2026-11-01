package plugin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionGreater(t *testing.T) {
	cases := []struct {
		left, right string
		want        bool
	}{
		{"1.0.1", "1.0.0", true},
		{"1.1.0", "1.0.9", true},
		{"v2.0.0", "1.9.9", true},
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "1.0.1", false},
	}
	for _, item := range cases {
		if got := VersionGreater(item.left, item.right); got != item.want {
			t.Fatalf("VersionGreater(%q,%q)=%v want %v", item.left, item.right, got, item.want)
		}
	}
}

func TestFetchRegistry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"packages":[{"name":"a/b","version":"1.2.3",
			"dist":{"url":"http://example.com/a.zip"}}]}`))
	}))
	defer server.Close()

	packages, err := FetchRegistry(server.URL, "zh_CN")
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0].Name != "a/b" ||
		packages[0].Version != "1.2.3" || packages[0].Dist.URL == "" {
		t.Fatalf("unexpected packages: %+v", packages)
	}
}
