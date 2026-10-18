// Package model chứa các struct GORM ánh xạ thẳng xuống bảng database.
//
// # Không có khoá ngoại
//
// Cơ sở dữ liệu này gần như KHÔNG có ràng buộc khoá ngoại nào. Các cột như
// MachineSession.MachineID hay Order.MemberID chỉ là cột uuid có index, không
// có REFERENCES: database sẵn sàng nhận một machine_id trỏ tới hư không.
//
// Trước đây mọi cột khoá ngoại đều mang tag `constraint:OnUpdate:CASCADE,
// OnDelete:CASCADE` (hoặc SET NULL / RESTRICT) — tổng cộng 68 cái. Không tag
// nào trong số đó từng có tác dụng: GORM chỉ đọc `constraint:` trên TRƯỜNG QUAN
// HỆ (một trường kiểu struct hoặc slice, ví dụ `Member Member`), còn ở đây
// chúng được viết trên trường ID vô hướng. Toàn bộ đã được gỡ bỏ vì đọc vào chỉ
// khiến người sau tin rằng hệ thống có thứ nó không có.
//
// Hai ngoại lệ, là hai khoá ngoại THẬT duy nhất: bảng nối user_roles và
// role_permissions, do các trường many2many trong user.go sinh ra. Cả hai mang
// hành vi mặc định NO ACTION, tức là CHẶN chứ không cascade.
//
// # Vậy toàn vẹn tham chiếu nằm ở đâu
//
// Ở tầng service, trong internal/service/rang_buoc.go: hàm kiemTraPhuThuoc đếm
// bản ghi phụ thuộc trước khi cho xoá, còn từng hàm Delete tự dọn những bảng con
// thuộc quyền sở hữu của mình trong một transaction. Đọc file đó trước khi thêm
// một quan hệ mới.
//
// # Vì sao không thêm khoá ngoại thật
//
// Mười tám model gốc dùng xoá mềm (gorm.DeletedAt): Member, Machine, Product,
// Order, User, Category, Combo, Supplier, Promotion, PrinterConfig,
// MachineGroup và những model khác. Với chúng, db.Delete phát ra
// `UPDATE ... SET deleted_at = now()` chứ không phải DELETE, nên mệnh đề
// ON DELETE không bao giờ có cơ hội chạy. Thêm khoá ngoại vào sẽ tốn công mà
// không giải quyết được đúng vấn đề — vấn đề nằm ở chỗ hàm xoá không kiểm tra
// gì cả.
//
// Nếu sau này bạn muốn ràng buộc thật ở tầng database, phải làm hai việc cùng
// lúc: thêm một trường quan hệ bên cạnh cột ID — chẳng hạn trong Order thêm
// trường Member kiểu Member với tag gorm foreignKey:MemberID và
// constraint:OnDelete:SET NULL — VÀ dọn dữ liệu mồ côi đang có, nếu không
// ALTER TABLE sẽ thất bại ngay khi migrate.
package model
