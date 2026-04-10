#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_NAME="parquet-rowcount"
OUTPUT_DIR="${ROOT_DIR}/dist"

TARGETS=(
  "darwin amd64"
  "darwin arm64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
  "windows arm64"
)

rm -rf "${OUTPUT_DIR}"
mkdir -p "${OUTPUT_DIR}"

for target in "${TARGETS[@]}"; do
  read -r goos goarch <<<"${target}"
  ext=""
  if [[ "${goos}" == "windows" ]]; then
    ext=".exe"
  fi

  out="${OUTPUT_DIR}/${BIN_NAME}-${goos}-${goarch}${ext}"
  echo "building ${out}"
  CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
    go build -trimpath -ldflags="-s -w" -o "${out}" ./cmd/parquet-rowcount
done

echo "artifacts written to ${OUTPUT_DIR}"
