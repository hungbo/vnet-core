**VNET Client @VERSION@** — bản **alpha** của phần mềm máy trạm (Windows 64-bit) để chạy thử, có thể còn lỗi. Dùng cùng Release **VNET Server** mới nhất: https://github.com/@REPO@/releases

## Tải về

| Tệp | Dùng để |
|---|---|
| **`vnet-client-setup-@VERSION@.exe`** | Bộ cài cho máy trạm: hỏi địa chỉ máy chủ và tài khoản kỹ thuật, rồi đăng ký dịch vụ nền. **Cài mới thì dùng tệp này** |
| `vnet-client-amd64.exe` · `vnet-client-arm64.exe` | Tệp để **công bố bản cập nhật** ở trang quản trị → Cập nhật máy khách (máy trạm tự tải, kiểm SHA-256 rồi cài khi không có khách). Không cần tải nếu chỉ cài mới |
| `SHA256SUMS.txt` | Mã SHA-256 để kiểm tệp tải về, và để dán vào ô **Băm SHA-256** khi công bố bản cập nhật |

## Cài đặt

1. Cài máy chủ trước bằng Release **VNET Server**.
2. Trên máy trạm, chạy `vnet-client-setup-@VERSION@.exe` bằng quyền quản trị. Nhập địa chỉ máy chủ (ví dụ `http://192.168.1.10:20800`) và tài khoản + mật khẩu quản trị máy trạm (ít nhất 6 ký tự) để nhân viên kỹ thuật mở máy khi mất mạng.
3. Mã máy chính là tên máy Windows. Máy tự xuất hiện ở trang **Máy** sau lần nối đầu tiên — xếp nhóm cho máy thì khách mới đăng nhập được.

Hướng dẫn đầy đủ: https://github.com/@REPO@/blob/client-@VERSION@/docs/HUONG-DAN.md

## Windows cảnh báo khi mở tệp?

Bộ cài chưa ký số nên Windows SmartScreen hiện "Windows protected your PC": bấm **More info** rồi **Run anyway**. Muốn chắc tệp tải về đúng bản này, so mã SHA-256 bên dưới với kết quả của lệnh:

```powershell
Get-FileHash .\vnet-client-setup-@VERSION@.exe -Algorithm SHA256
```

## SHA-256

```
@SHA256SUMS@
```
