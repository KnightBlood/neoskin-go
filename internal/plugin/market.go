package plugin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type MarketPackage struct {
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Readme      string         `json:"readme"`
	Require     map[string]any `json:"require"`
	Dist        struct {
		URL string `json:"url"`
	} `json:"dist"`
}

// FetchRegistry 拉取插件市场注册表（支持逗号分隔的多个 URL 与 {lang} 占位）。
func FetchRegistry(registry, lang string) ([]MarketPackage, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	var result []MarketPackage
	for _, raw := range strings.Split(registry, ",") {
		url := strings.TrimSpace(strings.ReplaceAll(raw, "{lang}", lang))
		if url == "" {
			continue
		}
		response, err := client.Get(url)
		if err != nil {
			return nil, err
		}
		body := response.Body
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			_ = body.Close()
			return nil, fmt.Errorf("registry %s returned %d", url, response.StatusCode)
		}
		var payload struct {
			Packages []MarketPackage `json:"packages"`
		}
		decodeErr := json.NewDecoder(body).Decode(&payload)
		_ = body.Close()
		if decodeErr != nil {
			return nil, decodeErr
		}
		result = append(result, payload.Packages...)
	}
	return result, nil
}

// VersionGreater 是 semver 比较的简化版（major.minor.patch，忽略预发布标签）。
func VersionGreater(left, right string) bool {
	return compareVersion(left, right) > 0
}

func compareVersion(left, right string) int {
	parse := func(value string) [3]int {
		value = strings.TrimPrefix(strings.TrimSpace(value), "v")
		if index := strings.IndexAny(value, "-+"); index >= 0 {
			value = value[:index]
		}
		parts := strings.Split(value, ".")
		var result [3]int
		for i := 0; i < 3 && i < len(parts); i++ {
			result[i], _ = strconv.Atoi(parts[i])
		}
		return result
	}
	a, b := parse(left), parse(right)
	for i := 0; i < 3; i++ {
		if a[i] > b[i] {
			return 1
		}
		if a[i] < b[i] {
			return -1
		}
	}
	return 0
}

func (m *Manager) DownloadPackage(url string) error {
	file, err := os.CreateTemp("", "neoskin-market-*.zip")
	if err != nil {
		return err
	}
	path := file.Name()
	_ = file.Close()
	defer os.Remove(path)

	if err := m.Download(url, path); err != nil {
		return err
	}
	return m.ExtractZip(path)
}
