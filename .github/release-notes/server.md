**VNET Server @VERSION@** — bản **alpha** để chạy thử, có thể còn lỗi.

## Tải về

| Tệp | Dùng để |
|---|---|
| **`vnet-server-setup-@VERSION@.exe`** | Cài máy chủ VNET lên Windows 64-bit (Server 2016 trở lên, hoặc Windows 10/11). Có sẵn trang quản trị và PostgreSQL, không cần Docker |
| `SHA256SUMS.txt` | Mã SHA-256 để kiểm tệp tải về |

Máy trạm cài bằng bộ cài ở Release **VNET Client** mới nhất: https://github.com/@REPO@/releases

## Cài đặt

1. Chạy `vnet-server-setup-@VERSION@.exe` bằng quyền quản trị rồi bấm Next theo hướng dẫn. Mặc định cài vào `C:\VNET`.
2. Cài lần đầu: mật khẩu ngẫu nhiên của `admin`, `manager`, `staff` nằm trong `C:\VNET\data\initial-admin-password.txt`. Đăng nhập tại http://localhost:20800, đổi mật khẩu rồi xoá tệp đó.
3. Máy trạm trỏ về `http://<IP máy chủ>:20800`.
4. Cập nhật: chạy bộ cài của bản mới đè lên bản đang dùng — dữ liệu và cấu hình được giữ nguyên.

Hướng dẫn đầy đủ: https://github.com/@REPO@/blob/server-@VERSION@/docs/HUONG-DAN.md

## Windows cảnh báo khi mở tệp?

Bộ cài chưa ký số nên Windows SmartScreen hiện "Windows protected your PC": bấm **More info** rồi **Run anyway**. Muốn chắc tệp tải về đúng bản này, so mã SHA-256 bên dưới với kết quả của lệnh:

```powershell
Get-FileHash .\vnet-server-setup-@VERSION@.exe -Algorithm SHA256
```

## SHA-256

```
@SHA256SUMS@
```
