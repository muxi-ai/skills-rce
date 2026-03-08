FROM golang:1.26-alpine AS go-builder

WORKDIR /build
COPY src/go.mod src/go.sum ./
RUN go mod download
COPY .version ./cmd/rce/version.txt
COPY src/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" -o skills-rce ./cmd/rce

# --------------------------------------------------------------------------
# Python packages -- build in a full Ubuntu, strip, copy to final
# --------------------------------------------------------------------------
FROM ubuntu:22.04 AS python-builder

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3.11 python3.11-venv python3.11-dev python3-pip build-essential \
    libmagic1 \
    && update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.11 1 \
    && rm -rf /var/lib/apt/lists/*

RUN pip3 install --no-cache-dir --no-compile --target=/install \
    numpy pandas scipy scikit-learn statsmodels sympy \
    matplotlib seaborn plotly bokeh altair \
    pypdf pdfplumber reportlab fpdf2 \
    python-docx openpyxl python-pptx xlsxwriter xlrd xlwt \
    pillow pytesseract pdf2image qrcode python-barcode \
    beautifulsoup4 markdownify lxml Markdown \
    requests httpx aiofiles \
    pyyaml jinja2 tabulate chardet orjson python-magic \
    cryptography

# Strip test data, type stubs, bytecode
RUN find /install -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true \
    && find /install -type d -name tests -exec rm -rf {} + 2>/dev/null || true \
    && find /install -type d -name test -exec rm -rf {} + 2>/dev/null || true \
    && find /install -name '*.pyi' -delete 2>/dev/null || true \
    && find /install -name '*.pyc' -delete 2>/dev/null || true \
    && find /install -name '*.pyx' -delete 2>/dev/null || true \
    && find /install -name '*.c' -delete 2>/dev/null || true \
    && find /install -name '*.h' -delete 2>/dev/null || true

# --------------------------------------------------------------------------
# Bun + JS packages
# --------------------------------------------------------------------------
FROM ubuntu:22.04 AS bun-builder

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl ca-certificates unzip \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://bun.sh/install | bash
ENV PATH="/root/.bun/bin:${PATH}"

RUN mkdir -p /opt/bun-packages && cd /opt/bun-packages \
    && bun init -y \
    && bun add \
       lodash axios cheerio sharp csv-parse date-fns zod marked \
       uuid yaml jsonwebtoken chalk node-fetch papaparse jsdom commander \
    && rm -rf /root/.bun/install/cache /tmp/*

# --------------------------------------------------------------------------
# Final image -- only runtime deps, no compilers or build tools
# --------------------------------------------------------------------------
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Runtime-only system packages (no -dev, no build-essential)
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl wget git ca-certificates \
    poppler-utils qpdf tesseract-ocr \
    ffmpeg imagemagick \
    python3.11 python3.11-venv python3-pip \
    libmagic1 \
    && update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.11 1 \
    && apt-get purge -y --auto-remove -o APT::AutoRemove::RecommendsImportant=false \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/* \
    && find /usr/share/doc -depth -type f ! -name copyright -delete 2>/dev/null || true \
    && find /usr/share/doc -empty -delete 2>/dev/null || true \
    && rm -rf /usr/share/man /usr/share/info /usr/share/lintian

# Node.js 20 (for npx and node compatibility)
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# uv (fast Python package manager)
RUN curl -LsSf https://astral.sh/uv/install.sh | sh
ENV PATH="/root/.local/bin:${PATH}"

# Copy Python packages (no pip, no dev headers in final image)
COPY --from=python-builder /install /usr/local/lib/python3.11/dist-packages

# Copy Bun runtime + packages
COPY --from=bun-builder /root/.bun /root/.bun
COPY --from=bun-builder /opt/bun-packages /opt/bun-packages
ENV PATH="/root/.bun/bin:${PATH}"
ENV NODE_PATH="/opt/bun-packages/node_modules"

# Copy Go binary
COPY --from=go-builder /build/skills-rce /usr/local/bin/skills-rce

# Go toolchain (for running go scripts)
COPY --from=go-builder /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

RUN mkdir -p /cache/skills

EXPOSE 7891

HEALTHCHECK --interval=10s --timeout=5s --retries=3 --start-period=10s \
    CMD curl -f http://localhost:7891/health || exit 1

ENTRYPOINT ["skills-rce"]
