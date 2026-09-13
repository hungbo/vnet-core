package service

import (
	"errors"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

type SystemManageService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewSystemManageService(db *gorm.DB, audit *AuditService) *SystemManageService {
	return &SystemManageService{db: db, audit: audit}
}

type SystemListParams struct {
	Current int    `form:"current"`
	Size    int    `form:"size"`
	Search  string `form:"search"`
	Sort    string `form:"sort"`
	Order   string `form:"order"`
}

type UserListParams struct {
	SystemListParams
	UserName   string `form:"userName"`
	NickName   string `form:"nickName"`
	UserGender string `form:"userGender"`
	UserPhone  string `form:"userPhone"`
	UserEmail  string `form:"userEmail"`
	Status     string `form:"status"`
}

type RoleListParams struct {
	SystemListParams
	RoleName string `form:"roleName"`
	RoleCode string `form:"roleCode"`
	Status   string `form:"status"`
}

func (p *SystemListParams) Normalize() {
	if p.Current < 1 {
		p.Current = 1
	}
	if p.Size < 1 || p.Size > 100 {
		p.Size = 20
	}
	if p.Sort == "" {
		p.Sort = "created_at"
	}
	p.Sort = pagination.SafeSort(p.Sort, "created_at")
	p.Order = pagination.SafeOrder(p.Order)
}

type PaginatedRecords struct {
	Records interface{} `json:"records"`
	Total   int64       `json:"total"`
	Current int         `json:"current"`
	Size    int         `json:"size"`
}

type UserManageResponse struct {
	ID         string   `json:"id"`
	UserName   string   `json:"userName"`
	UserGender string   `json:"userGender,omitempty"`
	NickName   string   `json:"nickName,omitempty"`
	UserPhone  string   `json:"userPhone,omitempty"`
	UserEmail  string   `json:"userEmail,omitempty"`
	UserRoles  []string `json:"userRoles"`
	Status     string   `json:"status,omitempty"`
	CreateBy   string   `json:"createBy,omitempty"`
	CreateTime string   `json:"createTime,omitempty"`
	UpdateBy   string   `json:"updateBy,omitempty"`
	UpdateTime string   `json:"updateTime,omitempty"`
	// last_login_at ĐƯỢC ghi ở AuthService.Login nhưng chưa API nào trả ra, nên
	// không cách nào biết tài khoản nào lâu rồi không ai dùng.
	LastLoginTime string `json:"lastLoginTime,omitempty"`
}

type RoleManageResponse struct {
	ID         string `json:"id"`
	RoleName   string `json:"roleName"`
	RoleCode   string `json:"roleCode"`
	RoleDesc   string `json:"roleDesc,omitempty"`
	Status     string `json:"status,omitempty"`
	CreateBy   string `json:"createBy,omitempty"`
	CreateTime string `json:"createTime,omitempty"`
	UpdateBy   string `json:"updateBy,omitempty"`
	UpdateTime string `json:"updateTime,omitempty"`
}

type AllRoleResponse struct {
	ID       string `json:"id"`
	RoleName string `json:"roleName"`
	RoleCode string `json:"roleCode"`
}

type CreateUserRequest struct {
	UserName   string   `json:"userName"`
	Password   string   `json:"password"`
	NickName   string   `json:"nickName"`
	UserGender string   `json:"userGender"`
	UserPhone  string   `json:"userPhone"`
	UserEmail  string   `json:"userEmail"`
	UserRoles  []string `json:"userRoles"`
}

type UpdateUserRequest struct {
	ID         string   `json:"id"`
	UserName   string   `json:"userName"`
	Password   string   `json:"password,omitempty"`
	NickName   string   `json:"nickName"`
	UserGender string   `json:"userGender"`
	UserPhone  string   `json:"userPhone"`
	UserEmail  string   `json:"userEmail"`
	UserRoles  []string `json:"userRoles"`
	Status     string   `json:"status"`
}

type CreateRoleRequest struct {
	RoleName string `json:"roleName"`
	RoleCode string `json:"roleCode"`
	RoleDesc string `json:"roleDesc"`
}

type UpdateRoleRequest struct {
	ID       string `json:"id"`
	RoleName string `json:"roleName"`
	RoleCode string `json:"roleCode"`
	RoleDesc string `json:"roleDesc"`
	Status   string `json:"status"`
}

type MenuManageResponse struct {
	ID         string                `json:"id"`
	ParentID   string                `json:"parentId"`
	MenuType   string                `json:"menuType"`
	MenuName   string                `json:"menuName"`
	RouteName  string                `json:"routeName"`
	RoutePath  string                `json:"routePath"`
	Component  string                `json:"component"`
	Icon       string                `json:"icon"`
	IconType   string                `json:"iconType"`
	Status     string                `json:"status"`
	HideInMenu bool                  `json:"hideInMenu"`
	Order      int                   `json:"order"`
	I18nKey    string                `json:"i18nKey"`
	Children   []*MenuManageResponse `json:"children,omitempty"`
	CreateBy   string                `json:"createBy"`
	CreateTime string                `json:"createTime"`
	UpdateBy   string                `json:"updateBy"`
	UpdateTime string                `json:"updateTime"`
}

type MenuTreeResponse struct {
	ID       string              `json:"id"`
	Label    string              `json:"label"`
	PID      string              `json:"pId"`
	Children []*MenuTreeResponse `json:"children,omitempty"`
}

func toUserManageResponse(user *model.User) *UserManageResponse {
	roles := make([]string, 0)
	for _, r := range user.Roles {
		roles = append(roles, r.Name)
	}

	status := "1"
	if !user.IsActive {
		status = "2"
	}

	resp := &UserManageResponse{
		ID:         user.ID,
		UserName:   user.Username,
		NickName:   user.FullName,
		UserPhone:  user.Phone,
		UserEmail:  user.Email,
		UserRoles:  roles,
		Status:     status,
		CreateTime: user.CreatedAt.Format(time.RFC3339),
		UpdateTime: user.UpdatedAt.Format(time.RFC3339),
	}
	if user.LastLoginAt != nil {
		resp.LastLoginTime = user.LastLoginAt.Format(time.RFC3339)
	}
	return resp
}

func toRoleManageResponse(role *model.Role) *RoleManageResponse {
	return &RoleManageResponse{
		ID:         role.ID,
		RoleName:   role.Name,
		RoleCode:   role.Name,
		RoleDesc:   role.Description,
		Status:     "1",
		CreateTime: role.CreatedAt.Format(time.RFC3339),
		UpdateTime: role.CreatedAt.Format(time.RFC3339),
	}
}

func (s *SystemManageService) ListUsers(params *UserListParams) (*PaginatedRecords, error) {
	params.Normalize()
	query := s.db.Model(&model.User{})

	if params.Search != "" {
		search := "%" + params.Search + "%"
		query = query.Where(
			// Họ tên là chỗ duy nhất có dấu; tên đăng nhập, số điện thoại và
			// email đều không dấu nên giữ ILIKE trần cho rẻ.
			"username ILIKE ? OR unaccent(full_name) ILIKE unaccent(?) OR phone ILIKE ? OR email ILIKE ?",
			search, search, search, search,
		)
	} else {
		if params.UserName != "" {
			query = query.Where("username ILIKE ?", "%"+params.UserName+"%")
		}
		if params.NickName != "" {
			query = query.Where("unaccent(full_name) ILIKE unaccent(?)", "%"+params.NickName+"%")
		}
		if params.UserPhone != "" {
			query = query.Where("phone ILIKE ?", "%"+params.UserPhone+"%")
		}
		if params.UserEmail != "" {
			query = query.Where("email ILIKE ?", "%"+params.UserEmail+"%")
		}
	}
	if params.Status == "1" {
		query = query.Where("is_active = ?", true)
	} else if params.Status == "2" {
		query = query.Where("is_active = ?", false)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var users []model.User
	offset := (params.Current - 1) * params.Size
	if err := query.
		Preload("Roles").
		Offset(offset).
		Limit(params.Size).
		Order(params.Sort + " " + params.Order).
		Find(&users).Error; err != nil {
		return nil, err
	}

	records := make([]*UserManageResponse, len(users))
	for i := range users {
		records[i] = toUserManageResponse(&users[i])
	}

	return &PaginatedRecords{
		Records: records,
		Total:   total,
		Current: params.Current,
		Size:    params.Size,
	}, nil
}

func (s *SystemManageService) GetUserByID(id string) (*UserManageResponse, error) {
	var user model.User
	if err := s.db.Where("id = ?", id).Preload("Roles").First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return toUserManageResponse(&user), nil
}

func (s *SystemManageService) CreateUser(req *CreateUserRequest) (*UserManageResponse, error) {
	if req.UserName == "" {
		return nil, errors.New("username is required")
	}
	if req.Password == "" {
		return nil, errors.New("phải nhập mật khẩu")
	}

	var existing model.User
	if err := s.db.Where("username = ?", req.UserName).First(&existing).Error; err == nil {
		return nil, errors.New("username already exists")
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := model.User{
		Username:     req.UserName,
		PasswordHash: hash,
		FullName:     req.NickName,
		Email:        req.UserEmail,
		Phone:        req.UserPhone,
		IsActive:     true,
	}

	tx := s.db.Begin()
	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if len(req.UserRoles) > 0 {
		var roles []model.Role
		if err := tx.Where("name IN ?", req.UserRoles).Find(&roles).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Model(&user).Association("Roles").Replace(roles); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "user",
		EntityID:   user.ID,
		Metadata:   map[string]interface{}{"username": req.UserName},
	})

	return s.GetUserByID(user.ID)
}

func (s *SystemManageService) UpdateUser(req *UpdateUserRequest) (*UserManageResponse, error) {
	var user model.User
	if err := s.db.Where("id = ?", req.ID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.UserName != "" {
		updates["username"] = req.UserName
	}
	if req.NickName != "" {
		updates["full_name"] = req.NickName
	}
	if req.UserPhone != "" {
		updates["phone"] = req.UserPhone
	}
	if req.UserEmail != "" {
		updates["email"] = req.UserEmail
	}
	if req.Password != "" {
		hash, err := utils.HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		updates["password_hash"] = hash
	}
	if req.Status == "1" {
		updates["is_active"] = true
	} else if req.Status == "2" {
		updates["is_active"] = false
	}

	tx := s.db.Begin()

	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		if err := tx.Model(&user).Updates(updates).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if req.UserRoles != nil {
		var roles []model.Role
		if err := tx.Where("name IN ?", req.UserRoles).Find(&roles).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Model(&user).Association("Roles").Replace(roles); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "user",
		EntityID:   req.ID,
		Metadata:   map[string]interface{}{"username": req.UserName, "changes": updates},
	})

	return s.GetUserByID(user.ID)
}

// DeleteUser xoá một tài khoản nhân viên.
//
// Nhân viên đã làm việc là nhân viên có chứng từ mang tên mình: ca làm, phiếu
// bàn giao tiền, đơn đã lập, giao dịch số dư đã lập, phiếu kho, phiên kiểm kê.
// Xoá tài khoản mà bỏ lại chúng thì đối soát ca trỏ tới một người không còn.
//
// audit_logs KHÔNG chặn, dù nó cũng mang user_id. Nhật ký hệ thống ghi cả lần
// đăng nhập đầu tiên, nên chặn theo nó nghĩa là mọi tài khoản từng đăng nhập
// một lần đều vĩnh viễn không xoá được — kể cả tài khoản vừa tạo nhầm và chưa
// làm gì. Chính hành động xoá này cũng ghi vào đó.
func (s *SystemManageService) DeleteUser(id string) error {
	var user model.User
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}
	if err := kiemTraPhuThuoc(s.db, id, []phuThuoc{
		{Bang: &model.Shift{}, Cot: "user_id", Nhan: "ca làm việc"},
		{Bang: &model.Order{}, Cot: "created_by", Nhan: "đơn hàng đã lập"},
		{Bang: &model.MemberTransaction{}, Cot: "created_by", Nhan: "giao dịch số dư đã lập"},
		{Bang: &model.CashHandover{}, Cot: "created_by", Nhan: "phiếu bàn giao tiền"},
		{Bang: &model.StockTransaction{}, Cot: "created_by", Nhan: "phiếu kho"},
		{Bang: &model.InventoryCountSession{}, Cot: "opened_by", Nhan: "phiên kiểm kê"},
	}, "hãy khoá tài khoản (đổi trạng thái sang Ngừng) thay vì xoá"); err != nil {
		return err
	}
	if err := s.db.Delete(&user).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "user",
		EntityID:   id,
		Metadata:   map[string]interface{}{"username": user.Username},
	})

	return nil
}

