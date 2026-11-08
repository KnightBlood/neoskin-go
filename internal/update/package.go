package update

import (
	"archive/zip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// Spec 是更新源 JSON 的 spec 版本（与 PHP UpdateController::SPEC 一致）。
	Spec = 2
	// MaxFiles / MaxBytes 与 PHP UpdatePackage 的限制一致。
	MaxFiles = 30000
	MaxBytes = int64(1 << 30)
)

var versionPattern = regexp.MustCompile(`[\'"]version[\'"]\s*=>\s*[\'"]([^\'"]+)[\'"]`)

// Info 是已校验并落盘的运行包元数据（对应 package.json）。
type Info struct {
	ID        string   `json:"id"`
	Version   string   `json:"version"`
	SHA256    string   `json:"sha256"`
	Files     []string `json:"files"`
	Count     int      `json:"count"`
	CreatedAt int64    `json:"created_at"`
}

// Manager 负责运行包的校验、暂存与安装。
type Manager struct {
	// Root 站点根目录（对应 PHP base_path()）。
	Root string
	// Storage 存储根目录（对应 PHP storage_path()）。
	Storage string
	// CurrentVersion 当前版本（对应 config('app.version')）。
	CurrentVersion string
}

func NewManager(root, storage, currentVersion string) *Manager {
	return &Manager{Root: root, Storage: storage, CurrentVersion: currentVersion}
}

func (m *Manager) directory(id string) (string, error) {
	if !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(id) {
		return "", errors.New("更新任务格式错误。")
	}
	return filepath.Join(m.Storage, "app", "system-updates", id), nil
}

// Prepare 校验 ZIP 运行包并把允许的文件解压到临时目录。
func (m *Manager) Prepare(zipPath string) (*Info, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, errors.New("请选择有效的 ZIP 运行包。")
	}
	defer reader.Close()

	if len(reader.File) > MaxFiles {
		return nil, errors.New("更新包文件数超过 30000。")
	}

	names := map[string]*zip.File{}
	seen := map[string]bool{}
	var total int64
	for _, file := range reader.File {
		if err := validateEntryName(file.Name); err != nil {
			return nil, err
		}
		key := strings.ToLower(strings.TrimRight(file.Name, "/"))
		if seen[key] {
			return nil, errors.New("ZIP 包含重复文件路径。")
		}
		seen[key] = true
		if file.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("更新包应使用普通文件和目录。")
		}
		total += int64(file.UncompressedSize64)
		if total > MaxBytes {
			return nil, errors.New("解压大小超过 1 GiB。")
		}
		names[file.Name] = file
	}

	prefix, err := detectPrefix(names)
	if err != nil {
		return nil, err
	}
	for _, required := range []string{
		"artisan", "bootstrap/app.php", "config/app.php", "public/index.php",
		"vendor/autoload.php", "resources/views/assets/app.twig",
	} {
		if names[prefix+required] == nil {
			return nil, fmt.Errorf("运行包缺少 %s。", required)
		}
	}
	compiled := false
	for name := range names {
		if strings.HasPrefix(name, prefix+"public/app/") && strings.HasSuffix(name, ".js") {
			compiled = true
			break
		}
	}
	if !compiled {
		return nil, errors.New("运行包缺少已编译的前端资源。")
	}

	configFile := names[prefix+"config/app.php"]
	configData, err := readEntry(configFile)
	if err != nil {
		return nil, err
	}
	match := versionPattern.FindSubmatch(configData)
	if match == nil {
		return nil, errors.New("无法读取运行包版本号。")
	}
	version := string(match[1])
	if Compare(version, m.CurrentVersion) < 0 {
		return nil, errors.New("请选择当前版本或更高版本的运行包。")
	}

	id := randomID()
	dir, _ := m.directory(id)
	payload := filepath.Join(dir, "payload")
	if err := os.MkdirAll(payload, 0o700); err != nil {
		return nil, err
	}

	files := make([]string, 0, len(names))
	err = func() error {
		for name, file := range names {
			if !strings.HasPrefix(name, prefix) || strings.HasSuffix(name, "/") {
				continue
			}
			relative := strings.TrimPrefix(name, prefix)
			if !allowed(relative) {
				continue
			}
			target := filepath.Join(payload, filepath.FromSlash(relative))
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			source, err := file.Open()
			if err != nil {
				return errors.New("无法读取更新文件。")
			}
			written, err := writeFile(target, source)
			source.Close()
			if err != nil {
				return err
			}
			if written != int64(file.UncompressedSize64) {
				return errors.New("更新文件解压不完整。")
			}
			files = append(files, relative)
		}
		return nil
	}()
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	sort.Strings(files)

	sum, err := fileSHA256(zipPath)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	info := &Info{
		ID: id, Version: version, SHA256: sum, Files: files,
		Count: len(files), CreatedAt: time.Now().Unix(),
	}
	data, err := json.Marshal(info)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	return info, nil
}

