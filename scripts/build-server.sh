#!/bin/bash
set -e

# Go thường được cài qua `go install golang.org/dl/go1.25.0@latest` rồi
# `go1.25.0 download`, để lại binary ở ~/sdk/goX.Y.Z/bin mà KHÔNG thêm vào PATH.
# Không có đoạn này, script chết giữa chừng với đúng một dòng "go: command not
# found" — sau khi đã build xong frontend, nên rất dễ tưởng lỗi ở frontend.
if ! command -v go >/dev/null 2>&1; then
  for candidate in "$HOME"/sdk/go*/bin /usr/local/go/bin /opt/homebrew/opt/go/libexec/bin; do
    if [ -x "$candidate/go" ]; then
      export PATH="$candidate:$PATH"
      break
    fi
  done
fi
if ! command -v go >/dev/null 2>&1; then
  echo "Không tìm thấy Go. Cài Go 1.25+ rồi thêm vào PATH, ví dụ:" >&2
  echo "  export PATH=\"\$HOME/sdk/go1.25.0/bin:\$PATH\"" >&2
  exit 1
fi
echo "-> Go: $(go version)"

VERSION=${1:-dev}
echo "=== Building VNET Server v$VERSION ==="
echo "-> Building Admin UI..."
cd "$(dirname "$0")/../admin"
npm ci 2>/dev/null
npm run build
mkdir -p ../backend/cmd/server/embed
cp -r dist/* ../backend/cmd/server/embed/

echo "-> Building Server..."
cd ../backend
go build -ldflags="-s -w -X main.version=$VERSION" -o vnet-server ./cmd/server

echo "=== Done: backend/vnet-server ==="
sha256sum vnet-server
ls -lh vnet-server