// BatchDeleteUsers xoá nhiều tài khoản, lặp qua DeleteUser đúng như
// BatchDeleteRoles vẫn làm.
//
// Bản cũ chạy một câu "WHERE id IN (...)" duy nhất, nên nó bỏ qua toàn bộ phần
// kiểm ràng buộc VÀ không ghi audit cho từng tài khoản — chỉ một dòng gộp không
// nói được ai đã bị xoá. Dừng ở tài khoản đầu tiên bị chặn: những cái trước đó
// đã xoá thật và audit đã ghi, nên không có trạng thái nửa vời nào bị giấu.
func (s *SystemManageService) BatchDeleteUsers(ids []string) error {
	if len(ids) == 0 {
		return errors.New("chưa chọn bản ghi nào")
	}
	for _, id := range ids {
		if err := s.DeleteUser(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *SystemManageService) ListRoles(params *RoleListParams) (*PaginatedRecords, error) {
	params.Normalize()
	query := s.db.Model(&model.Role{})

	if params.Search != "" {
		search := "%" + params.Search + "%"
		query = query.Where("unaccent(name) ILIKE unaccent(?) OR unaccent(description) ILIKE unaccent(?)", search, search)
	} else {
		if params.RoleName != "" {
			query = query.Where("unaccent(name) ILIKE unaccent(?)", "%"+params.RoleName+"%")
		}
		if params.RoleCode != "" {
			query = query.Where("name ILIKE ?", "%"+params.RoleCode+"%")
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var roles []model.Role
	offset := (params.Current - 1) * params.Size
	if err := query.
		Offset(offset).
		Limit(params.Size).
		Order(params.Sort + " " + params.Order).
		Find(&roles).Error; err != nil {
		return nil, err
	}

	records := make([]*RoleManageResponse, len(roles))
	for i := range roles {
		records[i] = toRoleManageResponse(&roles[i])
	}

	return &PaginatedRecords{
		Records: records,
		Total:   total,
		Current: params.Current,
		Size:    params.Size,
	}, nil
}

func (s *SystemManageService) GetAllRoles() ([]*AllRoleResponse, error) {
	var roles []model.Role
	if err := s.db.Find(&roles).Error; err != nil {
		return nil, err
	}
	result := make([]*AllRoleResponse, len(roles))
	for i := range roles {
		result[i] = &AllRoleResponse{
			ID:       roles[i].ID,
			RoleName: roles[i].Name,
			RoleCode: roles[i].Name,
		}
	}
	return result, nil
}

func (s *SystemManageService) CreateRole(req *CreateRoleRequest) (*RoleManageResponse, error) {
	if req.RoleName == "" {
		return nil, errors.New("phải nhập tên vai trò")
	}

	role := model.Role{
		Name:        req.RoleName,
		Description: req.RoleDesc,
	}

	if err := s.db.Create(&role).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "role",
		EntityID:   role.ID,
		Metadata:   map[string]interface{}{"name": req.RoleName},
	})

	return toRoleManageResponse(&role), nil
}

func (s *SystemManageService) UpdateRole(req *UpdateRoleRequest) (*RoleManageResponse, error) {
	var role model.Role
	if err := s.db.Where("id = ?", req.ID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy vai trò")
		}
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.RoleName != "" {
		updates["name"] = req.RoleName
	}
	if req.RoleDesc != "" {
		updates["description"] = req.RoleDesc
	}

	if len(updates) > 0 {
		if err := s.db.Model(&role).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	s.db.Where("id = ?", req.ID).First(&role)

	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "role",
		EntityID:   req.ID,
		Metadata:   map[string]interface{}{"name": req.RoleName, "description": req.RoleDesc},
	})

	return toRoleManageResponse(&role), nil
}

func (s *SystemManageService) DeleteRole(id string) error {
	var role model.Role
	if err := s.db.Where("id = ?", id).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy vai trò")
		}
		return err
	}

	// Tài khoản bị xoá là xoá MỀM, nên dòng trong user_roles vẫn còn. Đếm thẳng
	// user_roles sẽ báo "vai trò còn người dùng" vì một tài khoản đã xoá — vai
	// trò đó không bao giờ xoá được nữa mà người vận hành không hiểu vì sao.
	var userCount int64
	s.db.Model(&model.UserRole{}).
		Joins("JOIN users ON users.id = user_roles.user_id AND users.deleted_at IS NULL").
		Where("user_roles.role_id = ?", id).
		Count(&userCount)
	if userCount > 0 {
		return chanVi("không xoá được vai trò đang gán cho %d tài khoản — hãy gỡ vai trò khỏi họ trước", userCount)
	}

	// Hai bảng nối phải dọn TRƯỚC, và đây là chỗ duy nhất trong hệ thống mà
	// khoá ngoại thật sự tồn tại: user_roles và role_permissions do các trường
	// many2many trong model/user.go sinh ra, cả hai đều NO ACTION (xem
	// TestModel_ChiCoHaiKhoaNgoaiThat). Role lại xoá CỨNG, nên xoá thẳng một vai
	// trò còn quyền sẽ bị PostgreSQL chặn bằng lỗi 23503 thô, hiện lên trang
	// quản trị dưới dạng một câu tiếng Anh không ai hiểu.
	//
	// user_roles vẫn có thể còn dòng dù phần đếm ở trên đã qua: phần đếm chỉ
	// tính tài khoản CHƯA xoá, còn tài khoản xoá mềm vẫn giữ dòng của nó.
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", id).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", id).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		return tx.Delete(&role).Error
	}); err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "role",
		EntityID:   id,
		Metadata:   map[string]interface{}{"name": role.Name},
	})

	return nil
}

