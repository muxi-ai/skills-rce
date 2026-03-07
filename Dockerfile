FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY src/go.mod src/go.sum ./
RUN go mod download
COPY src/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" -o skills-rce ./cmd/rce

FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# System packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl wget git ca-certificates unzip \
    poppler-utils qpdf tesseract-ocr \
    ffmpeg imagemagick \
    python3.11 python3.11-venv python3.11-dev python3-pip \
    libmagic1 \
    && update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.11 1 \
    && rm -rf /var/lib/apt/lists/*

# Bun (runs JS, TS, and JSX natively)
RUN curl -fsSL https://bun.sh/install | bash
ENV PATH="/root/.bun/bin:${PATH}"

# Python packages -- data, viz, docs, general purpose
RUN pip3 install --no-cache-dir --break-system-packages \
    # Data and analysis
    numpy pandas scipy scikit-learn statsmodels sympy \
    # Visualization
    matplotlib seaborn plotly bokeh altair \
    # PDF
    pypdf pdfplumber reportlab fpdf2 \
    # Office docs
    python-docx openpyxl python-pptx xlsxwriter xlrd xlwt \
    # Image
    pillow pytesseract pdf2image qrcode python-barcode \
    # HTML/XML
    beautifulsoup4 markdownify lxml Markdown \
    # HTTP
    requests httpx aiofiles \
    # General purpose
    pyyaml jinja2 tabulate chardet orjson python-magic \
    cryptography

COPY --from=builder /build/skills-rce /usr/local/bin/skills-rce

RUN mkdir -p /cache/skills

EXPOSE 5580

HEALTHCHECK --interval=10s --timeout=5s --retries=3 --start-period=10s \
    CMD curl -f http://localhost:5580/health || exit 1

ENTRYPOINT ["skills-rce"]
