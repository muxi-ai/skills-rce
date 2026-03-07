package executor

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const maxOutputBytes = 100 * 1024 // 100KB

type RunRequest struct {
	ID       string
	Language string
	Code     string
	Files    map[string]string
	Timeout  int
	Env      map[string]string
}

type SkillRunRequest struct {
	ID         string
	Command    string
	InputFiles map[string]string
	Timeout    int
	Env        map[string]string
	SkillDir   string
}

type Artifact struct {
	Name    string `json:"name"`
	Mime    string `json:"mime"`
	Size    int    `json:"size"`
	Content string `json:"content"`
}

type Result struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	ExitCode   int        `json:"exit_code"`
	Stdout     string     `json:"stdout"`
	Stderr     string     `json:"stderr"`
	DurationMs int64      `json:"duration_ms"`
	Artifacts  []Artifact `json:"artifacts"`
}

var languageCommands = map[string]string{
	"python":     "python3",
	"node":       "bun",
	"javascript": "bun",
	"typescript": "bun",
	"bash":       "bash",
	"php":        "php",
	"ruby":       "ruby",
	"go":         "go run",
}

var languageExtensions = map[string]string{
	"python":     ".py",
	"node":       ".js",
	"javascript": ".js",
	"typescript": ".ts",
	"bash":       ".sh",
	"php":        ".php",
	"ruby":       ".rb",
	"go":         ".go",
}

func Run(req *RunRequest) *Result {
	start := time.Now()
	result := &Result{ID: req.ID, Artifacts: []Artifact{}}

	workDir, err := os.MkdirTemp("", fmt.Sprintf("rce-job-%s-", req.ID))
	if err != nil {
		return errorResult(req.ID, fmt.Sprintf("create workdir: %s", err), start)
	}
	defer os.RemoveAll(workDir)

	if err := writeBase64Files(workDir, req.Files); err != nil {
		return errorResult(req.ID, fmt.Sprintf("write input files: %s", err), start)
	}

	ext := languageExtensions[req.Language]
	if ext == "" {
		return errorResult(req.ID, fmt.Sprintf("unsupported language: %s", req.Language), start)
	}

	scriptFile := filepath.Join(workDir, "script"+ext)
	if err := os.WriteFile(scriptFile, []byte(req.Code), 0644); err != nil {
		return errorResult(req.ID, fmt.Sprintf("write script: %s", err), start)
	}

	cmdStr := languageCommands[req.Language]
	var cmd *exec.Cmd
	if req.Language == "go" {
		cmd = exec.Command("go", "run", scriptFile)
	} else {
		cmd = exec.Command(cmdStr, scriptFile)
	}
	cmd.Dir = workDir

	beforeFiles := snapshotDir(workDir)
	result = execute(cmd, req.ID, req.Timeout, req.Env, workDir, start, beforeFiles)
	return result
}

func RunSkill(req *SkillRunRequest) *Result {
	start := time.Now()

	workDir, err := os.MkdirTemp("", fmt.Sprintf("rce-skill-%s-", req.ID))
	if err != nil {
		return errorResult(req.ID, fmt.Sprintf("create workdir: %s", err), start)
	}
	defer os.RemoveAll(workDir)

	// Symlink skill directory contents into work dir
	entries, err := os.ReadDir(req.SkillDir)
	if err != nil {
		return errorResult(req.ID, fmt.Sprintf("read skill dir: %s", err), start)
	}
	for _, entry := range entries {
		src := filepath.Join(req.SkillDir, entry.Name())
		dst := filepath.Join(workDir, entry.Name())
		if err := os.Symlink(src, dst); err != nil {
			return errorResult(req.ID, fmt.Sprintf("symlink %s: %s", entry.Name(), err), start)
		}
	}

	if err := writeBase64Files(workDir, req.InputFiles); err != nil {
		return errorResult(req.ID, fmt.Sprintf("write input files: %s", err), start)
	}

	parts := strings.Fields(req.Command)
	if len(parts) == 0 {
		return errorResult(req.ID, "empty command", start)
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = workDir

	beforeFiles := snapshotDir(workDir)
	return execute(cmd, req.ID, req.Timeout, req.Env, workDir, start, beforeFiles)
}

func execute(cmd *exec.Cmd, id string, timeout int, env map[string]string, workDir string, start time.Time, beforeFiles map[string]bool) *Result {
	if timeout <= 0 {
		timeout = 30
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	cmd.Dir = workDir

	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &limitedWriter{w: &stdout, max: maxOutputBytes}
	cmd.Stderr = &limitedWriter{w: &stderr, max: maxOutputBytes}

	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	result := &Result{
		ID:         id,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		DurationMs: duration,
		Artifacts:  collectArtifacts(workDir, beforeFiles),
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.Status = "timeout"
		result.ExitCode = -1
		if result.Stderr == "" {
			result.Stderr = "Execution exceeded timeout"
		}
		return result
	}

	if err != nil {
		result.Status = "error"
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			if result.Stderr == "" {
				result.Stderr = err.Error()
			}
		}
		return result
	}

	result.Status = "success"
	result.ExitCode = 0
	return result
}

func errorResult(id, msg string, start time.Time) *Result {
	return &Result{
		ID:         id,
		Status:     "error",
		ExitCode:   1,
		Stderr:     msg,
		DurationMs: time.Since(start).Milliseconds(),
		Artifacts:  []Artifact{},
	}
}

func writeBase64Files(dir string, files map[string]string) error {
	for relPath, b64 := range files {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return fmt.Errorf("decode %s: %w", relPath, err)
		}
		fullPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("mkdir for %s: %w", relPath, err)
		}
		if err := os.WriteFile(fullPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", relPath, err)
		}
	}
	return nil
}

func snapshotDir(dir string) map[string]bool {
	snapshot := make(map[string]bool)
	filepath.Walk(dir, func(path string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() {
			snapshot[path] = true
		}
		return nil
	})
	return snapshot
}

func collectArtifacts(dir string, before map[string]bool) []Artifact {
	var artifacts []Artifact
	filepath.Walk(dir, func(path string, info os.FileInfo, _ error) error {
		if info == nil || info.IsDir() || before[path] {
			return nil
		}
		// Skip symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		mimeType := mime.TypeByExtension(filepath.Ext(path))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		artifacts = append(artifacts, Artifact{
			Name:    rel,
			Mime:    mimeType,
			Size:    len(data),
			Content: base64.StdEncoding.EncodeToString(data),
		})
		return nil
	})
	return artifacts
}

type limitedWriter struct {
	w       *strings.Builder
	max     int
	written int
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	remaining := lw.max - lw.written
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	n, err := lw.w.Write(p)
	lw.written += n
	return len(p), err
}
