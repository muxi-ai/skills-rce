package cache

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type SkillInfo struct {
	Name      string
	Hash      string
	FileCount int
}

type Manager struct {
	baseDir string
	mu      sync.RWMutex
	skills  map[string]*SkillInfo
}

func NewManager(baseDir string) (*Manager, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	return &Manager{
		baseDir: baseDir,
		skills:  make(map[string]*SkillInfo),
	}, nil
}

func (m *Manager) skillDir(name string) string {
	return filepath.Join(m.baseDir, name)
}

func (m *Manager) Get(name string) *SkillInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.skills[name]
}

func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.skills))
	for name := range m.skills {
		names = append(names, name)
	}
	return names
}

func (m *Manager) UploadZip(name, hash string, zipData []byte) (*SkillInfo, error) {
	dir := m.skillDir(name)
	if err := os.RemoveAll(dir); err != nil {
		return nil, fmt.Errorf("clean skill dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create skill dir: %w", err)
	}

	count, err := m.extractZip(dir, zipData)
	if err != nil {
		return nil, err
	}

	info := &SkillInfo{Name: name, Hash: hash, FileCount: count}
	m.mu.Lock()
	m.skills[name] = info
	m.mu.Unlock()
	return info, nil
}

func (m *Manager) Upload(name, hash string, files map[string]string) (*SkillInfo, error) {
	dir := m.skillDir(name)
	if err := os.RemoveAll(dir); err != nil {
		return nil, fmt.Errorf("clean skill dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create skill dir: %w", err)
	}

	if err := m.writeFiles(dir, files); err != nil {
		return nil, err
	}

	info := &SkillInfo{Name: name, Hash: hash, FileCount: len(files)}
	m.mu.Lock()
	m.skills[name] = info
	m.mu.Unlock()
	return info, nil
}

func (m *Manager) UpdateZip(name, hash string, zipData []byte) (*SkillInfo, error) {
	m.mu.RLock()
	existing := m.skills[name]
	m.mu.RUnlock()
	if existing == nil {
		return nil, nil
	}

	dir := m.skillDir(name)
	if _, err := m.extractZip(dir, zipData); err != nil {
		return nil, err
	}

	count := m.countFiles(dir)
	info := &SkillInfo{Name: name, Hash: hash, FileCount: count}
	m.mu.Lock()
	m.skills[name] = info
	m.mu.Unlock()
	return info, nil
}

func (m *Manager) Update(name, hash string, files map[string]string) (*SkillInfo, error) {
	m.mu.RLock()
	existing := m.skills[name]
	m.mu.RUnlock()
	if existing == nil {
		return nil, nil
	}

	dir := m.skillDir(name)
	if err := m.writeFiles(dir, files); err != nil {
		return nil, err
	}

	count := m.countFiles(dir)
	info := &SkillInfo{Name: name, Hash: hash, FileCount: count}
	m.mu.Lock()
	m.skills[name] = info
	m.mu.Unlock()
	return info, nil
}

func (m *Manager) Delete(name string) bool {
	m.mu.Lock()
	_, exists := m.skills[name]
	if exists {
		delete(m.skills, name)
	}
	m.mu.Unlock()

	if exists {
		os.RemoveAll(m.skillDir(name))
	}
	return exists
}

func (m *Manager) Dir(name string) string {
	return m.skillDir(name)
}

func (m *Manager) writeFiles(dir string, files map[string]string) error {
	for relPath, b64Content := range files {
		data, err := base64.StdEncoding.DecodeString(b64Content)
		if err != nil {
			return fmt.Errorf("decode %s: %w", relPath, err)
		}
		fullPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("create parent dirs for %s: %w", relPath, err)
		}
		if err := os.WriteFile(fullPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", relPath, err)
		}
	}
	return nil
}

func (m *Manager) countFiles(dir string) int {
	count := 0
	filepath.Walk(dir, func(_ string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}

func (m *Manager) extractZip(dir string, zipData []byte) (int, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return 0, fmt.Errorf("open zip: %w", err)
	}

	count := 0
	for _, f := range r.File {
		// Sanitize: reject absolute paths and path traversal
		name := filepath.Clean(f.Name)
		if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "..") {
			continue
		}
		fullPath := filepath.Join(dir, name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fullPath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return 0, fmt.Errorf("create parent dirs for %s: %w", name, err)
		}

		rc, err := f.Open()
		if err != nil {
			return 0, fmt.Errorf("open zip entry %s: %w", name, err)
		}

		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return 0, fmt.Errorf("read zip entry %s: %w", name, err)
		}

		if err := os.WriteFile(fullPath, data, 0644); err != nil {
			return 0, fmt.Errorf("write %s: %w", name, err)
		}
		count++
	}
	return count, nil
}
