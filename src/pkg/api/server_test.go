package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/muxi-ai/skills-rce/pkg/cache"
	"github.com/muxi-ai/skills-rce/pkg/config"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	return setupTestServerWithToken(t, "")
}

func setupTestServerWithToken(t *testing.T, token string) *Server {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		CacheDir:       dir,
		DefaultTimeout: 10,
		MaxTimeout:     30,
		MaxOutputBytes: 100 * 1024,
		AuthToken:      token,
	}
	cm, err := cache.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	logger := zerolog.Nop()
	return NewServer(cfg, cm, &logger, "test")
}

func TestHealthEndpoint(t *testing.T) {
	s := setupTestServer(t)
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "healthy" {
		t.Fatalf("expected healthy, got %v", resp["status"])
	}
	if resp["version"] != "test" {
		t.Fatalf("expected version test, got %v", resp["version"])
	}
}

func TestRunEndpoint(t *testing.T) {
	s := setupTestServer(t)
	body := map[string]interface{}{
		"id":       "t1",
		"language": "python",
		"code":     "print('hello')",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/run", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "success" {
		t.Fatalf("expected success, got %v: %v", resp["status"], resp["stderr"])
	}
}

func TestRunEndpointValidation(t *testing.T) {
	s := setupTestServer(t)
	body := map[string]interface{}{"id": "t1"}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/run", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSkillCRUD(t *testing.T) {
	s := setupTestServer(t)

	// GET non-existent skill
	req := httptest.NewRequest("GET", "/skill/my-skill", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["cached"] != false {
		t.Fatalf("expected not cached")
	}

	// POST upload skill
	files := map[string]string{
		"run.py": base64.StdEncoding.EncodeToString([]byte("print('skill')")),
	}
	body := map[string]interface{}{
		"hash":  "sha256:" + "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		"files": files,
	}
	b, _ := json.Marshal(body)
	req = httptest.NewRequest("POST", "/skill/my-skill", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// GET cached skill
	req = httptest.NewRequest("GET", "/skill/my-skill", nil)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["cached"] != true {
		t.Fatalf("expected cached")
	}

	// PATCH update skill
	body = map[string]interface{}{
		"hash":  "sha256:" + "b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3",
		"files": map[string]string{"run.py": base64.StdEncoding.EncodeToString([]byte("print('updated')"))},
	}
	b, _ = json.Marshal(body)
	req = httptest.NewRequest("PATCH", "/skill/my-skill", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// DELETE skill
	req = httptest.NewRequest("DELETE", "/skill/my-skill", nil)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 on delete, got %d", w.Code)
	}

	// DELETE again -> 404
	req = httptest.NewRequest("DELETE", "/skill/my-skill", nil)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSkillRunEndpoint(t *testing.T) {
	s := setupTestServer(t)

	// Upload a skill with a script
	files := map[string]string{
		"run.py": base64.StdEncoding.EncodeToString([]byte("print('from skill')")),
	}
	body := map[string]interface{}{
		"hash":  "sha256:" + "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		"files": files,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/skill/test-skill", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("upload failed: %d %s", w.Code, w.Body.String())
	}

	// Run against cached skill
	runBody := map[string]interface{}{
		"id":      "run-1",
		"command": "python3 run.py",
	}
	b, _ = json.Marshal(runBody)
	req = httptest.NewRequest("POST", "/skill/test-skill/run", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "success" {
		t.Fatalf("expected success, got %v: %v", resp["status"], resp["stderr"])
	}
}

func TestSkillRunNotCached(t *testing.T) {
	s := setupTestServer(t)
	body := map[string]interface{}{
		"id":      "r1",
		"command": "echo hi",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/skill/nonexistent/run", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAuthHealthBypassesToken(t *testing.T) {
	s := setupTestServerWithToken(t, "secret-token")

	// /health should work without auth
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 for /health without token, got %d", w.Code)
	}
}

func TestAuthRejectsWithoutToken(t *testing.T) {
	s := setupTestServerWithToken(t, "secret-token")

	body := map[string]interface{}{
		"id": "t1", "language": "python", "code": "print(1)",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/run", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401 without token, got %d", w.Code)
	}
}

func TestAuthRejectsWrongToken(t *testing.T) {
	s := setupTestServerWithToken(t, "secret-token")

	body := map[string]interface{}{
		"id": "t1", "language": "python", "code": "print(1)",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/run", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401 with wrong token, got %d", w.Code)
	}
}

func TestAuthAcceptsValidToken(t *testing.T) {
	s := setupTestServerWithToken(t, "secret-token")

	body := map[string]interface{}{
		"id": "t1", "language": "python", "code": "print(1)",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/run", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-token")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 with valid token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPatchNotCached(t *testing.T) {
	s := setupTestServer(t)
	body := map[string]interface{}{
		"hash":  "sha256:" + "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		"files": map[string]string{"f.txt": base64.StdEncoding.EncodeToString([]byte("x"))},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PATCH", "/skill/nonexistent", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
