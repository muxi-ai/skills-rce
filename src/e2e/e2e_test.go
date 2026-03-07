package e2e

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/muxi-ai/skills-rce/pkg/api"
	"github.com/muxi-ai/skills-rce/pkg/cache"
	"github.com/muxi-ai/skills-rce/pkg/config"
)

func startServer(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		CacheDir:       dir,
		DefaultTimeout: 30,
		MaxTimeout:     60,
		MaxOutputBytes: 100 * 1024,
	}
	cm, err := cache.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	logger := zerolog.Nop()
	srv := api.NewServer(cfg, cm, &logger, "e2e-test")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	httpSrv := &http.Server{Handler: srv.Handler()}
	go httpSrv.Serve(listener)

	// Wait for server to be ready
	for i := 0; i < 50; i++ {
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return baseURL, func() { httpSrv.Close() }
}

func postJSON(t *testing.T, url string, body interface{}) map[string]interface{} {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	result["_status_code"] = float64(resp.StatusCode)
	return result
}

func getJSON(t *testing.T, url string) map[string]interface{} {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	result["_status_code"] = float64(resp.StatusCode)
	return result
}

func deleteReq(t *testing.T, url string) map[string]interface{} {
	t.Helper()
	req, _ := http.NewRequest("DELETE", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	result["_status_code"] = float64(resp.StatusCode)
	return result
}

func TestE2EHealth(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	resp := getJSON(t, base+"/health")
	if resp["status"] != "healthy" {
		t.Fatalf("expected healthy, got %v", resp["status"])
	}
	if resp["version"] != "e2e-test" {
		t.Fatalf("expected version e2e-test, got %v", resp["version"])
	}
}

func TestE2EAdHocPython(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	resp := postJSON(t, base+"/run", map[string]interface{}{
		"id":       "e2e-1",
		"language": "python",
		"code":     "print(2 ** 10)",
	})

	if resp["status"] != "success" {
		t.Fatalf("expected success: %v", resp["stderr"])
	}
	if resp["stdout"] != "1024\n" {
		t.Fatalf("expected '1024\\n', got %q", resp["stdout"])
	}
}

func TestE2EAdHocWithFilesAndArtifacts(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	code := `
import csv, json
with open('data.csv') as f:
    rows = list(csv.DictReader(f))
with open('result.json', 'w') as f:
    json.dump(rows, f)
print('done')
`
	csvData := base64.StdEncoding.EncodeToString([]byte("name,age\nalice,30\nbob,25\n"))

	resp := postJSON(t, base+"/run", map[string]interface{}{
		"id":       "e2e-files",
		"language": "python",
		"code":     code,
		"files":    map[string]string{"data.csv": csvData},
	})

	if resp["status"] != "success" {
		t.Fatalf("expected success: %v", resp["stderr"])
	}
	artifacts := resp["artifacts"].([]interface{})
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}
	art := artifacts[0].(map[string]interface{})
	if art["name"] != "result.json" {
		t.Fatalf("expected result.json, got %v", art["name"])
	}
}

func TestE2EAdHocTimeout(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	resp := postJSON(t, base+"/run", map[string]interface{}{
		"id":       "e2e-timeout",
		"language": "python",
		"code":     "import time; time.sleep(30)",
		"timeout":  1,
	})

	if resp["status"] != "timeout" {
		t.Fatalf("expected timeout, got %v", resp["status"])
	}
}

func TestE2EAdHocError(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	resp := postJSON(t, base+"/run", map[string]interface{}{
		"id":       "e2e-err",
		"language": "python",
		"code":     "raise Exception('boom')",
	})

	if resp["status"] != "error" {
		t.Fatalf("expected error, got %v", resp["status"])
	}
}