func (s *SystemManageService) GetMenuList(params *SystemListParams) (*PaginatedRecords, error) {
	params.Normalize()
	var allMenus []*MenuManageResponse

	menuDefs := []struct {
		Name     string
		Path     string
		Icon     string
		Order    int
		Children []struct {
			Name    string
			Path    string
			Icon    string
			Order   int
			I18nKey string
		}
	}{
		{
			Name: "vnet", Path: "/vnet", Icon: "ant-design:appstore-outlined", Order: 1,
			Children: []struct {
				Name    string
				Path    string
				Icon    string
				Order   int
				I18nKey string
			}{
				{Name: "vnet_dashboard", Path: "/vnet/dashboard", Icon: "ic:round-dashboard", Order: 1, I18nKey: "route.vnet_dashboard"},
				{Name: "vnet_members", Path: "/vnet/members", Icon: "ic:round-people", Order: 2, I18nKey: "route.vnet_members"},
				{Name: "vnet_machines", Path: "/vnet/machines", Icon: "ic:round-computer", Order: 4, I18nKey: "route.vnet_machines"},
				{Name: "vnet_sessions", Path: "/vnet/sessions", Icon: "ic:round-play-circle", Order: 5, I18nKey: "route.vnet_sessions"},
				{Name: "vnet_orders", Path: "/vnet/orders", Icon: "ic:round-receipt", Order: 6, I18nKey: "route.vnet_orders"},
				{Name: "vnet_products", Path: "/vnet/products", Icon: "ic:round-inventory", Order: 7, I18nKey: "route.vnet_products"},
				{Name: "vnet_categories", Path: "/vnet/categories", Icon: "ic:round-category", Order: 8, I18nKey: "route.vnet_categories"},
				{Name: "vnet_suppliers", Path: "/vnet/suppliers", Icon: "ic:round-business", Order: 11, I18nKey: "route.vnet_suppliers"},
				{Name: "vnet_stock-transactions", Path: "/vnet/stock-transactions", Icon: "ic:round-swap-vert", Order: 12, I18nKey: "route.vnet_stock-transactions"},
				{Name: "vnet_combos", Path: "/vnet/combos", Icon: "ic:round-discount", Order: 13, I18nKey: "route.vnet_combos"},
				{Name: "vnet_shifts", Path: "/vnet/shifts", Icon: "ic:round-schedule", Order: 14, I18nKey: "route.vnet_shifts"},
				{Name: "vnet_bookings", Path: "/vnet/bookings", Icon: "ic:round-calendar-today", Order: 15, I18nKey: "route.vnet_bookings"},
				{Name: "vnet_promotions", Path: "/vnet/promotions", Icon: "ic:round-campaign", Order: 16, I18nKey: "route.vnet_promotions"},
				{Name: "vnet_reports", Path: "/vnet/reports", Icon: "ic:round-bar-chart", Order: 17, I18nKey: "route.vnet_reports"},
				{Name: "vnet_transactions", Path: "/vnet/transactions", Icon: "ic:round-receipt-long", Order: 19, I18nKey: "route.vnet_transactions"},
				{Name: "vnet_settings", Path: "/vnet/settings", Icon: "ic:round-settings", Order: 20, I18nKey: "route.vnet_settings"},
				{Name: "vnet_audit", Path: "/vnet/audit", Icon: "ic:round-history", Order: 21, I18nKey: "route.vnet_audit"},
				{Name: "vnet_backups", Path: "/vnet/backups", Icon: "ic:round-backup", Order: 23, I18nKey: "route.vnet_backups"},
				{Name: "vnet_machine-groups", Path: "/vnet/machine-groups", Icon: "carbon:data-center", Order: 24, I18nKey: "route.vnet_machine-groups"},
				{Name: "vnet_member-groups", Path: "/vnet/member-groups", Icon: "carbon:user-multiple", Order: 25, I18nKey: "route.vnet_member-groups"},
				{Name: "vnet_notifications", Path: "/vnet/notifications", Icon: "ic:round-notifications", Order: 26, I18nKey: "route.vnet_notifications"},
				{Name: "vnet_printers", Path: "/vnet/printers", Icon: "ic:round-print", Order: 27, I18nKey: "route.vnet_printers"},
				{Name: "vnet_cards", Path: "/vnet/cards", Icon: "ic:round-card-giftcard", Order: 28, I18nKey: "route.vnet_cards"},
				{Name: "vnet_inventory-counts", Path: "/vnet/inventory-counts", Icon: "ic:round-fact-check", Order: 29, I18nKey: "route.vnet_inventory-counts"},
				{Name: "vnet_attendance", Path: "/vnet/attendance", Icon: "ic:round-event-available", Order: 30, I18nKey: "route.vnet_attendance"},
				{Name: "vnet_website-blocking", Path: "/vnet/website-blocking", Icon: "ic:round-block", Order: 31, I18nKey: "route.vnet_website-blocking"},
				{Name: "vnet_app-updates", Path: "/vnet/app-updates", Icon: "ic:round-system-update", Order: 32, I18nKey: "route.vnet_app-updates"},
				{Name: "vnet_curfew", Path: "/vnet/curfew", Icon: "ic:round-nightlight", Order: 33, I18nKey: "route.vnet_curfew"},
				{Name: "vnet_feedback", Path: "/vnet/feedback", Icon: "ic:round-star-rate", Order: 35, I18nKey: "route.vnet_feedback"},
				{Name: "vnet_machine-assets", Path: "/vnet/machine-assets", Icon: "ic:round-inventory-2", Order: 34, I18nKey: "route.vnet_machine-assets"},
			},
		},
		{
			Name: "system", Path: "/system", Icon: "carbon:cloud-service-management", Order: 9,
			Children: []struct {
				Name    string
				Path    string
				Icon    string
				Order   int
				I18nKey string
			}{
				{Name: "system_user", Path: "/system/user", Icon: "carbon:user-admin", Order: 1, I18nKey: "route.system_user"},
				{Name: "system_role", Path: "/system/role", Icon: "carbon:user-role", Order: 2, I18nKey: "route.system_role"},
				{Name: "system_menu", Path: "/system/menu", Icon: "carbon:tree-view", Order: 3, I18nKey: "route.system_menu"},
			},
		},
	}

	for _, dir := range menuDefs {
		parent := &MenuManageResponse{
			ID:        dir.Name,
			ParentID:  "0",
			MenuType:  "1",
			MenuName:  dir.Name,
			RouteName: dir.Name,
			RoutePath: dir.Path,
			Icon:      dir.Icon,
			IconType:  "1",
			Status:    "1",
			Order:     dir.Order,
		}

		for _, child := range dir.Children {
			childMenu := &MenuManageResponse{
				ID:        child.Name,
				ParentID:  dir.Name,
				MenuType:  "2",
				MenuName:  child.Name,
				RouteName: child.Name,
				RoutePath: child.Path,
				Icon:      child.Icon,
				IconType:  "1",
				Status:    "1",
				Order:     child.Order,
				I18nKey:   child.I18nKey,
			}
			// Danh sách này PHẲNG: mỗi mục menu đúng một dòng, quan hệ cha–con
			// nằm ở cột parentId. Gắn thêm childMenu vào parent.Children khiến
			// 33 mục con xuất hiện HAI LẦN trên cùng một trang (một lần là dòng
			// gốc, một lần là dòng con do el-table tự trải) — Vue kêu "Duplicate
			// keys" và người xem đếm menu ra gấp đôi. Cây menu riêng đã có
			// endpoint GetMenuTree.
			allMenus = append(allMenus, childMenu)
		}

		allMenus = append(allMenus, parent)
	}

	offset := (params.Current - 1) * params.Size
	end := offset + params.Size
	if offset > len(allMenus) {
		offset = len(allMenus)
	}
	if end > len(allMenus) {
		end = len(allMenus)
	}
	pageItems := allMenus[offset:end]

	return &PaginatedRecords{
		Records: pageItems,
		Total:   int64(len(allMenus)),
		Current: params.Current,
		Size:    params.Size,
	}, nil
}

