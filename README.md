# Skills RCE

Code execution service for AI agent skills. Runs scripts in isolated subprocesses with skill directory caching -- upload a skill once, execute against it many times.

This repo contains three things:

1. **The RCE server** -- a standalone Go binary that exposes an HTTP API for code execution. Install it anywhere, point your agent runtime at it.
2. **A Docker image** (`muxi/skills-rce`) -- bundles the server with Python, Bun, Node.js, Go, and common packages used by agent skills. Ready to run.
3. **A SIF image** (coming soon) -- packages the Docker image as a Singularity container for native Linux execution without Docker.

## The Server

The server is a single Go binary. Build it yourself or grab it from the Docker image:

```bash
cd src && go build -o skills-rce ./cmd/rce
./skills-rce
```

It listens on port 7891 by default and exposes 7 endpoints. The server doesn't care what runtimes are installed -- it shells out to whatever is available on the host. If Python isn't installed, Python jobs will fail. The Docker image solves this by bundling everything.

### Configuration

All via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `RCE_PORT` | `7891` | Listen port |
| `RCE_CACHE_DIR` | `/cache/skills` | Skill cache directory |
| `RCE_DEFAULT_TIMEOUT` | `30` | Default job timeout (seconds) |
| `RCE_MAX_TIMEOUT` | `300` | Maximum allowed timeout |
| `RCE_AUTH_TOKEN` | (none) | Bearer token for all non-health endpoints |

### Authentication

Set `RCE_AUTH_TOKEN` to require a bearer token on all endpoints except `/health`:

```bash
RCE_AUTH_TOKEN=my-secret ./skills-rce
```

```bash
curl -H "Authorization: Bearer my-secret" http://localhost:7891/run ...
```

## Docker Image

```bash
docker run -d -p 7891:7891 muxi/skills-rce:latest
```

The image bundles the server with runtimes and packages commonly used by agent skills.

### Runtimes

| Runtime | Version | Languages |
|---------|---------|-----------|
| Python | 3.11 | python |
| Bun | latest | javascript, typescript |
| Node.js | 20 | (npx, npm) |
| Go | 1.26 | go |
| Bash | 5.1 | bash |
| Perl | 5.34 | perl |

Also includes: `uv`, `pip`, `npx`, `npm`.

### Python Packages

**Data & analysis:** numpy, pandas, scipy, scikit-learn, statsmodels, sympy

**Visualization:** matplotlib, seaborn, plotly, bokeh, altair

**PDF:** pypdf, pdfplumber, reportlab, fpdf2

**Office docs:** python-docx, openpyxl, python-pptx, xlsxwriter, xlrd, xlwt

**Image:** pillow, pytesseract, pdf2image, qrcode, python-barcode

**HTML/XML:** beautifulsoup4, markdownify, lxml, Markdown

**HTTP:** requests, httpx, aiofiles

**General:** pyyaml, jinja2, tabulate, chardet, orjson, python-magic, cryptography

### JS/TS Packages

lodash, axios, cheerio, sharp, csv-parse, date-fns, zod, marked, uuid, yaml, jsonwebtoken, chalk, node-fetch, papaparse, jsdom, commander

### System Tools

curl, wget, git, ffmpeg, imagemagick, poppler-utils, qpdf, tesseract-ocr

## SIF Image

> Coming soon.

A Singularity Image Format (SIF) build of the Docker image for running on Linux without Docker. Useful for bare-metal deployments and HPC environments where Docker isn't available.

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check, runtime detection, system resources |
| POST | `/run` | Execute ad-hoc code |
| POST | `/skill/{id}` | Upload/cache a skill directory |
| GET | `/skill/{id}` | Check cache status + hash |
| PATCH | `/skill/{id}` | Update specific files in cached skill |
| DELETE | `/skill/{id}` | Remove cached skill |
| POST | `/skill/{id}/run` | Execute command against cached skill |

See [openapi.yaml](openapi.yaml) for the full spec.

### GET /health

Returns detected runtimes, available languages, system resources, and cached skills:

```json
{
  "status": "healthy",
  "version": "0.1.0",
  "runtimes": [
    {"name": "python", "version": "3.11.0"},
    {"name": "bun", "version": "1.3.10"},
    {"name": "bash", "version": "5.1.16"},
    {"name": "go", "version": "1.26.1"},
    {"name": "node", "version": "20.20.1"},
    {"name": "npx", "version": "10.8.2"},
    {"name": "uv", "version": "0.10.9"},
    {"name": "pip", "version": "22.0.2"}
  ],
  "languages": ["python", "javascript", "typescript", "bash", "go", "perl"],
  "resources": {
    "cpus": 4,
    "memory_mb": 8192,
    "disk_mb": 20480
  },
  "cached_skills": [],
  "uptime_seconds": 120
}
```

Runtimes with `"version": null` are not installed. Languages are derived from detected runtimes.

### POST /run

Execute a script with no skill context:

```bash
curl -X POST http://localhost:7891/run \
  -H "Content-Type: application/json" \
  -d '{"id": "test-1", "language": "python", "code": "print(40 + 2)"}'
```

```json
{
  "id": "test-1",
  "language": "python",
  "code": "print(40 + 2)",
  "files": {},
  "timeout": 30,
  "env": {}
}
```

### POST /skill/{id}

Upload a skill directory. Files are stored with directory structure preserved:

```bash
curl -X POST http://localhost:7891/skill/my-skill \
  -H "Content-Type: application/json" \
  -d '{"hash": "sha256:abc...", "files": {"SKILL.md": "<base64>", "scripts/run.py": "<base64>"}}'
```

### POST /skill/{id}/run

Execute a command against a cached skill. Only sends the command and input files -- the skill directory is already on the server:

```bash
curl -X POST http://localhost:7891/skill/my-skill/run \
  -H "Content-Type: application/json" \
  -d '{"id": "test-2", "command": "python scripts/run.py input.csv", "input_files": {"input.csv": "<base64>"}}'
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

`status` is `success`, `error`, or `timeout`.

## MUXI Integration

### With MUXI Runtime

```yaml
# formation.yaml
rce:
  url: "http://localhost:7891"
  auth:
    type: "bearer"
    token: "${{ secrets.RCE_TOKEN }}"
```

### With MUXI Server

Server formations get Skills RCE automatically. Only configure `rce` if you need a custom instance (specific packages, GPU, etc.).

## Security

- Each job runs in an isolated subprocess with resource limits
- Cached skill directories are read-only during execution
- Working directories cleaned up after each job
- Output truncated at 100KB
- No host filesystem access beyond mounted volumes

## License

Apache License 2.0
