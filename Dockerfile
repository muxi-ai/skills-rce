FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY src/go.mod src/go.sum ./
RUN go mod download
COPY src/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" -o skills-rce ./cmd/rce

FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl wget git ca-certificates \
    poppler-utils qpdf tesseract-ocr \
    ffmpeg imagemagick \
    python3.11 python3.11-venv python3-pip \
    nodejs npm \
    && update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.11 1 \
    && rm -rf /var/lib/apt/lists/*

RUN pip3 install --no-cache-dir --break-system-packages \
    numpy pandas matplotlib seaborn plotly \
    pypdf pdfplumber reportlab \
    python-docx openpyxl python-pptx \
    pillow pytesseract pdf2image \
    beautifulsoup4 requests httpx \
    scipy scikit-learn

COPY --from=builder /build/skills-rce /usr/local/bin/skills-rce

RUN mkdir -p /cache/skills

EXPOSE 5580

HEALTHCHECK --interval=10s --timeout=5s --retries=3 --start-period=10s \
    CMD curl -f http://localhost:5580/health || exit 1

ENTRYPOINT ["skills-rce"]
