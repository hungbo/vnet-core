#!/bin/bash
set -e

# Máy khách để bàn là MỘT binary với hai chế độ:
#   vnet-client.exe                              → giao diện Wails
#   vnet-client.exe --watch <pid> <mã máy> <url> → tiến trình trông chừng
# Không có binary agent riêng.
#
# Frontend được nhúng bằng //go:embed all:frontend/dist nên không cần `wails
# build` để đóng gói tài nguyên, NHƯNG hai cờ dưới đây là bắt buộc:
#
#   -tags desktop,production
#       Không có tag `production`, Go chọn internal/app/app_default_windows.go
#       — bản rỗng chỉ bật hộp thoại "Wails applications will not build without
#       the correct build tags" rồi thoát. Tệp .exe vẫn build ra bình thường,
#       vẫn chạy được, chỉ là KHÔNG BAO GIỜ mở giao diện.
#
#   -H windowsgui
#       Không có cờ này, mỗi lần khách mở máy sẽ có một cửa sổ console đen nhảy
#       lên sau giao diện.
#
# Đây đúng là bộ cờ `wails build` dùng cho bản production (xem
# pkg/commands/build/base.go trong wails v2).

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
ARCHS=${2:-amd64 arm64}

echo "=== Building VNET Desktop Client v$VERSION ==="
cd "$(dirname "$0")/../client"

echo "-> Building frontend..."
cd src/frontend
npm ci 2>/dev/null || npm install
npm run build
cd ../..

# Số phiên bản dạng a.b.c.d cho thông tin tệp của Windows. "dev" hay chuỗi lạ
# thì ghi 0.0.0.0 — go-winres từ chối mọi thứ không phải số.
WIN_VER=$(echo "$VERSION" | sed 's/^v//')
echo "$WIN_VER" | grep -Eq '^[0-9]+(\.[0-9]+){0,3}$' || WIN_VER=0.0.0.0

rm -rf dist && mkdir -p dist
for arch in $ARCHS; do
  echo "-> Building windows/$arch..."
  # Icon và thông tin phiên bản cho tệp .exe. `go build` thường (không qua
  # `wails build`) không gắn tài nguyên nào, nên trước đây .exe hiện icon trống
  # của Windows. Tệp .syso nằm cạnh mã nguồn thì `go build` tự nhúng; xoá ngay
  # sau khi build để không lọt vào git.
  #
  # --manifest none có chủ đích: manifest đổi chế độ DPI của cửa sổ, mà vị trí
  # thanh dọc đang được tính theo chế độ hiện tại.
  #
  # Icon nhúng HAI lần: tên "APP" cho File Explorer, và mã số 3 cho Wails —
  # Wails nạp icon cửa sổ/thanh tác vụ bằng đúng mã 3 (như `wails build` nhúng).
  # Chế độ `simply` chỉ nhúng tên "APP", nên tệp .exe có icon mà cửa sổ mở ra
  # vẫn mang icon mặc định của Windows.
  WINRES_DIR=$(mktemp -d)
  cp src/build/windows/icon.ico "$WINRES_DIR/icon.ico"
  cat > "$WINRES_DIR/winres.json" <<JSON
{
  "RT_GROUP_ICON": {
    "APP": { "0000": "icon.ico" },
    "#3": { "0000": "icon.ico" }
  },
  "RT_VERSION": {
    "#1": {
      "0000": {
        "fixed": { "file_version": "$WIN_VER", "product_version": "$WIN_VER" },
        "info": {
          "0409": {
            "CompanyName": "VNET",
            "FileDescription": "VNET Client",
            "FileVersion": "$WIN_VER",
            "LegalCopyright": "VNET",
            "OriginalFilename": "vnet-client.exe",
            "ProductName": "VNET Client",
            "ProductVersion": "$WIN_VER"
          }
        }
      }
    }
  }
}
JSON
  (cd src && go run github.com/tc-hib/go-winres@v0.3.3 make \
      --in "$WINRES_DIR/winres.json" --arch "$arch" --out rsrc)
  rm -rf "$WINRES_DIR"

  # CGO tắt: Wails trên Windows dùng go-webview2 thuần Go, không cần trình biên
  # dịch C — nhờ vậy biên dịch chéo từ macOS/Linux chạy được.
  GOOS=windows GOARCH="$arch" CGO_ENABLED=0 go build -buildvcs=false \
    -tags desktop,production \
    -ldflags="-s -w -H windowsgui -X main.version=$VERSION" \
    -o "dist/vnet-client-$arch.exe" ./src
  rm -f src/rsrc_windows_*.syso

  # Thiếu tag `production` thì tệp vẫn build ra và vẫn chạy — chỉ hiện hộp thoại
  # lỗi rồi tắt. Không có cách nào phát hiện từ máy Mac ngoài việc soi chuỗi này
  # trong binary, nên kiểm ngay tại đây thay vì để tới lúc khách mở máy.
  if strings "dist/vnet-client-$arch.exe" 2>/dev/null | grep -q "will not build without the correct build tags"; then
    echo "LỖI: dist/vnet-client-$arch.exe build thiếu tag — mở lên sẽ chỉ hiện hộp thoại lỗi của Wails." >&2
    exit 1
  fi
done

cd dist
zip -r "../vnet-client-windows-${VERSION}.zip" ./*
cd ..

# Bộ cài Inno Setup: đăng ký dịch vụ nền và ghi config.json. iscc chỉ chạy trên
# Windows, nên máy khác vẫn xuất được .exe + .zip như thường — bỏ qua chứ không
# làm hỏng cả lần build.
if command -v iscc >/dev/null 2>&1; then
  echo "-> Đóng gói bộ cài..."
  iscc "/DVersion=$VERSION" "/DSourceExe=..\\client\\dist\\vnet-client-amd64.exe" \
    "$(dirname "$0")/../installer/vnet-client.iss"
else
  echo "-> Bỏ qua bộ cài: không có iscc (Inno Setup chỉ chạy trên Windows)"
fi

echo "=== Done: vnet-client-windows-${VERSION}.zip ==="
ls -lh dist/

# In băm SHA-256 để dán thẳng vào form công bố bản cập nhật trên trang quản trị.
# Máy trạm từ chối chạy tệp có băm không khớp, nên thiếu con số này là không
# công bố được bản nào.
echo
echo "SHA-256 (dán vào ô Băm khi công bố bản cập nhật):"
for f in dist/*.exe; do
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$f"
  else
    shasum -a 256 "$f"
  fi
done
