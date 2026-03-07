package executor

import (
	"encoding/base64"
	"runtime"
	"testing"
)

func TestRunPython(t *testing.T) {
	result := Run(&RunRequest{
		ID:       "test-1",
		Language: "python",
		Code:     "print(40 + 2)",
		Timeout:  10,
	})

	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "42\n" {
		t.Fatalf("expected '42\\n', got %q", result.Stdout)
	}
}

func TestRunBash(t *testing.T) {
	result := Run(&RunRequest{
		ID:       "test-bash",
		Language: "bash",
		Code:     "echo hello",
		Timeout:  10,
	})

	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Stderr)
	}
	if result.Stdout != "hello\n" {
		t.Fatalf("expected 'hello\\n', got %q", result.Stdout)
	}
}

func TestRunError(t *testing.T) {
	result := Run(&RunRequest{
		ID:       "test-err",
		Language: "python",
		Code:     "raise ValueError('boom')",
		Timeout:  10,
	})

	if result.Status != "error" {
		t.Fatalf("expected error, got %s", result.Status)
	}
	if result.ExitCode == 0 {
		t.Fatal("expected non-zero exit code")
	}
}

func TestRunTimeout(t *testing.T) {
	sleepCode := "import time; time.sleep(10)"
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("timeout test requires unix")
	}

	result := Run(&RunRequest{
		ID:       "test-timeout",
		Language: "python",
		Code:     sleepCode,
		Timeout:  1,
	})

	if result.Status != "timeout" {
		t.Fatalf("expected timeout, got %s", result.Status)
	}
	if result.ExitCode != -1 {
		t.Fatalf("expected exit code -1, got %d", result.ExitCode)
	}
}

func TestRunWithFiles(t *testing.T) {
	csvContent := base64.StdEncoding.EncodeToString([]byte("a,b\n1,2\n3,4\n"))
	result := Run(&RunRequest{
		ID:       "test-files",
		Language: "python",
		Code:     "print(open('data.csv').read().strip())",
		Files:    map[string]string{"data.csv": csvContent},
		Timeout:  10,
	})

	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Stderr)
	}
	if result.Stdout != "a,b\n1,2\n3,4\n" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
}

func TestRunArtifacts(t *testing.T) {
	result := Run(&RunRequest{
		ID:       "test-artifacts",
		Language: "python",
		Code:     "open('output.txt', 'w').write('generated')",
		Timeout:  10,
	})

	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Stderr)
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(result.Artifacts))
	}
	if result.Artifacts[0].Name != "output.txt" {
		t.Fatalf("expected artifact name 'output.txt', got %q", result.Artifacts[0].Name)
	}
}

func TestRunUnsupportedLanguage(t *testing.T) {
	result := Run(&RunRequest{
		ID:       "test-unsupported",
		Language: "cobol",
		Code:     "DISPLAY 'HELLO'",
		Timeout:  10,
	})

	if result.Status != "error" {
		t.Fatalf("expected error, got %s", result.Status)
	}
}
