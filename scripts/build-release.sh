#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-0.1.0-dev}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILT_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
OUT_DIR="${OUT_DIR:-dist}"
APP_NAME="${APP_NAME:-aruba-ap-to-iap-converter}"

mkdir -p "$OUT_DIR"

go test ./...
go build \
  -ldflags "-X github.com/dailypush/aruba-ap-to-iap-converter/internal/version.Version=$VERSION -X github.com/dailypush/aruba-ap-to-iap-converter/internal/version.Commit=$COMMIT -X github.com/dailypush/aruba-ap-to-iap-converter/internal/version.BuiltAt=$BUILT_AT" \
  -o "$OUT_DIR/$APP_NAME" ./cmd/iap325-converter

echo "Built $OUT_DIR/$APP_NAME"
echo "Version: $VERSION"
echo "Commit:  $COMMIT"
echo "Built:   $BUILT_AT"
