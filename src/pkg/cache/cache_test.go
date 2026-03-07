package cache

import (
	"archive/zip"
	"bytes"
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

func makeTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(content))
	}
	w.Close()
	return buf.Bytes()
}

func TestManagerUploadZip(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)

	zipData := makeTestZip(t, map[string]string{
		"run.py":         "print('hi')",
		"scripts/lib.py": "x = 1",
	})

	info, err := m.UploadZip("zip-skill", "sha256:zip1", zipData)
	if err != nil {
		t.Fatal(err)
	}
	if info.FileCount != 2 {
		t.Fatalf("expected 2 files, got %d", info.FileCount)
	}

	data, err := os.ReadFile(filepath.Join(dir, "zip-skill", "run.py"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "print('hi')" {
		t.Fatalf("expected print('hi'), got %q", string(data))
	}

	data, err = os.ReadFile(filepath.Join(dir, "zip-skill", "scripts", "lib.py"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "x = 1" {
		t.Fatalf("expected 'x = 1', got %q", string(data))
	}
}

func TestManagerUpdateZip(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)

	// Update non-existent returns nil
	zipData := makeTestZip(t, map[string]string{"f.txt": "x"})
	info, err := m.UpdateZip("nope", "sha256:x", zipData)
	if err != nil {
		t.Fatal(err)
	}
	if info != nil {
		t.Fatal("expected nil for non-existent skill")
	}

	// Upload then update with zip
	m.Upload("s1", "sha256:v1", map[string]string{
		"a.txt": base64.StdEncoding.EncodeToString([]byte("original")),
	})

	zipData = makeTestZip(t, map[string]string{
		"a.txt": "updated",
		"b.txt": "new",
	})
	info, err = m.UpdateZip("s1", "sha256:v2", zipData)
	if err != nil {
		t.Fatal(err)
	}
	if info.Hash != "sha256:v2" {
		t.Fatalf("expected hash v2, got %s", info.Hash)
	}
	if info.FileCount != 2 {
		t.Fatalf("expected 2 files, got %d", info.FileCount)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "s1", "a.txt"))
	if string(data) != "updated" {
		t.Fatalf("expected 'updated', got %q", string(data))
	}
}
