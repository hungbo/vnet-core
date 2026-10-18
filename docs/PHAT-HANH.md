# Phát hành bản mới (GitHub Release)

Đẩy tag lên GitHub thì GitHub Actions (`.github/workflows/release.yml`) tự dựng bộ cài và đăng vào mục **Releases**. Máy chủ và máy khách có số phiên bản riêng nên có tag riêng.

| Tag | Release | Tệp đính kèm |
|---|---|---|
| `server-X.Y.Z` | VNET Server X.Y.Z | `vnet-server-setup-X.Y.Z.exe`, `SHA256SUMS.txt` |
| `client-X.Y.Z` | VNET Client X.Y.Z | `vnet-client-setup-X.Y.Z.exe`, `vnet-client-amd64.exe`, `vnet-client-arm64.exe`, `SHA256SUMS.txt` |

Tên tệp không có chữ `v`. Giai đoạn alpha, mọi Release đều gắn nhãn **Pre-release**. Hai tệp `vnet-client-<kiến trúc>.exe` là tệp dùng để công bố bản cập nhật ở trang quản trị.

## Quy trình

1. Tính năng đã commit, push và qua phần *Verification* trong [AGENTS.md](../AGENTS.md).
2. Cắt tag (annotated) trên đúng commit đó rồi push. Chỉ cắt tag của phần có thay đổi:

   ```bash
   git tag -a server-1.1.18 -m "server 1.1.18"
   git tag -a client-1.0.20 -m "client 1.0.20"
   git push origin server-1.1.18 client-1.0.20
   ```

3. Theo dõi ở tab **Actions** → *Release*. Server là phần lâu nhất (dựng trang quản trị, tải PostgreSQL ~320 MB khi cache trống).
4. Mở trang Releases, tải bộ cài về, so SHA-256, cài thử lên một máy.
5. Bản ổn thì bỏ nhãn: `gh release edit server-1.1.18 --prerelease=false`.
6. Máy trạm đã cài: công bố bản cập nhật ở trang quản trị → **Cập nhật máy khách**, dùng `vnet-client-amd64.exe` và SHA-256 trong `SHA256SUMS.txt` (xem [HUONG-DAN.md](HUONG-DAN.md)).

Tag chỉ chạy workflow nằm **trong commit mà nó trỏ tới**. Các tag cắt trước khi có `release.yml` (đến `server-1.1.17` và `client-1.0.19`) sẽ không có Release.

## Chạy thử mà không phát hành

Đẩy một nhánh tên `ci/<gì-đó>`: workflow dựng đủ các bộ cài với số phiên bản `0.0.0-ci` nhưng **không** tạo Release. Tệp nằm ở mục *Artifacts* cuối trang lần chạy, giữ 3 ngày. Dùng khi sửa `release.yml` hoặc script build, để khỏi cắt tag chỉ để thử.

## Khi lỗi

- **Job dựng đỏ, chưa có Release:** sửa, commit, rồi cắt lại tag. Tag đang trỏ vào commit cũ nên phải xoá trước:

  ```bash
  git push origin :refs/tags/server-1.1.18 && git tag -d server-1.1.18
  ```

- **Release đã đăng nhưng tệp sai:** `gh release delete server-1.1.18 --cleanup-tag --yes`, sửa rồi cắt lại. Đừng thay tệp trên Release đã đăng: người tải trước đó giữ bản cũ mà số phiên bản vẫn trùng.

## Những thứ được ghim

| Thứ | Nằm ở | Lưu ý |
|---|---|---|
| PostgreSQL portable | `PG_VERSION`, `PG_SHA256` trong `scripts/build-server-windows.sh` | Đổi bản **chính** (16 → 17) là phá dữ liệu quán đang chạy |
| TightVNC (remote desktop) | URL và SHA-256 trong job `client-installer` | `installer/tightvnc.msi` bị `.gitignore`; thiếu nó bộ cài vẫn build nhưng máy trạm không remote được, nên CI tải về và kiểm băm |
| pnpm | `version` trong job `server` | Cùng bản chính với máy phát triển: `admin/pnpm-workspace.yaml` dùng khoá `allowBuilds` của pnpm 11 |
| Go | dòng `go` trong `backend/go.mod`, `client/go.mod` | |
| Chữ trong ghi chú Release | `.github/release-notes/server.md`, `client.md` | |

## Dựng tay trên máy (khi CI hỏng)

Hai script ghi thẳng vào `dist/`, `client/dist/` và `backend/cmd/server/embed/` (kể cả `index.html` đang được git theo dõi), và `build-client.sh` còn `rm -rf client/dist`. Dựng trong một worktree tại đúng tag để không đụng cây làm việc:

```bash
git worktree add ../vnet-release server-1.1.18
cd ../vnet-release
bash scripts/build-server-windows.sh 1.1.18      # cần makensis; ra dist/vnet-server-setup-1.1.18.exe
bash scripts/build-client.sh 1.0.20              # ra client/dist/vnet-client-{amd64,arm64}.exe
```

Với `build-client.sh`, tham số thứ hai là **một chuỗi** (`"amd64 arm64"`); để trống thì dựng cả hai. Bộ cài máy khách (Inno Setup) chỉ dựng được trên Windows.

Bộ cài chưa ký số nên Windows SmartScreen sẽ cảnh báo lúc mở; ghi chú Release đã dặn người dùng bấm *More info* → *Run anyway*.
