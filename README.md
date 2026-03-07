# Skills RCE

A standalone code execution service for AI agents. Runs code in isolated containers with skill directory caching -- upload a skill once, execute against it many times.

Ships as `muxi/skills-rce` Docker image. Any agent runtime that speaks HTTP can use it.

## Quick Start

```bash
docker run -d -p 5580:5580 muxi/skills-rce:latest
```

```bash
# Health check
curl http://localhost:5580/health

# Run ad-hoc code
curl -X POST http://localhost:5580/run \
  -H "Content-Type: application/json" \
  -d '{"id": "test-1", "language": "python", "code": "print(40 + 2)"}'

# Upload a skill
curl -X POST http://localhost:5580/skill/my-skill \
  -H "Content-Type: application/json" \
  -d '{"hash": "sha256:abc...", "files": {"SKILL.md": "<base64>", "scripts/run.py": "<base64>"}}'

# Execute against cached skill
curl -X POST http://localhost:5580/skill/my-skill/run \
  -H "Content-Type: application/json" \
  -d '{"id": "test-2", "command": "python scripts/run.py input.csv", "input_files": {"input.csv": "<base64>"}}'
```

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| POST | `/run` | Execute ad-hoc code |
| POST | `/skill/{id}` | Upload/cache a skill directory |
| GET | `/skill/{id}` | Check cache status + hash |
| PATCH | `/skill/{id}` | Update specific files in cached skill |
| DELETE | `/skill/{id}` | Remove cached skill |
| POST | `/skill/{id}/run` | Execute command against cached skill |

### POST /run

Execute code with no skill context.

```json
{
  "id": "job-abc123",
  "language": "python",
  "code": "import pandas as pd\nprint(pd.DataFrame({'a': [1,2,3]}).to_json())",
  "files": {},
  "timeout": 30,
  "env": {}
}
```

### POST /skill/{id}

Cache a skill directory. Files are stored with their directory structure preserved.

```json
{
  "hash": "sha256:abc123...",
  "files": {
    "SKILL.md": "<base64>",
    "scripts/extract.py": "<base64>",
    "scripts/utils.py": "<base64>",
    "references/spec.md": "<base64>"
  }
}
```

### POST /skill/{id}/run

Execute a command against a cached skill. Only sends the command and input files -- the skill directory is already on the server.

```json
{
  "id": "job-def456",
  "command": "python scripts/extract.py input.pdf",
  "input_files": {
    "input.pdf": "<base64>"
  },
  "timeout": 60
}
```

### Response Format

All execution endpoints return:

```json
{
  "id": "job-abc123",
  "status": "success",
  "exit_code": 0,
  "stdout": "...",
  "stderr": "",
  "duration_ms": 1234,
  "artifacts": [
    {
      "name": "output.png",
      "mime": "image/png",
      "size": 34512,
      "content": "<base64>"
    }
  ]
}
```

`status` is one of `success`, `error`, or `timeout`.

## Built-in Skills

The image ships with pre-cached skills:

- **`generate-file`** -- Generate files (charts, documents, images) via Python code execution. Includes constraints and library preferences for reliable artifact generation.

## Pre-installed Languages and Packages

**Languages:** Python 3.11, Node.js 20, Bash

**Python packages:** numpy, pandas, matplotlib, seaborn, plotly, pypdf, pdfplumber, reportlab, python-docx, openpyxl, python-pptx, pillow, pytesseract, pdf2image, beautifulsoup4, requests, httpx, scipy, scikit-learn

**System tools:** curl, wget, git, ffmpeg, imagemagick, poppler-utils, qpdf, tesseract-ocr

## Configuration

### With MUXI Runtime

```yaml
# formation.yaml
rce:
  url: "http://localhost:5580"
  auth:
    type: "bearer"
    token: "${{ secrets.RCE_TOKEN }}"
```

### With MUXI Server

Server formations get the built-in Skills RCE automatically. Only configure `rce` if you need a custom instance (specific packages, GPU, etc.).

### Authentication

Optional. Supports `bearer`, `header`, `basic`, or `none` (default).

## Security

- Each job runs in an isolated subprocess with resource limits
- Cached skill directories are read-only during execution
- Working directories cleaned up after each job
- Output truncated at 100KB
- No host filesystem access beyond mounted volumes

## License

Apache License 2.0
