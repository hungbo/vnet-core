package service

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// ErrRangBuoc đánh dấu "không xoá được vì còn dữ liệu phụ thuộc".
//
// Handler dùng errors.Is để trả 409 thay cho 400. Đây không phải lỗi của người
// gọi — yêu cầu hoàn toàn hợp lệ, chỉ là dữ liệu không cho phép.
var ErrRangBuoc = errors.New("ràng buộc dữ liệu")

// loiRangBuoc mang câu tiếng Việt đầy đủ nhưng vẫn khớp errors.Is(ErrRangBuoc).
//
// Không dùng fmt.Errorf("%w: ...") vì cách đó nhét tiền tố "ràng buộc dữ liệu: "
// vào đầu Error(), và chuỗi đó hiện thẳng lên toast của trang quản trị.
type loiRangBuoc struct{ msg string }

func (e *loiRangBuoc) Error() string { return e.msg }
func (e *loiRangBuoc) Unwrap() error { return ErrRangBuoc }

// phuThuoc mô tả MỘT bảng con trỏ tới bản ghi sắp xoá.
type phuThuoc struct {
	// Bang là con trỏ model rỗng, ví dụ &model.MachineSession{}. GORM tự suy ra
	// tên bảng và tự thêm "deleted_at IS NULL" cho model xoá mềm — nên KHÔNG
	// được viết tay điều kiện đó, viết tay sẽ sai với những bảng xoá cứng.
	Bang interface{}
	// Cot là tên cột khoá ngoại, ví dụ "machine_id".
	Cot string
	// Nhan là tên tiếng Việt số nhiều, không viết hoa, ví dụ "phiên chơi".
	Nhan string
}

// kiemTraPhuThuoc chặn xoá khi còn bản ghi phụ thuộc.
//
// Cơ sở dữ liệu KHÔNG có khoá ngoại nào: 68 tag constraint:OnDelete trong
// internal/model đặt trên trường ID vô hướng, mà GORM chỉ đọc tag đó trên trường
// quan hệ (không model nào khai). Và kể cả nếu có, 18 model gốc xoá mềm nên
// ON DELETE không bao giờ chạy. Toàn vẹn tham chiếu phải làm ở đây.
//
// Dừng ở phụ thuộc ĐẦU TIÊN còn dữ liệu, không gộp — nên xếp phụ thuộc dễ gặp
// nhất lên đầu ds, thông điệp đầu tiên phải là thông điệp hữu ích nhất.
//
// goiY là lối thoát cho người vận hành, ví dụ "hãy tắt hoạt động máy thay vì
// xoá". Không có nó thì người dùng bị chặn mà không biết phải làm gì.
func kiemTraPhuThuoc(db *gorm.DB, id string, ds []phuThuoc, goiY string) error {
	for _, p := range ds {
		var n int64
		// Lỗi của Count PHẢI kiểm: bỏ qua nó nghĩa là database sập cũng đọc ra
		// n = 0 và bản ghi bị xoá — đúng cái mà hàm này sinh ra để chặn.
		if err := db.Model(p.Bang).Where(p.Cot+" = ?", id).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return &loiRangBuoc{fmt.Sprintf("không xoá được: còn %d %s — %s", n, p.Nhan, goiY)}
		}
	}
	return nil
}

// chanVi trả lỗi ràng buộc cho những trường hợp chặn theo TRẠNG THÁI của chính
// bản ghi (đơn đã thanh toán, lịch đặt đã nhận máy) chứ không theo số bản ghi
// con. Cùng sentinel nên handler xử lý chung một đường.
func chanVi(format string, a ...interface{}) error {
	return &loiRangBuoc{fmt.Sprintf(format, a...)}
}