func TestE2ESkillFullLifecycle(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	skillURL := base + "/skill/test-e2e"
	hash1 := "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	hash2 := "sha256:b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3"

	// 1. Not cached yet
	resp := getJSON(t, skillURL)
	if resp["cached"] != false {
		t.Fatal("expected not cached initially")
	}

	// 2. Upload
	scriptCode := base64.StdEncoding.EncodeToString([]byte(`
import sys
name = "world"
if len(sys.argv) > 1:
    name = open(sys.argv[1]).read().strip()
print(f"hello {name}")
with open("greeting.txt", "w") as f:
    f.write(f"hello {name}\n")
`))
	resp = postJSON(t, skillURL, map[string]interface{}{
		"hash":  hash1,
		"files": map[string]string{"greet.py": scriptCode},
	})
	if resp["status"] != "cached" {
		t.Fatalf("expected cached, got %v", resp)
	}

	// 3. Verify cached
	resp = getJSON(t, skillURL)
	if resp["cached"] != true {
		t.Fatal("expected cached after upload")
	}
	if resp["hash"] != hash1 {
		t.Fatalf("expected hash %s, got %v", hash1, resp["hash"])
	}

	// 4. Run without input files
	resp = postJSON(t, skillURL+"/run", map[string]interface{}{
		"id":      "run-no-input",
		"command": "python3 greet.py",
	})
	if resp["status"] != "success" {
		t.Fatalf("expected success: %v", resp["stderr"])
	}
	if resp["stdout"] != "hello world\n" {
		t.Fatalf("expected 'hello world\\n', got %q", resp["stdout"])
	}
	artifacts := resp["artifacts"].([]interface{})
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}

	// 5. Run with input file
	nameFile := base64.StdEncoding.EncodeToString([]byte("alice"))
	resp = postJSON(t, skillURL+"/run", map[string]interface{}{
		"id":          "run-with-input",
		"command":     "python3 greet.py name.txt",
		"input_files": map[string]string{"name.txt": nameFile},
	})
	if resp["status"] != "success" {
		t.Fatalf("expected success: %v", resp["stderr"])
	}
	if resp["stdout"] != "hello alice\n" {
		t.Fatalf("expected 'hello alice\\n', got %q", resp["stdout"])
	}

	// 6. Patch the skill
	patchedCode := base64.StdEncoding.EncodeToString([]byte(`print("patched")`))
	b, _ := json.Marshal(map[string]interface{}{
		"hash":  hash2,
		"files": map[string]string{"greet.py": patchedCode},
	})
	req, _ := http.NewRequest("PATCH", skillURL, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	patchResp, _ := http.DefaultClient.Do(req)
	patchResp.Body.Close()
	if patchResp.StatusCode != 200 {
		t.Fatalf("expected 200 on patch, got %d", patchResp.StatusCode)
	}

	// 7. Run after patch
	resp = postJSON(t, skillURL+"/run", map[string]interface{}{
		"id":      "run-after-patch",
		"command": "python3 greet.py",
	})
	if resp["status"] != "success" {
		t.Fatalf("expected success: %v", resp["stderr"])
	}
	if resp["stdout"] != "patched\n" {
		t.Fatalf("expected 'patched\\n', got %q", resp["stdout"])
	}

	// 8. Delete
	resp = deleteReq(t, skillURL)
	if resp["status"] != "deleted" {
		t.Fatalf("expected deleted, got %v", resp["status"])
	}

	// 9. Confirm gone
	resp = getJSON(t, skillURL)
	if resp["cached"] != false {
		t.Fatal("expected not cached after delete")
	}

	// 10. Run against deleted skill -> 404
	resp = postJSON(t, skillURL+"/run", map[string]interface{}{
		"id":      "run-deleted",
		"command": "echo nope",
	})
	if resp["_status_code"] != float64(404) {
		t.Fatalf("expected 404 for deleted skill run, got %v", resp["_status_code"])
	}
}

func TestE2EHealthShowsCachedSkills(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	// Upload two skills
	for _, name := range []string{"skill-a", "skill-b"} {
		postJSON(t, base+"/skill/"+name, map[string]interface{}{
			"hash":  "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			"files": map[string]string{"f.txt": base64.StdEncoding.EncodeToString([]byte("x"))},
		})
	}

	resp := getJSON(t, base+"/health")
	skills := resp["cached_skills"].([]interface{})
	if len(skills) != 2 {
		t.Fatalf("expected 2 cached skills in health, got %d", len(skills))
	}
}