func (s *SystemManageService) GetAllPages() ([]string, error) {
	return []string{
		"vnet_dashboard",
		"vnet_members",
		"vnet_machines",
		"vnet_sessions",
		"vnet_orders",
		"vnet_products",
		"vnet_categories",
		"vnet_suppliers",
		"vnet_stock-transactions",
		"vnet_combos",
		"vnet_shifts",
		"vnet_bookings",
		"vnet_promotions",
		"vnet_reports",
		"vnet_transactions",
		"vnet_settings",
		"vnet_audit",
		"vnet_backups",
		"vnet_printers",
		"vnet_cards",
		"vnet_inventory-counts",
		"vnet_attendance",
		"vnet_website-blocking",
		"vnet_app-updates",
		"vnet_curfew",
		"vnet_feedback",
		"vnet_machine-assets",
		"vnet_machine-groups",
		"vnet_member-groups",
		"vnet_notifications",
		"system_user",
		"system_role",
		"system_menu",
		"system_user-detail",
		"login",
		"user-center",
	}, nil
}

func (s *SystemManageService) GetMenuTree() ([]*MenuTreeResponse, error) {
	return []*MenuTreeResponse{
		{
			ID: "0", Label: "root", PID: "-1",
			Children: []*MenuTreeResponse{
				{
					ID: "vnet", Label: "VNET", PID: "0",
					Children: []*MenuTreeResponse{
						{ID: "vnet_dashboard", Label: "Dashboard", PID: "vnet"},
						{ID: "vnet_members", Label: "Members", PID: "vnet"},
						{ID: "vnet_machines", Label: "Machines", PID: "vnet"},
						{ID: "vnet_sessions", Label: "Sessions", PID: "vnet"},
						{ID: "vnet_orders", Label: "Orders", PID: "vnet"},
						{ID: "vnet_products", Label: "Products", PID: "vnet"},
						{ID: "vnet_categories", Label: "Categories", PID: "vnet"},
						{ID: "vnet_suppliers", Label: "Suppliers", PID: "vnet"},
						{ID: "vnet_stock-transactions", Label: "Stock", PID: "vnet"},
						{ID: "vnet_combos", Label: "Combos", PID: "vnet"},
						{ID: "vnet_shifts", Label: "Shifts", PID: "vnet"},
						{ID: "vnet_bookings", Label: "Bookings", PID: "vnet"},
						{ID: "vnet_promotions", Label: "Promotions", PID: "vnet"},
						{ID: "vnet_reports", Label: "Reports", PID: "vnet"},
						{ID: "vnet_transactions", Label: "Transactions", PID: "vnet"},
						{ID: "vnet_settings", Label: "Settings", PID: "vnet"},
						{ID: "vnet_audit", Label: "Audit", PID: "vnet"},
						{ID: "vnet_backups", Label: "Backups", PID: "vnet"},
						{ID: "vnet_machine-groups", Label: "Machine Groups", PID: "vnet"},
						{ID: "vnet_member-groups", Label: "Member Groups", PID: "vnet"},
						{ID: "vnet_notifications", Label: "Notifications", PID: "vnet"},
					},
				},
				{
					ID: "system", Label: "System", PID: "0",
					Children: []*MenuTreeResponse{
						{ID: "system_user", Label: "User Management", PID: "system"},
						{ID: "system_role", Label: "Role Management", PID: "system"},
						{ID: "system_menu", Label: "Menu Management", PID: "system"},
					},
				},
			},
		},
	}, nil
}