// Install 安装已暂存的运行包（带备份与回滚）。
func (m *Manager) Install(id string) error {
	dir, err := m.directory(id)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return errors.New("更新任务已过期或已安装，请重新上传运行包。")
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return errors.New("更新任务已过期或已安装，请重新上传运行包。")
	}
	if info.CreatedAt < time.Now().Add(-time.Hour).Unix() {
		return errors.New("更新任务已过期或已安装，请重新上传运行包。")
	}
	if _, err := os.Stat(filepath.Join(dir, "installed.json")); err == nil {
		return errors.New("更新任务已过期或已安装，请重新上传运行包。")
	}
	if Compare(info.Version, m.CurrentVersion) < 0 {
		return errors.New("请选择当前版本或更高版本的运行包。")
	}

	lockPath := filepath.Join(m.Storage, "app", "system-updates", "install.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errors.New("系统更新正在执行，请稍后重试。")
	}
	_ = lock.Close()
	defer os.Remove(lockPath)

	for _, name := range info.Files {
		if err := m.checkTarget(name); err != nil {
			return err
		}
	}

	changed := make([]string, 0, len(info.Files))
	exists := map[string]bool{}
	restore := func() {
		for i := len(changed) - 1; i >= 0; i-- {
			name := changed[i]
			target := filepath.Join(m.Root, filepath.FromSlash(name))
			if exists[name] {
				_ = copyFile(filepath.Join(dir, "backup", filepath.FromSlash(name)), target)
			} else {
				_ = os.Remove(target)
			}
		}
	}

	for _, name := range info.Files {
		target := filepath.Join(m.Root, filepath.FromSlash(name))
		backup := filepath.Join(dir, "backup", filepath.FromSlash(name))
		_, statErr := os.Stat(target)
		exists[name] = statErr == nil
		if exists[name] {
			if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
				restore()
				return err
			}
			if err := copyFile(target, backup); err != nil {
				restore()
				return errors.New("无法备份更新文件。")
			}
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			restore()
			return err
		}
		if err := publish(filepath.Join(dir, "payload", filepath.FromSlash(name)), target); err != nil {
			restore()
			return err
		}
		changed = append(changed, name)
	}

	installed := map[string]any{"version": info.Version, "installed_at": time.Now().Unix()}
	payload, _ := json.Marshal(installed)
	if err := os.WriteFile(filepath.Join(dir, "installed.json"), payload, 0o600); err != nil {
		restore()
		return err
	}
	return nil
}

// Extract 把 ZIP 直接解压到站点根目录（兼容旧版完整包，无暂存/回滚）。
func (m *Manager) Extract(zipPath string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return errors.New("请选择有效的 ZIP 运行包。")
	}
	defer reader.Close()
	for _, file := range reader.File {
		if err := validateEntryName(file.Name); err != nil {
			return err
		}
		relative := filepath.FromSlash(strings.TrimRight(file.Name, "/"))
		if relative == "" {
			continue
		}
		target := filepath.Join(m.Root, relative)
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
		_, err = writeFile(target, source)
		source.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) checkTarget(name string) error {
	segments := strings.Split(filepath.ToSlash(name), "/")
	current := m.Root
	for _, segment := range segments {
		current = filepath.Join(current, segment)
		if info, err := os.Lstat(current); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("更新目标路径包含符号链接：%s", name)
		}
	}
	target := filepath.Join(m.Root, filepath.FromSlash(name))
	ancestor := target
	for {
		if _, err := os.Stat(ancestor); err == nil {
			break
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			break
		}
		ancestor = parent
	}
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return fmt.Errorf("更新目标目录或文件不可写：%s", name)
	}
	if !writable(ancestor) {
		return fmt.Errorf("更新目标目录或文件不可写：%s", name)
	}
	return nil
}

