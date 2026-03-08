# Skills RCE

Code execution service for AI agent skills. Runs scripts in isolated subprocesses with skill directory caching -- upload a skill once, execute against it many times.

This repo contains:

1. **The RCE server** -- a standalone Go binary that exposes an HTTP API for code execution. Install it anywhere, point your agent runtime at it.
2. **A Docker image** (`ghcr.io/muxi-ai/skills-rce`) -- bundles the server with Python, Bun, Node.js, Go, and common packages used by agent skills.
3. **A SIF image** -- Singularity/Apptainer container for native Linux execution without Docker. Published as GitHub Release assets.

## Quick Start

```bash
docker run -d -p 7891:7891 ghcr.io/muxi-ai/skills-rce:latest
```

```bash
# Check what's available
curl http://localhost:7891/status | jq .

# Run code
curl -X POST http://localhost:7891/run \
  -H "Content-Type: application/json" \
  -d '{"id": "test-1", "language": "python", "code": "print(40 + 2)"}'

# Upload a skill (zip)
zip -r skill.zip scripts/ SKILL.md
curl -X POST "http://localhost:7891/skill/my-skill?hash=sha256:abc..." \
  -H "Content-Type: application/zip" \
  --data-binary @skill.zip

# Execute against cached skill
curl -X POST http://localhost:7891/skill/my-skill/run \
  -H "Content-Type: application/json" \
  -d '{"id": "test-2", "command": "python scripts/run.py input.csv", "input_files": {"input.csv": "<base64>"}}'
```

## The Server

The server is a single Go binary. Build it yourself or grab it from the Docker image:

```bash
cd src && go build -o skills-rce ./cmd/rce
./skills-rce
```

It listens on port 7891 by default. The server doesn't care what runtimes are installed -- it shells out to whatever is available on the host. If Python isn't installed, Python jobs will fail. The Docker image solves this by bundling everything.

### Configuration

All via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `RCE_PORT` | `7891` | Listen port |
| `RCE_CACHE_DIR` | `/cache/skills` | Skill cache directory |
| `RCE_DEFAULT_TIMEOUT` | `30` | Default job timeout (seconds) |
| `RCE_MAX_TIMEOUT` | `300` | Maximum allowed timeout |
| `RCE_AUTH_TOKEN` | (none) | Bearer token for authenticated endpoints |

### Authentication

Set `RCE_AUTH_TOKEN` to require a bearer token on all endpoints except `/health` and `/status`:

```bash
docker run -d -p 7891:7891 -e RCE_AUTH_TOKEN=my-secret ghcr.io/muxi-ai/skills-rce:latest
```

```bash
curl -H "Authorization: Bearer my-secret" http://localhost:7891/run ...
```

## Docker Image

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

Use `GET /status` to see the exact versions of all packages installed in any given image.

## SIF Image

Singularity/Apptainer builds are published as GitHub Release assets for native Linux execution without Docker. Download the `.sif` file for your architecture (amd64 or arm64) from the [Releases](https://github.com/muxi-ai/skills-rce/releases) page.

To build locally:

```bash
./scripts/build/docker.sh
./scripts/build/sif.sh
```

## API

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/health` | No | Lightweight liveness check (status + version) |
| GET | `/status` | No | Full capabilities: runtimes, languages, packages, resources, cached skills |
| POST | `/run` | Yes | Execute ad-hoc code |
| POST | `/skill/{id}` | Yes | Upload/cache a skill directory (JSON or zip) |
| GET | `/skill/{id}` | Yes | Check cache status + hash |
| PATCH | `/skill/{id}` | Yes | Update files in cached skill (JSON or zip) |
| DELETE | `/skill/{id}` | Yes | Remove cached skill |
| POST | `/skill/{id}/run` | Yes | Execute command against cached skill |

See [openapi.yaml](openapi.yaml) for the full spec.

### GET /health

Lightweight liveness check for polling and load balancers:

```json
{"status": "healthy", "version": "0.20260308.0"}
```

### GET /status

Full server capabilities. Call once on connect to discover what's available:

```json
{
  "status": "healthy",
  "version": "0.20260308.0",
  "runtimes": [
    {"name": "python", "version": "3.11.0"},
    {"name": "bun", "version": "1.3.10"},
    {"name": "node", "version": "20.20.1"}
  ],
  "languages": ["python", "javascript", "typescript", "bash", "go", "perl"],
  "resources": {"cpus": 4, "memory_mb": 8192, "disk_mb": 20480},
  "packages": {
    "python": [{"name": "numpy", "version": "2.4.2"}, ...],
    "node": [{"name": "lodash", "version": "4.17.21"}, ...]
  },
  "cached_skills": [],
  "uptime_seconds": 120
}
```

### POST /run

Execute a script with no skill context:

```bash
curl -X POST http://localhost:7891/run \
  -H "Content-Type: application/json" \
  -d '{"id": "test-1", "language": "python", "code": "print(40 + 2)"}'
```

### POST /skill/{id}

Upload a skill directory. Accepts two formats:

**Zip upload** (recommended):
```bash
curl -X POST "http://localhost:7891/skill/my-skill?hash=sha256:abc..." \
  -H "Content-Type: application/zip" \
  --data-binary @skill.zip
```

**JSON upload:**
```bash
curl -X POST http://localhost:7891/skill/my-skill \
  -H "Content-Type: application/json" \
  -d '{"hash": "sha256:abc...", "files": {"SKILL.md": "<base64>", "scripts/run.py": "<base64>"}}'
```

### POST /skill/{id}/run

Execute a command against a cached skill:

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
  "stdout": "42\n",
  "stderr": "",
  "duration_ms": 12,
  "artifacts": [
    {"name": "output.png", "mime": "image/png", "size": 34512, "content": "<base64>"}
  ]
}
```

`status` is `success`, `error`, or `timeout`.

## Development

### Project Structure

```
.version                  # ScalVer version (MAJOR.YYYYMMDD.PATCH)
Dockerfile                # Multi-stage Docker build
openapi.yaml              # OpenAPI 3.1 spec
src/
  cmd/rce/                # Server entrypoint
  pkg/
    api/                  # HTTP handlers, auth, logging middleware
    cache/                # Filesystem skill cache manager
    config/               # Env-based configuration
    executor/             # Subprocess runner with timeouts and artifact collection
    sysinfo/              # Runtime detection, system resources, package detection
  e2e/                    # End-to-end tests
scripts/
  build/
    docker.sh             # Build Docker image
    sif.sh                # Convert Docker to SIF
```

### Build & Test

```bash
cd src && go build ./cmd/rce         # Build binary
cd src && go test ./... -count=1     # Run all tests
./scripts/build/docker.sh            # Build Docker image
./scripts/build/sif.sh               # Build SIF (requires Docker image)
```

### Versioning

Uses [ScalVer](https://scalver.org) (`MAJOR.YYYYMMDD.PATCH`). Version is auto-calculated on release -- don't bump manually.

### Git Workflow

```
develop (default) -> rc (release candidate) -> main (production)
                                                    |
                                            auto-merge back to develop
```

- Push to `develop`: CI runs tests + Docker build
- Push to `rc`: Full test matrix + Docker smoke test
- Push to `main`: Auto-release (version, Docker, SIF, GitHub Release)

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

Server formations get Skills RCE automatically. Only configure `rce` if you need a custom instance.

## Security

- Each job runs in an isolated subprocess with resource limits
- Cached skill directories are read-only during execution
- Working directories cleaned up after each job
- Output truncated at 100KB
- Zip uploads validated against path traversal attacks
- No host filesystem access beyond mounted volumes

## License

Apache License 2.0