// --- phân quyền cho vai trò ---------------------------------------------------
//
// Trước đây bảng role_permissions chỉ được ghi đúng một lần bởi cmd/seed. Cài
// xong rồi thì không đổi được quyền của vai trò nào nữa: CreateRole/UpdateRole/
// DeleteRole đã viết đủ nhưng không có handler, và trang Vai trò của Soybean là
// bản mẫu — bấm Lưu chỉ hiện thông báo thành công rồi không gửi gì.

type PermissionResponse struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Module string `json:"module"`
}

func (s *SystemManageService) GetAllPermissions() ([]PermissionResponse, error) {
	var perms []model.Permission
	if err := s.db.Order("module, code").Find(&perms).Error; err != nil {
		return nil, err
	}
	out := make([]PermissionResponse, len(perms))
	for i, p := range perms {
		out[i] = PermissionResponse{ID: p.ID, Code: p.Code, Name: p.Name, Module: p.Module}
	}
	return out, nil
}

// GetRolePermissions trả về ID các quyền vai trò đang có, để màn hình tick sẵn.
func (s *SystemManageService) GetRolePermissions(roleID string) ([]string, error) {
	var role model.Role
	if err := s.db.Preload("Permissions").Where("id = ?", roleID).First(&role).Error; err != nil {
		return nil, errors.New("không tìm thấy vai trò")
	}
	ids := make([]string, len(role.Permissions))
	for i, p := range role.Permissions {
		ids[i] = p.ID
	}
	return ids, nil
}

