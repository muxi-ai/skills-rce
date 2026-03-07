package cache

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestManagerUploadAndGet(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	if info := m.Get("test-skill"); info != nil {
		t.Fatal("expected nil for uncached skill")
	}

	files := map[string]string{
		"hello.txt":         base64.StdEncoding.EncodeToString([]byte("hello world")),
		"scripts/run.py":    base64.StdEncoding.EncodeToString([]byte("print('hi')")),
	}

	info, err := m.Upload("test-skill", "sha256:abc123", files)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "test-skill" || info.Hash != "sha256:abc123" || info.FileCount != 2 {
		t.Fatalf("unexpected info: %+v", info)
	}

	got := m.Get("test-skill")
	if got == nil || got.Hash != "sha256:abc123" {
		t.Fatalf("expected cached skill, got %+v", got)
	}

	data, err := os.ReadFile(filepath.Join(dir, "test-skill", "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Fatalf("expected 'hello world', got %q", string(data))
	}
}

func TestManagerUpdate(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Update non-existent skill returns nil
	info, err := m.Update("nope", "sha256:x", map[string]string{"a.txt": base64.StdEncoding.EncodeToString([]byte("a"))})
	if err != nil {
		t.Fatal(err)
	}
	if info != nil {
		t.Fatal("expected nil for non-existent skill update")
	}

	// Upload then update
	files := map[string]string{
		"a.txt": base64.StdEncoding.EncodeToString([]byte("original")),
	}
	m.Upload("s1", "sha256:v1", files)

	updated, err := m.Update("s1", "sha256:v2", map[string]string{
		"a.txt": base64.StdEncoding.EncodeToString([]byte("updated")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Hash != "sha256:v2" {
		t.Fatalf("expected hash v2, got %s", updated.Hash)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "s1", "a.txt"))
	if string(data) != "updated" {
		t.Fatalf("expected 'updated', got %q", string(data))
	}
}

func TestManagerDelete(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)

	if m.Delete("nope") {
		t.Fatal("expected false for non-existent delete")
	}

	m.Upload("s1", "sha256:x", map[string]string{
		"f.txt": base64.StdEncoding.EncodeToString([]byte("data")),
	})

	if !m.Delete("s1") {
		t.Fatal("expected true for existing delete")
	}
	if m.Get("s1") != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestManagerList(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)

	if len(m.List()) != 0 {
		t.Fatal("expected empty list")
	}

	m.Upload("a", "sha256:1", map[string]string{"f": base64.StdEncoding.EncodeToString([]byte("x"))})
	m.Upload("b", "sha256:2", map[string]string{"f": base64.StdEncoding.EncodeToString([]byte("y"))})

	list := m.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(list))
	}
}
