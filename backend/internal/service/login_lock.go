package service

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Khoá tài khoản khi gõ sai mật khẩu LIÊN TIẾP, áp cho cả nhân viên (users) lẫn
// hội viên (members):
//
//	sai lần thứ 3 → khoá 1 phút
//	sai lần thứ 4 → khoá 5 phút
//	sai lần thứ 5 → khoá hẳn (is_active = false), quản lý phải kích hoạt lại
//
// Đăng nhập đúng thì bộ đếm về 0. Đang bị khoá tạm thì không kiểm mật khẩu và
// không đếm thêm — bấm liên tục không làm khoá dài thêm.
//
// Bộ đếm nằm trong database chứ không trong bộ nhớ như loginThrottle (chặn theo
// cặp tên + IP): khởi động lại máy chủ hay đổi máy dò thì vẫn còn nguyên.
//
// Ngoại lệ: tài khoản nhân viên có siêu quyền "*" (chủ quán) chỉ bị khoá tạm 5
// phút mỗi lần, không bao giờ khoá hẳn. Ai cũng gõ được tên "admin" ở màn đăng
// nhập; khoá hẳn tài khoản đó là không còn ai đủ quyền mở khoá cho chính nó.
const (
	saiLanKhoa1Phut = 3
	saiLanKhoa5Phut = 4
	saiLanKhoaHan   = 5
)

// LoginLockedError là lỗi trả cho người đang bị khoá. Thông báo nói rõ chờ bao
// lâu hay phải nhờ ai, thay cho "sai mật khẩu" chung chung.
type LoginLockedError struct {
	Until     time.Time // mốc hết khoá tạm; rỗng khi khoá hẳn
	Permanent bool
}

func (e *LoginLockedError) Error() string {
	if e.Permanent {
		return "tài khoản đã bị khoá do nhập sai mật khẩu nhiều lần — liên hệ quản lý để kích hoạt lại"
	}
	con := time.Until(e.Until).Round(time.Second)
	if con < time.Second {
		con = time.Second
	}
	phut, giay := int(con/time.Minute), int(con%time.Minute/time.Second)
	cho := fmt.Sprintf("%d giây", giay)
	switch {
	case phut > 0 && giay > 0:
		cho = fmt.Sprintf("%d phút %d giây", phut, giay)
	case phut > 0:
		cho = fmt.Sprintf("%d phút", phut)
	}
	return "nhập sai mật khẩu nhiều lần — tài khoản tạm khoá, thử lại sau " + cho
}

// IsLoginLocked cho handler biết lỗi đăng nhập là do khoá chứ không phải sai mật khẩu.
func IsLoginLocked(err error) bool {
	var l *LoginLockedError
	return errors.As(err, &l)
}

// khoaDangNhap là trạng thái khoá của một tài khoản vừa đọc lên.
type khoaDangNhap struct {
	bang        string // "users" hoặc "members"
	id          string
	isActive    bool
	failed      int
	lockedUntil *time.Time
	sieuQuyen   bool // chủ quán: không khoá hẳn
}

// truocKhiKiem chạy TRƯỚC khi so mật khẩu. Trả lỗi nếu tài khoản đang bị khoá.
func (k khoaDangNhap) truocKhiKiem(db *gorm.DB, now time.Time) error {
	if !k.isActive {
		if k.failed >= saiLanKhoaHan {
			return &LoginLockedError{Permanent: true}
		}
		return nil // bị tắt vì lý do khác — người gọi tự báo như cũ
	}
	// Đang hoạt động mà bộ đếm đã tới mức khoá hẳn: quản lý vừa kích hoạt lại.
	// Đưa về 0, không thì gõ nhầm một lần là khoá hẳn lại ngay.
	if k.failed >= saiLanKhoaHan && !k.sieuQuyen {
		return db.Table(k.bang).Where("id = ?", k.id).
			Updates(map[string]interface{}{"failed_logins": 0, "locked_until": nil}).Error
	}
	if k.lockedUntil != nil && k.lockedUntil.After(now) {
		return &LoginLockedError{Until: *k.lockedUntil}
	}
	return nil
}

// sauKhiSai ghi một lần sai và khoá nếu tới ngưỡng. Trả lỗi để báo cho người
// dùng: ErrBadCredentials, hoặc lỗi khoá nếu chính lần sai này làm tài khoản bị khoá.
//
// Cộng bằng một câu UPDATE ... RETURNING: hai lượt sai gửi cùng lúc đều được đếm,
// không lượt nào ghi đè số của lượt kia.
func (k khoaDangNhap) sauKhiSai(db *gorm.DB, now time.Time) error {
	var n int
	if err := db.Raw("UPDATE "+k.bang+" SET failed_logins = failed_logins + 1 WHERE id = ? RETURNING failed_logins", k.id).
		Scan(&n).Error; err != nil {
		return err
	}
	switch {
	case n >= saiLanKhoaHan && !k.sieuQuyen:
		if err := db.Table(k.bang).Where("id = ?", k.id).
			Updates(map[string]interface{}{"is_active": false, "locked_until": nil}).Error; err != nil {
			return err
		}
		return &LoginLockedError{Permanent: true}
	case n >= saiLanKhoa5Phut:
		return k.khoaTam(db, now.Add(5*time.Minute))
	case n >= saiLanKhoa1Phut:
		return k.khoaTam(db, now.Add(time.Minute))
	}
	return ErrBadCredentials
}

func (k khoaDangNhap) khoaTam(db *gorm.DB, until time.Time) error {
	if err := db.Table(k.bang).Where("id = ?", k.id).Update("locked_until", until).Error; err != nil {
		return err
	}
	return &LoginLockedError{Until: until}
}

// sauKhiDung xoá bộ đếm khi đăng nhập đúng. Không tốn truy vấn nếu chưa sai lần nào.
func (k khoaDangNhap) sauKhiDung(db *gorm.DB) {
	if k.failed == 0 && k.lockedUntil == nil {
		return
	}
	db.Table(k.bang).Where("id = ?", k.id).
		Updates(map[string]interface{}{"failed_logins": 0, "locked_until": nil})
}
