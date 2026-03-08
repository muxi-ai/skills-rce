#!/usr/bin/env bash
#
# Build Skills RCE Docker image
#
# Usage:
#   ./scripts/build/docker.sh                     # Build for native arch
#   ./scripts/build/docker.sh --no-cache          # Rebuild from scratch
#   ./scripts/build/docker.sh --platform linux/amd64  # Cross-build
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/../.."

VERSION=$(cat "$PROJECT_ROOT/.version" | tr -d '[:space:]')
IMAGE_NAME="muxi/skills-rce"
EXTRA_ARGS=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --platform|--platform=*)
            if [[ "$1" == --platform=* ]]; then
                EXTRA_ARGS="$EXTRA_ARGS --platform ${1#*=}"
            else
                EXTRA_ARGS="$EXTRA_ARGS --platform $2"
                shift
            fi
            shift
            ;;
        *)
            EXTRA_ARGS="$EXTRA_ARGS $1"
            shift
            ;;
    esac
done

echo "Building $IMAGE_NAME:$VERSION"
echo ""

docker build \
    -f "$PROJECT_ROOT/Dockerfile" \
    -t "$IMAGE_NAME:$VERSION" \
    -t "$IMAGE_NAME:latest" \
    $EXTRA_ARGS \
    "$PROJECT_ROOT"

echo ""
echo "Built: $IMAGE_NAME:$VERSION"
echo "       $IMAGE_NAME:latest"
docker images "$IMAGE_NAME" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"
