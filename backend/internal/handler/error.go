package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/vnet/core/pkg/response"
)

var constraintMessages = map[string]string{
	"idx_product_material":           "Nguyên liệu đã được thêm vào sản phẩm này",
	"idx_machine_groups_name":        "Tên nhóm máy đã tồn tại",
	"idx_machines_machine_code":      "Mã máy đã tồn tại",
	"idx_categories_name":            "Tên danh mục đã tồn tại",
	"idx_products_name":              "Tên sản phẩm đã tồn tại",
	"idx_combos_name":                "Tên combo đã tồn tại",
	"idx_product_option_groups_name": "Tên nhóm tuỳ chọn đã tồn tại",
	"idx_option_group_name":          "Tên tuỳ chọn đã tồn tại trong nhóm",
	"uni_members_username":           "Tên đăng nhập hội viên đã tồn tại",
	"uni_users_username":             "Tên đăng nhập đã tồn tại",
	"uni_roles_name":                 "Tên vai trò đã tồn tại",
	"uni_permissions_code":           "Mã quyền đã tồn tại",
	"uni_stores_code":                "Mã cửa hàng đã tồn tại",
	"uni_orders_order_code":          "Mã đơn hàng đã tồn tại",
	"uni_topup_cards_code":           "Mã thẻ nạp đã tồn tại",
	"uni_gift_cards_code":            "Mã thẻ quà tặng đã tồn tại",
}

var validationFieldLabels = map[string]string{
	"MachineCode":       "Mã máy",
	"MachineID":         "Máy",
	"MemberID":          "Hội viên",
	"GroupID":           "Nhóm",
	"CategoryID":        "Danh mục",
	"ProductID":         "Sản phẩm",
	"ParentID":          "Danh mục cha",
	"OrderID":           "Đơn hàng",
	"PrinterID":         "Máy in",
	"ComboID":           "Combo",
	"SupplierID":        "Nhà cung cấp",
	"MaterialID":        "Nguyên liệu",
	"WarehouseID":       "Kho",
	"UnitID":            "Đơn vị tính",
	"RoleID":            "Vai trò",
	"PermissionID":      "Quyền",
	"Name":              "Tên",
	"FullName":          "Họ tên",
	"Username":          "Tên đăng nhập",
	"PasswordHash":      "Mật khẩu",
	"Email":             "Email",
	"Phone":             "Số điện thoại",
	"Code":              "Mã",
	"OrderCode":         "Mã đơn hàng",
	"Price":             "Giá",
	"PricePerHour":      "Giá theo giờ",
	"TotalAmount":       "Tổng tiền",
	"FinalAmount":       "Thành tiền",
	"DiscountAmount":    "Giảm giá",
	"Amount":            "Số tiền",
	"Quantity":          "Số lượng",
	"UnitPrice":         "Đơn giá",
	"Subtotal":          "Tạm tính",
	"DepositAmount":     "Tiền cọc",
	"FaceValue":         "Mệnh giá",
	"Balance":           "Số dư",
	"InitialBalance":    "Số dư ban đầu",
	"Type":              "Loại",
	"Status":            "Trạng thái",
	"Description":       "Mô tả",
	"Notes":             "Ghi chú",
	"Address":           "Địa chỉ",
	"Icon":              "Biểu tượng",
	"ImageURL":          "Hình ảnh",
	"AvatarURL":         "Ảnh đại diện",
	"SortOrder":         "Thứ tự",
	"IsActive":          "Kích hoạt",
	"IsDefault":         "Mặc định",
	"StartTime":         "Giờ bắt đầu",
	"EndTime":           "Giờ kết thúc",
	"StartedAt":         "Bắt đầu lúc",
	"EndedAt":           "Kết thúc lúc",
	"EffectiveFrom":     "Hiệu lực từ",
	"EffectiveTo":       "Hiệu lực đến",
	"ValidFrom":         "Hiệu lực từ",
	"ValidTo":           "Hiệu lực đến",
	"SlotStart":         "Giờ bắt đầu",
	"SlotEnd":           "Giờ kết thúc",
	"TotalMinutes":      "Tổng số phút",
	"ValidityDays":      "Số ngày hiệu lực",
	"RemainingMinutes":  "Phút còn lại",
	"DurationMinutes":   "Thời lượng",
	"MemberPrefix":      "Tiền tố hội viên",
	"MemberCount":       "Số lượng hội viên",
	"CustomerName":      "Tên khách hàng",
	"CustomerPhone":     "Số điện thoại khách",
	"AssetType":         "Loại tài sản",
	"Brand":             "Thương hiệu",
	"Model":             "Model",
	"Serial":            "Số serial",
	"RewardType":        "Loại thưởng",
	"RewardValue":       "Giá trị thưởng",
	"Probability":       "Xác suất",
	"MaxPerDay":         "Tối đa mỗi ngày",
	"ConditionKey":      "Điều kiện",
	"ConditionValue":    "Giá trị điều kiện",
	"PaymentMethod":     "Phương thức thanh toán",
	"ReferenceCode":     "Mã tham chiếu",
	"ReferenceID":       "Mã tham chiếu",
	"TransactionType":   "Loại giao dịch",
	"HandoverType":      "Loại bàn giao",
	"PrinterType":       "Loại máy in",
	"IPAddress":         "Địa chỉ IP",
	"RuleType":          "Loại quy tắc",
	"Pattern":           "Mẫu",
	"DayOfWeek":         "Thứ trong tuần",
	"CurfewStart":       "Giờ giới nghiêm bắt đầu",
	"CurfewEnd":         "Giờ giới nghiêm kết thúc",
	"MaxMinorHours":     "Giờ tối đa cho trẻ vị thành niên",
	"FileName":          "Tên file",
	"FilePath":          "Đường dẫn file",
	"FileURL":           "Đường dẫn file",
	"Version":           "Phiên bản",
	"Platform":          "Nền tảng",
	"Changelog":         "Ghi chú thay đổi",
	"Role":              "Vai trò",
}

