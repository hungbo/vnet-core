#!/bin/bash
set -e

# Bộ cài máy chủ VNET cho Windows: dist/vnet-server-setup-<phiên bản>.exe
#
# Một tệp chứa đủ thứ để quán "bật lên là dùng" trên Windows Server mà không
# cần Docker (Server 2019 không chạy được container Linux):
#   vnet-server.exe   API + trang quản trị nhúng sẵn, chạy như dịch vụ Windows
#   vnet-migrate.exe, vnet-seed.exe
#   pgsql\            PostgreSQL bản portable (có pg_dump/pg_restore cho sao lưu)
#   setup\            vc_redist.x64.exe + vnet-server-install.ps1
#
# Cập nhật phiên bản: build lại với số mới, chạy tệp setup mới trên máy chủ.
#
#   bash scripts/build-server-windows.sh 1.2.0
#
# Cần: Go 1.25+, pnpm, makensis (brew install makensis), curl, unzip.

if ! command -v go >/dev/null 2>&1; then
  for candidate in "$HOME"/sdk/go*/bin /usr/local/go/bin /opt/homebrew/opt/go/libexec/bin; do
    if [ -x "$candidate/go" ]; then
      export PATH="$candidate:$PATH"
      break
    fi
  done
fi
for tool in go pnpm makensis curl unzip; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "Thiếu $tool. makensis: brew install makensis (hoặc apt install nsis)." >&2
    exit 1
  fi
done
echo "-> Go: $(go version)"

VERSION=${1:-dev}
# Cùng bản chính với image postgres:16-alpine trong docker-compose.yml. ĐỔI BẢN
# CHÍNH (16 → 17) là phá dữ liệu quán đang chạy: database cũ cần pg_upgrade, và
# bộ cài sẽ từ chối cài đè. Đổi bản vá (16.10 → 16.11) thì an toàn — nhớ cập
# nhật cả băm.
PG_VERSION=16.10-1
PG_SHA256=ebb3b6af4fa69dea9951b66855bc4d42dc04e56ccb9aa7024ce3c58bd89d6b0c

ROOT=$(cd "$(dirname "$0")/.." && pwd)
CACHE=${VNET_BUILD_CACHE:-$HOME/.cache/vnet-build}
STAGE="$ROOT/dist/vnet-server-windows"
ICON="$ROOT/client/src/build/windows/icon.ico"
OUT="$ROOT/dist/vnet-server-setup-$VERSION.exe"

# Số phiên bản dạng a.b.c.d cho thông tin tệp của Windows; "dev" thành 0.0.0.0.
WIN_VER=$(echo "$VERSION" | sed 's/^v//')
echo "$WIN_VER" | grep -Eq '^[0-9]+(\.[0-9]+){0,3}$' || WIN_VER=0.0.0.0
while [ "$(echo "$WIN_VER" | tr -cd . | wc -c)" -lt 3 ]; do WIN_VER="$WIN_VER.0"; done

echo "=== Building VNET Server (Windows) v$VERSION ==="

# ------------------------------------------------------------------ tải về
mkdir -p "$CACHE"
PG_ZIP="$CACHE/postgresql-$PG_VERSION-windows-x64-binaries.zip"
if [ ! -f "$PG_ZIP" ]; then
  echo "-> Tải PostgreSQL $PG_VERSION (~320 MB, chỉ lần đầu)..."
  curl -fL --retry 3 -o "$PG_ZIP.part" "https://get.enterprisedb.com/postgresql/postgresql-$PG_VERSION-windows-x64-binaries.zip"
  mv "$PG_ZIP.part" "$PG_ZIP"
fi
echo "$PG_SHA256  $PG_ZIP" | shasum -a 256 -c - >/dev/null || {
  echo "LỖI: băm của $PG_ZIP không khớp — xoá tệp đó rồi build lại." >&2
  exit 1
}

# Tải mới mỗi lần build: Microsoft thay tệp sau cùng một đường dẫn và ký số nó,
# nên không ghim băm được — và cũ thì kém hơn mới.
VCREDIST="$CACHE/vc_redist.x64.exe"
echo "-> Tải Visual C++ Runtime..."
curl -fsSL --retry 3 -o "$VCREDIST.part" "https://aka.ms/vs/17/release/vc_redist.x64.exe" && mv "$VCREDIST.part" "$VCREDIST"