func writable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return true
	}
	probe, err := os.CreateTemp(path, ".bs-write-*")
	if err != nil {
		return false
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return true
}

func publish(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return errors.New("无法读取更新文件。")
	}
	defer input.Close()
	temp, err := os.CreateTemp(filepath.Dir(target), ".bs-update-*")
	if err != nil {
		return errors.New("无法写入更新文件。")
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := io.Copy(temp, input); err != nil {
		_ = temp.Close()
		return errors.New("无法写入更新文件。")
	}
	_ = temp.Close()
	mode := os.FileMode(0o644)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
	}
	_ = os.Chmod(tempName, mode)
	if err := os.Rename(tempName, target); err != nil {
		return errors.New("无法替换更新文件。")
	}
	return nil
}

func detectPrefix(names map[string]*zip.File) (string, error) {
	prefix := ""
	for name := range names {
		if name != "composer.json" && !(strings.Count(name, "/") == 1 &&
			strings.HasSuffix(name, "/composer.json")) {
			continue
		}
		data, err := readEntry(names[name])
		if err != nil {
			return "", err
		}
		var composer struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &composer) != nil {
			continue
		}
		if composer.Name != "bs-community/blessing-skin-server" {
			continue
		}
		if prefix != "" {
			return "", errors.New("请上传单个系统运行包。")
		}
		prefix = strings.TrimSuffix(name, "composer.json")
	}
	if prefix == "" {
		return "", errors.New("未识别到 NeoSkin 系统运行包。")
	}
	return prefix, nil
}

func validateEntryName(name string) error {
	if strings.ContainsAny(name, "\x00\\") || strings.HasPrefix(name, "/") {
		return errors.New("ZIP 文件路径格式错误。")
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "." || segment == ".." {
			return errors.New("ZIP 文件路径格式错误。")
		}
	}
	if len(name) > 1 && name[1] == ':' {
		return errors.New("ZIP 文件路径格式错误。")
	}
	return nil
}

func allowed(name string) bool {
	name = filepath.ToSlash(name)
	root := name
	if index := strings.IndexByte(name, '/'); index >= 0 {
		root = name[:index]
	}
	switch root {
	case "app", "config", "database", "resources", "routes", "vendor":
		return true
	case "bootstrap":
		return !strings.HasPrefix(name, "bootstrap/cache/")
	case "public":
		for _, reserved := range []string{"public/storage", "public/uploads",
			"public/textures", "public/plugins"} {
			if name == reserved || strings.HasPrefix(name, reserved+"/") {
				return false
			}
		}
		return true
	}
	switch name {
	case "artisan", "composer.json", "composer.lock", "package.json", "index.html",
		"LICENSE", "README.md", "README-zh.md", "BUILD-PHP85.md", "CHANGELOG-PATCH.md":
		return true
	}
	return false
}

func readEntry(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func writeFile(path string, source io.Reader) (int64, error) {
	target, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	written, err := io.Copy(target, source)
	closeErr := target.Close()
	if err != nil {
		return written, err
	}
	return written, closeErr
}

func copyFile(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = io.Copy(output, input)
	closeErr := output.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func randomID() string {
	buffer := make([]byte, 16)
	_, _ = rand.Read(buffer)
	return hex.EncodeToString(buffer)
}

// Compare 比较两个点分版本号（忽略 -/+ 等于后缀，如 6.0.11_patch）。
func Compare(left, right string) int {
	parse := func(value string) [3]int {
		value = strings.TrimPrefix(strings.TrimSpace(value), "v")
		for i := 0; i < len(value); i++ {
			if (value[i] < '0' || value[i] > '9') && value[i] != '.' {
				value = value[:i]
				break
			}
		}
		parts := strings.Split(value, ".")
		var result [3]int
		for i := 0; i < 3 && i < len(parts); i++ {
			digits := parts[i]
			for j := 0; j < len(digits); j++ {
				if digits[j] < '0' || digits[j] > '9' {
					digits = digits[:j]
					break
				}
			}
			result[i], _ = strconv.Atoi(digits)
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