func handleCreateError(c *gin.Context, err error) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if msg, ok := constraintMessages[pgErr.ConstraintName]; ok {
			response.BadRequest(c, msg)
			return
		}
		response.BadRequest(c, fmt.Sprintf("Dữ liệu '%s' đã tồn tại", pgErr.ConstraintName))
		return
	}
	handleValidationError(c, err)
}

func handleValidationError(c *gin.Context, err error) {
	var verr validator.ValidationErrors
	if errors.As(err, &verr) {
		fe := verr[0]
		response.BadRequest(c, validationErrorMessage(fe))
		return
	}
	var jerr *json.UnmarshalTypeError
	if errors.As(err, &jerr) {
		response.BadRequest(c, fmt.Sprintf("Trường '%s' không đúng định dạng", jerr.Field))
		return
	}
	var serr *json.SyntaxError
	if errors.As(err, &serr) {
		response.BadRequest(c, "Dữ liệu JSON không hợp lệ")
		return
	}
	response.BadRequest(c, "Dữ liệu không hợp lệ")
}

func validationErrorMessage(fe validator.FieldError) string {
	label := fieldLabel(fe.Field())
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s không được để trống", label)
	case "email":
		return "Email không hợp lệ"
	case "min":
		if fe.Type().Kind() == reflect.String {
			return fmt.Sprintf("%s tối thiểu %s ký tự", label, fe.Param())
		}
		return fmt.Sprintf("%s tối thiểu là %s", label, fe.Param())
	case "max":
		if fe.Type().Kind() == reflect.String {
			return fmt.Sprintf("%s tối đa %s ký tự", label, fe.Param())
		}
		return fmt.Sprintf("%s tối đa là %s", label, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s không hợp lệ", label)
	case "numeric":
		return fmt.Sprintf("%s phải là số", label)
	case "boolean":
		return fmt.Sprintf("%s phải là đúng/sai", label)
	default:
		return fmt.Sprintf("%s không hợp lệ", label)
	}
}

func fieldLabel(field string) string {
	if label, ok := validationFieldLabels[field]; ok {
		return label
	}
	// Convert PascalCase to readable label
	var result []rune
	for i, r := range field {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, ' ')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}
