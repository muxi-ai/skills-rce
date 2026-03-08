#!/usr/bin/env bash
#
# Convert Skills RCE Docker image to Singularity SIF
#
# Usage:
#   ./scripts/build/sif.sh                # Auto-detect arch
#   ./scripts/build/sif.sh --arch arm64   # Specific arch
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/../.."

ARCH=""
while [[ $# -gt 0 ]]; do
    case $1 in
        --arch)
            ARCH="$2"
            shift 2
            ;;
        --arch=*)
            ARCH="${1#*=}"
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--arch amd64|arm64]"
            exit 1
            ;;
    esac
done

VERSION=$(cat "$PROJECT_ROOT/.version" | tr -d '[:space:]')
IMAGE_NAME="muxi/skills-rce"

if [ -z "$ARCH" ]; then
    ARCH=$(docker inspect "$IMAGE_NAME:$VERSION" --format '{{.Architecture}}' 2>/dev/null || true)
    if [ -z "$ARCH" ]; then
        case "$(uname -m)" in
            aarch64|arm64) ARCH="arm64" ;;
            x86_64|amd64)  ARCH="amd64" ;;
            *)             ARCH="$(uname -m)" ;;
        esac
    fi
fi

SIF_DIR="$PROJECT_ROOT/sif-builds"
TARBALL="$PROJECT_ROOT/skills-rce-$VERSION.tar"
SIF_FILE="$SIF_DIR/skills-rce-$VERSION-linux-$ARCH.sif"
LATEST_SIF="$SIF_DIR/skills-rce-latest-linux-$ARCH.sif"

mkdir -p "$SIF_DIR"

echo "Converting Docker to SIF"
echo "  Version:  $VERSION"
echo "  Arch:     $ARCH"
echo "  Image:    $IMAGE_NAME:$VERSION"
echo "  Output:   $SIF_FILE"
echo ""

if ! docker image inspect "$IMAGE_NAME:$VERSION" > /dev/null 2>&1; then
    echo "Error: Docker image $IMAGE_NAME:$VERSION not found"
    echo "Build it first: ./scripts/build/docker.sh"
    exit 1
fi

echo "Exporting Docker image..."
docker save "$IMAGE_NAME:$VERSION" -o "$TARBALL"

echo "Building SIF..."
if command -v singularity &> /dev/null; then
    singularity build "$SIF_FILE" "docker-archive://$TARBALL"
elif command -v apptainer &> /dev/null; then
    apptainer build "$SIF_FILE" "docker-archive://$TARBALL"
else
    echo "Using Docker-wrapped Singularity..."
    docker run --rm --privileged \
        -v "$(pwd):/work" \
        -w /work \
        ghcr.io/muxi-ai/runtime-runner:latest \
        build "$SIF_FILE" "docker-archive://$TARBALL"
fi

cp "$SIF_FILE" "$LATEST_SIF"
rm "$TARBALL"

echo ""
echo "SIF files:"
echo "  Versioned: $SIF_FILE ($(du -h "$SIF_FILE" | cut -f1))"
echo "  Latest:    $LATEST_SIF"
