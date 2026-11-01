package storage

import (
	"os"
	"path/filepath"
	"time"
)

// Local 是纹理文件存储（对应 PHP storage 盘 textures）。
type Local struct {
	Root string
}

func NewLocal(root string) *Local {
	if root == "" {
		root = "storage"
	}

	return &Local{Root: root}
}

func (s *Local) path(hash string) string {
	return filepath.Join(s.Root, "textures", hash)
}

func (s *Local) Exists(hash string) bool {
	info, err := os.Stat(s.path(hash))
	return err == nil && !info.IsDir()
}

func (s *Local) Read(hash string) ([]byte, error) {
	return os.ReadFile(s.path(hash))
}

func (s *Local) Write(hash string, data []byte) error {
	path := s.path(hash)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// ModTime 返回纹理文件的最后修改时间（对应 Storage::lastModified）。
func (s *Local) ModTime(hash string) (time.Time, error) {
	info, err := os.Stat(s.path(hash))
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func (s *Local) Delete(hash string) error {
	err := os.Remove(s.path(hash))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
