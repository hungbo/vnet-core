// Package authz giữ DANH MỤC QUYỀN của hệ thống — nguồn sự thật duy nhất cho
// cả router (gác API), cmd/seed (cài mới) và cmd/migrate (hệ thống đã chạy).
//
// Mỗi API của nhân viên gắn đúng một mã quyền. Trước đây chỉ có 11 quyền và
// chỉ 6 trong số đó gác được gì; mọi thứ còn lại chỉ cần "là nhân viên", nên
// một bạn trực quầy đổi được giá sản phẩm, sửa khuyến mãi, xoá nhóm máy.
//
// Quy ước mã: "<nhóm>.<việc>". Việc đọc là `view`; thêm/sửa/xoá là
// `create`/`update`/`delete`; thao tác đặc thù mang tên riêng (`topup`,
// `remote`, `dispatch`…) để cấp lẻ được.
package authz

// Permission là một dòng trong danh mục.
type Permission struct {
	Code   string
	Name   string
	Module string
}

// Wildcard là quyền siêu người dùng; middleware.PermissionRequired coi nó là
// qua tất. Chỉ vai trò chủ quán giữ mã này.
const Wildcard = "*"

// Catalog là toàn bộ quyền hệ thống hiểu được.
var Catalog = []Permission{
	{Wildcard, "Toàn quyền", "all"},

	{"members.view", "Xem hội viên", "members"},
	{"members.create", "Thêm hội viên", "members"},
	{"members.update", "Sửa hội viên", "members"},
	{"members.delete", "Xoá hội viên", "members"},
	{"members.topup", "Nạp tiền", "members"},
	{"members.refund", "Hoàn tiền", "members"},
	{"members.reset_password", "Đặt lại mật khẩu hội viên", "members"},
	{"members.refresh_tiers", "Xếp lại hạng", "members"},

	{"member_groups.view", "Xem nhóm hội viên", "member_groups"},
	{"member_groups.create", "Thêm nhóm hội viên", "member_groups"},
	{"member_groups.update", "Sửa nhóm hội viên", "member_groups"},
	{"member_groups.delete", "Xoá nhóm hội viên", "member_groups"},

	{"machines.view", "Xem máy", "machines"},
	{"machines.create", "Thêm máy", "machines"},
	{"machines.update", "Sửa máy", "machines"},
	{"machines.delete", "Xoá máy", "machines"},
	{"machines.remote", "Điều khiển máy từ xa", "machines"},

	{"machine_groups.view", "Xem nhóm máy", "machine_groups"},
	{"machine_groups.create", "Thêm nhóm máy", "machine_groups"},
	{"machine_groups.update", "Sửa nhóm máy", "machine_groups"},
	{"machine_groups.delete", "Xoá nhóm máy", "machine_groups"},

	{"machine_assets.view", "Xem tài sản máy", "machine_assets"},
	{"machine_assets.create", "Thêm tài sản máy", "machine_assets"},
	{"machine_assets.update", "Sửa tài sản máy", "machine_assets"},
	{"machine_assets.delete", "Xoá tài sản máy", "machine_assets"},

	{"pricing.view", "Xem bảng giá", "pricing"},
	{"pricing.create", "Thêm bảng giá", "pricing"},
	{"pricing.update", "Sửa bảng giá", "pricing"},
	{"pricing.delete", "Xoá bảng giá", "pricing"},

	{"sessions.view", "Xem phiên chơi", "sessions"},
	{"sessions.start", "Mở máy", "sessions"},
	{"sessions.end", "Trả máy", "sessions"},
	{"sessions.switch", "Chuyển máy", "sessions"},

	{"combos.view", "Xem gói dịch vụ", "combos"},
	{"combos.create", "Thêm gói dịch vụ", "combos"},
	{"combos.update", "Sửa gói dịch vụ", "combos"},
	{"combos.delete", "Xoá gói dịch vụ", "combos"},
	{"combos.sell", "Bán gói cho hội viên", "combos"},

	{"bookings.view", "Xem đặt chỗ", "bookings"},
	{"bookings.create", "Thêm đặt chỗ", "bookings"},
	{"bookings.update", "Sửa đặt chỗ", "bookings"},
	{"bookings.delete", "Xoá đặt chỗ", "bookings"},
	{"bookings.checkin", "Nhận khách đặt chỗ", "bookings"},

	{"promotions.view", "Xem khuyến mãi", "promotions"},
	{"promotions.create", "Thêm khuyến mãi", "promotions"},
	{"promotions.update", "Sửa khuyến mãi", "promotions"},
	{"promotions.delete", "Xoá khuyến mãi", "promotions"},

	{"lucky_spin.view", "Xem vòng quay", "lucky_spin"},
	{"lucky_spin.create", "Thêm phần thưởng", "lucky_spin"},
	{"lucky_spin.update", "Sửa phần thưởng", "lucky_spin"},
	{"lucky_spin.delete", "Xoá phần thưởng", "lucky_spin"},
	{"lucky_spin.spin", "Quay thưởng hộ khách", "lucky_spin"},

	{"curfew.view", "Xem giới nghiêm", "curfew"},
	{"curfew.create", "Thêm khung giới nghiêm", "curfew"},
	{"curfew.update", "Sửa khung giới nghiêm", "curfew"},
	{"curfew.delete", "Xoá khung giới nghiêm", "curfew"},
	{"curfew.override", "Miễn trừ giới nghiêm", "curfew"},

	{"categories.view", "Xem danh mục", "categories"},
	{"categories.create", "Thêm danh mục", "categories"},
	{"categories.update", "Sửa danh mục", "categories"},
	{"categories.delete", "Xoá danh mục", "categories"},

	{"products.view", "Xem sản phẩm", "products"},
	{"products.create", "Thêm sản phẩm", "products"},
	{"products.update", "Sửa sản phẩm (gồm đổi giá)", "products"},
	{"products.delete", "Xoá sản phẩm", "products"},
	{"products.ingredients", "Sửa định lượng nguyên liệu", "products"},

	{"orders.view", "Xem đơn hàng", "orders"},
	{"orders.create", "Tạo đơn hàng", "orders"},
	{"orders.update", "Sửa đơn hàng", "orders"},
	{"orders.delete", "Xoá đơn hàng", "orders"},
	{"orders.status", "Đổi trạng thái đơn", "orders"},
	{"orders.split", "Tách đơn", "orders"},
	{"orders.pay", "Thanh toán đơn", "orders"},
	{"orders.print", "In phiếu", "orders"},

	{"topup_cards.view", "Xem thẻ nạp", "cards"},
	{"topup_cards.generate", "Phát hành thẻ nạp", "cards"},
	{"topup_cards.cancel", "Huỷ thẻ nạp", "cards"},
	{"topup_cards.sell", "Bán thẻ nạp", "cards"},
	{"gift_cards.view", "Xem thẻ quà tặng", "cards"},
	{"gift_cards.generate", "Phát hành thẻ quà tặng", "cards"},
	{"gift_cards.cancel", "Huỷ thẻ quà tặng", "cards"},

	{"app_updates.view", "Xem bản cập nhật máy khách", "app_updates"},
	{"app_updates.create", "Công bố bản cập nhật", "app_updates"},
	{"app_updates.update", "Bật/tắt bản cập nhật", "app_updates"},
	{"app_updates.delete", "Xoá bản cập nhật", "app_updates"},

	{"website_rules.view", "Xem luật chặn web", "website_rules"},
	{"website_rules.create", "Thêm luật chặn web", "website_rules"},
	{"website_rules.update", "Sửa luật chặn web", "website_rules"},
	{"website_rules.delete", "Xoá luật chặn web", "website_rules"},

	{"feedback.view", "Xem đánh giá dịch vụ", "feedback"},
	{"attendance.view", "Xem điểm danh", "attendance"},

	{"inventory_counts.view", "Xem phiếu kiểm kê", "inventory"},
	{"inventory_counts.open", "Mở phiếu kiểm kê", "inventory"},
	{"inventory_counts.count", "Ghi số đếm", "inventory"},
	{"inventory_counts.commit", "Chốt kiểm kê", "inventory"},
	{"inventory_counts.cancel", "Huỷ phiếu kiểm kê", "inventory"},
	{"stock.view", "Xem sổ kho", "inventory"},
	{"stock.create", "Nhập/xuất kho", "inventory"},
	{"units.view", "Xem đơn vị tính", "inventory"},

	{"printers.view", "Xem máy in", "printers"},
	{"printers.create", "Thêm máy in", "printers"},
	{"printers.update", "Sửa máy in", "printers"},
	{"printers.delete", "Xoá máy in", "printers"},
	{"printers.test", "In thử", "printers"},

	{"suppliers.view", "Xem nhà cung cấp", "suppliers"},
	{"suppliers.create", "Thêm nhà cung cấp", "suppliers"},
	{"suppliers.update", "Sửa nhà cung cấp", "suppliers"},
	{"suppliers.delete", "Xoá nhà cung cấp", "suppliers"},

	{"shifts.view", "Xem ca làm việc", "shifts"},
	{"shifts.open", "Mở ca", "shifts"},
	{"shifts.close", "Đóng ca", "shifts"},
	{"shifts.handover", "Bàn giao ca", "shifts"},

	{"transactions.view", "Xem sổ giao dịch", "reports"},
	{"reports.view", "Xem báo cáo", "reports"},

	{"settings.view", "Xem cài đặt", "settings"},
	{"settings.edit", "Sửa cài đặt", "settings"},

	{"chat.moderate", "Quản trị phòng chat", "chat"},
	{"upload.file", "Tải tệp lên", "system"},

	{"audit.view", "Xem nhật ký hoạt động", "backoffice"},
	{"backups.view", "Xem bản sao lưu", "backoffice"},
	{"backups.create", "Tạo bản sao lưu", "backoffice"},
	{"backups.restore", "Phục hồi từ bản sao lưu", "backoffice"},
	{"backups.delete", "Xoá bản sao lưu", "backoffice"},
	{"notifications.view", "Xem thông báo đã soạn", "backoffice"},
	{"notifications.create", "Soạn thông báo", "backoffice"},
	{"notifications.update", "Sửa thông báo", "backoffice"},
	{"notifications.delete", "Xoá thông báo", "backoffice"},
	{"notifications.dispatch", "Gửi thông báo tới hội viên", "backoffice"},

	{"system.users.view", "Xem tài khoản nhân viên", "system"},
	{"system.users.create", "Thêm tài khoản nhân viên", "system"},
	{"system.users.update", "Sửa tài khoản nhân viên", "system"},
	{"system.users.delete", "Xoá tài khoản nhân viên", "system"},
	{"system.roles.view", "Xem vai trò", "system"},
	{"system.roles.create", "Thêm vai trò", "system"},
	{"system.roles.update", "Sửa vai trò", "system"},
	{"system.roles.delete", "Xoá vai trò", "system"},
	{"system.roles.permissions", "Phân quyền cho vai trò", "system"},
	{"system.menus.view", "Xem cây menu", "system"},

	// Giữ lại mã cũ: máy trạm Wails dùng nó để nhận diện kết nối quản trị, và
	// vài bản cài đã cấp nó cho vai trò riêng. Không còn gác API nào.
	{"client.admin", "Máy khách quản trị", "client"},
}