# -------------------------------------------------------------- trang quản trị
echo "-> Building Admin UI..."
cd "$ROOT/admin"
# .env mang mã phản hồi 8888/7777/9999 và chế độ route; build thiếu nó cho ra
# trang quản trị hỏng (xem Dockerfile).
test -f .env || { echo "LỖI: thiếu admin/.env" >&2; exit 1; }
# Chỉ cài khi chưa có, và gọi thẳng vite thay vì `pnpm build`: pnpm 11 chạy
# lại `pnpm install` trước mỗi script, rồi dừng hẳn vì các mục allowBuilds
# trong pnpm-workspace.yaml còn để trống — dù node_modules đã đầy đủ.
[ -d node_modules ] || pnpm install --frozen-lockfile
./node_modules/.bin/vite build --mode prod
# Dọn bản build cũ trước khi chép: vite đặt tên tệp theo băm nội dung, nên chép
# đè cứ cộng dồn — thư mục này từng phình tới 400 MB và nhét hết vào .exe.
# index.html được giữ vì đó là tệp giữ chỗ có trong git.
mkdir -p ../backend/cmd/server/embed
find ../backend/cmd/server/embed -mindepth 1 -maxdepth 1 ! -name index.html -exec rm -rf {} +
cp -r dist/* ../backend/cmd/server/embed/

# ---------------------------------------------------------------------- Go
echo "-> Building Go binaries..."
cd "$ROOT/backend"
# Icon và thông tin phiên bản cho vnet-server.exe — cùng icon với máy khách.
# Tệp .syso cạnh mã nguồn thì `go build` tự nhúng; xoá ngay để không lọt vào git.
WINRES_DIR=$(mktemp -d)
cp "$ICON" "$WINRES_DIR/icon.ico"
cat > "$WINRES_DIR/winres.json" <<JSON
{
  "RT_GROUP_ICON": { "APP": { "0000": "icon.ico" } },
  "RT_VERSION": {
    "#1": {
      "0000": {
        "fixed": { "file_version": "$WIN_VER", "product_version": "$WIN_VER" },
        "info": {
          "0409": {
            "CompanyName": "VNET",
            "FileDescription": "VNET Server",
            "FileVersion": "$WIN_VER",
            "LegalCopyright": "VNET",
            "OriginalFilename": "vnet-server.exe",
            "ProductName": "VNET Server",
            "ProductVersion": "$WIN_VER"
          }
        }
      }
    }
  }
}
JSON
(cd cmd/server && go run github.com/tc-hib/go-winres@v0.3.3 make \
    --in "$WINRES_DIR/winres.json" --arch amd64 --out rsrc)
rm -rf "$WINRES_DIR"

rm -rf "$STAGE" && mkdir -p "$STAGE/setup"
# -tags timetzdata: Windows không có cơ sở dữ liệu múi giờ của Go. Thiếu nó thì
# LoadLocation("Asia/Ho_Chi_Minh") thất bại, và giá theo giờ, giờ giới nghiêm,
# lịch chạy job lệch sang UTC — không báo lỗi gì.
build() {
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -tags timetzdata \
    -ldflags="-s -w $2" -o "$STAGE/$1.exe" "./cmd/$3"
}
build vnet-server "-X main.version=$VERSION" server
rm -f cmd/server/rsrc_windows_*.syso
build vnet-migrate "" migrate
build vnet-seed "" seed

# ---------------------------------------------------------------- gom tệp
echo "-> Gom PostgreSQL..."
# Chỉ phần máy chủ: bỏ pgAdmin, StackBuilder, tài liệu và header (~700 MB).
unzip -q "$PG_ZIP" 'pgsql/bin/*' 'pgsql/lib/*' 'pgsql/share/*' \
  -x 'pgsql/bin/stackbuilder*' 'pgsql/bin/wx*' 'pgsql/lib/*.lib' -d "$STAGE"

cp "$VCREDIST" "$STAGE/setup/vc_redist.x64.exe"
cp "$ROOT/installer/vnet-server-install.ps1" "$STAGE/setup/"

echo "-> Đóng gói bộ cài..."
# makensis đọc tệp theo locale: dưới locale C (mặc định của nhiều shell, cả CI)
# nó báo "Bad text encoding" ở dòng tiếng Việt đầu tiên.
LC_ALL=en_US.UTF-8 makensis -V2 \
  "-DVERSION=$VERSION" "-DWINVER=$WIN_VER" \
  "-DSTAGE=$STAGE" "-DICON=$ICON" "-DOUTFILE=$OUT" \
  "$ROOT/installer/vnet-server.nsi"

echo "=== Done: $OUT ==="
ls -lh "$OUT"
shasum -a 256 "$OUT"
