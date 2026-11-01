package plugin

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"neoskin/internal/service"
)

type Manifest struct {
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Readme      string         `json:"readme"`
	Enchants    map[string]any `json:"enchants"`
	Require     any            `json:"require"`
	Conflicts   any            `json:"conflicts"`
}

type Plugin struct {
	Dir      string
	Manifest Manifest
	Enabled  bool
}

type Manager struct {
	Directory string
	Option    *service.Option
}

func NewManager(directory string, option *service.Option) *Manager {
	if directory == "" {
		directory = os.Getenv("NEOSKIN_PLUGINS_DIR")
	}
	if directory == "" {
		directory = "plugins"
	}
	return &Manager{Directory: directory, Option: option}
}

func (m *Manager) enabledSet() map[string]bool {
	result := map[string]bool{}
	raw := m.Option.Get("plugins_enabled")
	var entries []map[string]any
	if json.Unmarshal([]byte(raw), &entries) == nil {
		for _, entry := range entries {
			if name, ok := entry["name"].(string); ok {
				result[name] = true
			}
		}
	}
	return result
}

func (m *Manager) saveEnabled(enabled map[string]bool) error {
	names := make([]string, 0, len(enabled))
	for name := range enabled {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]map[string]any, 0, len(names))
	for _, name := range names {
		entries = append(entries, map[string]any{"name": name})
	}
	encoded, _ := json.Marshal(entries)
	return m.Option.Set("plugins_enabled", string(encoded))
}

// All 扫描插件目录并返回清单。
func (m *Manager) All() ([]Plugin, error) {
	entries, err := os.ReadDir(m.Directory)
	if err != nil {
		if os.IsNotExist(err) {
			return []Plugin{}, nil
		}
		return nil, err
	}
	enabled := m.enabledSet()
	var result []Plugin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(m.Directory, entry.Name(), "package.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest Manifest
		if json.Unmarshal(data, &manifest) != nil {
			continue
		}
		if manifest.Name == "" {
			manifest.Name = entry.Name()
		}
		result = append(result, Plugin{
			Dir: filepath.Join(m.Directory, entry.Name()), Manifest: manifest,
			Enabled: enabled[manifest.Name],
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Manifest.Name < result[j].Manifest.Name })
	return result, nil
}

func (m *Manager) Find(name string) (*Plugin, error) {
	all, err := m.All()
	if err != nil {
		return nil, err
	}
	for _, item := range all {
		if item.Manifest.Name == name {
			return &item, nil
		}
	}
	return nil, nil
}

func (m *Manager) SetEnabled(name string, enabled bool) error {
	plugins := m.enabledSet()
	if enabled {
		plugins[name] = true
	} else {
		delete(plugins, name)
	}
	return m.saveEnabled(plugins)
}

func (m *Manager) Delete(name string) error {
	item, err := m.Find(name)
	if err != nil || item == nil {
		return err
	}
	_ = m.SetEnabled(name, false)
	return os.RemoveAll(item.Dir)
}

func (m *Manager) Readme(name string) (string, error) {
	item, err := m.Find(name)
	if err != nil || item == nil {
		return "", err
	}
	readme := item.Manifest.Readme
	if readme == "" {
		for _, candidate := range []string{"README.md", "readme.md", "README"} {
			path := filepath.Join(item.Dir, candidate)
			if _, err := os.Stat(path); err == nil {
				data, readErr := os.ReadFile(path)
				return string(data), readErr
			}
		}
		return "", nil
	}
	data, err := os.ReadFile(filepath.Join(item.Dir, readme))
	return string(data), err
}

func (m *Manager) ExtractZip(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()

	root := filepath.Clean(m.Directory)
	for _, file := range reader.File {
		target := filepath.Join(root, file.Name)
		if !strings.HasPrefix(filepath.Clean(target), root+string(os.PathSeparator)) {
			return fmt.Errorf("illegal path in zip: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		destination, err := os.Create(target)
		if err != nil {
			_ = source.Close()
			return err
		}
		_, copyErr := io.Copy(destination, source)
		_ = destination.Close()
		_ = source.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func (m *Manager) Download(url, target string) error {
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("download failed: %d", response.StatusCode)
	}
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, response.Body)
	return err
}