// BackOfficeModules là những nhóm chỉ chủ quán mới nên chạm tới: quản lý tài
// khoản, sao lưu, nhật ký, gửi thông báo hàng loạt.
var BackOfficeModules = map[string]bool{
	"system":     true,
	"backoffice": true,
}

// ManagerCodes: quản lý lo toàn bộ việc kinh doanh hằng ngày nhưng không chạm
// back-office — đúng ý định đã ghi trong cmd/seed từ đầu.
func ManagerCodes() []string {
	out := make([]string, 0, len(Catalog))
	for _, p := range Catalog {
		if p.Code == Wildcard || p.Code == "client.admin" || BackOfficeModules[p.Module] {
			continue
		}
		out = append(out, p.Code)
	}
	return out
}

// StaffCodes: nhân viên quầy VẬN HÀNH chứ không CẤU HÌNH. Xem được hầu hết mọi
// thứ để làm việc, mở/trả máy, bán hàng, nhận đặt chỗ, kiểm kê, mở/đóng ca —
// nhưng không đổi giá, không sửa khuyến mãi, không xoá dữ liệu nền.
func StaffCodes() []string {
	return []string{
		"members.view", "members.create", "members.topup",
		"member_groups.view",
		"machines.view", "machines.remote",
		"machine_groups.view", "machine_assets.view",
		"pricing.view",
		"sessions.view", "sessions.start", "sessions.end", "sessions.switch",
		"combos.view", "combos.sell",
		"bookings.view", "bookings.create", "bookings.update", "bookings.checkin",
		"promotions.view",
		"lucky_spin.view", "lucky_spin.spin",
		"curfew.view",
		"categories.view",
		"products.view",
		"orders.view", "orders.create", "orders.update", "orders.status", "orders.split",
		"orders.pay", "orders.print",
		"topup_cards.view", "topup_cards.sell",
		"gift_cards.view",
		"website_rules.view",
		"feedback.view", "attendance.view",
		"inventory_counts.view", "inventory_counts.open", "inventory_counts.count",
		"stock.view", "stock.create", "units.view",
		"printers.view", "printers.test",
		"suppliers.view",
		"shifts.view", "shifts.open", "shifts.close", "shifts.handover",
		"transactions.view",
		"settings.view",
	}
}

// Codes trả về toàn bộ mã trong danh mục.
func Codes() []string {
	out := make([]string, 0, len(Catalog))
	for _, p := range Catalog {
		out = append(out, p.Code)
	}
	return out
}

// Has cho biết một mã có nằm trong danh mục hay không. Router gọi gián tiếp qua
// test để bắt lỗi gõ sai: một mã không có trong danh mục nghĩa là API đó không
// ai cấp quyền được, tức khoá cứng tính năng.
func Has(code string) bool {
	for _, p := range Catalog {
		if p.Code == code {
			return true
		}
	}
	return false
}