func (s *SystemManageService) UpdateRolePermissions(roleID string, permissionIDs []string) error {
	var role model.Role
	if err := s.db.Where("id = ?", roleID).First(&role).Error; err != nil {
		return errors.New("không tìm thấy vai trò")
	}

	var perms []model.Permission
	if len(permissionIDs) > 0 {
		if err := s.db.Where("id IN ?", permissionIDs).Find(&perms).Error; err != nil {
			return err
		}
		if len(perms) != len(permissionIDs) {
			return errors.New("có mã quyền không tồn tại")
		}
	}

	// Replace với danh sách rỗng là cách gỡ sạch quyền — đúng ý khi người dùng
	// bỏ tick hết, nên KHÔNG chặn trường hợp rỗng.
	if err := s.db.Model(&role).Association("Permissions").Replace(&perms); err != nil {
		return err
	}

	// Quyền nằm trong JWT, nên đổi quyền chỉ có hiệu lực ở lần đăng nhập (hoặc
	// làm mới token) sau. Ghi vào nhật ký để lúc truy vết còn biết mốc thời gian.
	codes := make([]string, len(perms))
	for i, p := range perms {
		codes[i] = p.Code
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "update_permissions",
		EntityType: "role",
		EntityID:   roleID,
		Metadata: map[string]interface{}{
			"role":        role.Name,
			"permissions": codes,
			"note":        "có hiệu lực ở lần đăng nhập kế tiếp vì quyền nằm trong token",
		},
	})
	return nil
}

func (s *SystemManageService) BatchDeleteRoles(ids []string) error {
	for _, id := range ids {
		if err := s.DeleteRole(id); err != nil {
			return err
		}
	}
	return nil
}
