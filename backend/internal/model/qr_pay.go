package model

import "time"

// Nạp tiền bằng QR, tự xác nhận nhờ điện thoại ở quầy đọc thông báo biến động
// số dư của app ngân hàng (app vnet-bank-notification). Kế hoạch:
// research/nap-qr-dien-thoai-android-plan.md.

const (
	QRRequestPending   = "pending"
	QRRequestPaid      = "paid"
	QRRequestExpired   = "expired"
	QRRequestCancelled = "cancelled"
)

// QRRequest là một mã QR đã đưa cho khách: gắn với đúng một đơn nạp chờ duyệt.
// Code nằm trong nội dung chuyển khoản; thông báo ngân hàng mang Code này về là
// biết tiền của đơn nào.
type QRRequest struct {
	ID        string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	OrderID   string     `gorm:"type:uuid;not null;index" json:"order_id"`
	MemberID  *string    `gorm:"type:uuid;index" json:"member_id"`
	MachineID *string    `gorm:"type:uuid" json:"machine_id"`
	Code      string     `gorm:"type:varchar(16);not null;uniqueIndex" json:"code"`
	Amount    int64      `gorm:"not null" json:"amount"`
	Bank      string     `gorm:"type:varchar(10);not null" json:"bank"`
	AccountNo string     `gorm:"type:varchar(30);not null" json:"account_no"`
	Status    string     `gorm:"type:varchar(20);not null;default:pending;index" json:"status"`
	ExpiresAt time.Time  `gorm:"type:timestamptz;not null;index" json:"expires_at"`
	PaidAt    *time.Time `gorm:"type:timestamptz" json:"paid_at"`
	// Thông báo ngân hàng đã khớp với mã này.
	BankNotificationID *string   `gorm:"type:uuid" json:"bank_notification_id"`
	CreatedBy          *string   `gorm:"type:uuid" json:"created_by"`
	CreatedAt          time.Time `gorm:"default:now();index" json:"created_at,omitempty"`
	UpdatedAt          time.Time `gorm:"default:now()" json:"updated_at,omitempty"`
}

const (
	BankNotifMatched   = "matched"   // khớp mã + số tiền, đã cộng tiền
	BankNotifSuggested = "suggested" // khớp nhưng tự duyệt đang tắt: chờ nhân viên bấm
	BankNotifMismatch  = "amount_mismatch"
	BankNotifUnmatched = "unmatched" // tiền vào, không thấy mã của yêu cầu nào
	BankNotifAssigned  = "assigned"  // nhân viên đã cộng tay cho một hội viên
	BankNotifIgnored   = "ignored"   // tiền ra, hoặc nhân viên bỏ qua
	BankNotifUnparsed  = "unparsed"  // không đọc được số tiền
)

// BankNotification là nguyên văn một thông báo điện thoại ở quầy gửi về, kèm
// phần máy chủ đọc được và đã làm gì với nó. Giữ nguyên văn để đối chiếu sao kê
// và để sửa bộ đọc khi app ngân hàng đổi định dạng.
type BankNotification struct {
	ID         string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ReceivedAt time.Time `gorm:"type:timestamptz;not null;index" json:"received_at"`
	// Giờ app ngân hàng đăng thông báo, theo đồng hồ điện thoại.
	PostedAt *time.Time `gorm:"type:timestamptz" json:"posted_at"`
	Package  string     `gorm:"type:varchar(100);not null" json:"package"`
	Title    string     `gorm:"type:text" json:"title"`
	Text     string     `gorm:"type:text" json:"text"`
	// Băm gói + nguyên văn: app ngân hàng hay đăng lại cùng một thông báo.
	Hash          string    `gorm:"type:varchar(64);not null;uniqueIndex" json:"-"`
	Bank          string    `gorm:"type:varchar(10)" json:"bank"`
	Amount        int64     `gorm:"default:0" json:"amount"`
	Incoming      bool      `gorm:"default:false" json:"incoming"`
	Code          string    `gorm:"type:varchar(16);index" json:"code"`
	BankRef       string    `gorm:"type:varchar(60)" json:"bank_ref"`
	SenderName    string    `gorm:"type:varchar(120)" json:"sender_name"`
	SenderAccount string    `gorm:"type:varchar(40)" json:"sender_account"`
	Content       string    `gorm:"type:text" json:"content"`
	Status        string    `gorm:"type:varchar(20);not null;index" json:"status"`
	QRRequestID   *string   `gorm:"type:uuid" json:"qr_request_id"`
	OrderID       *string   `gorm:"type:uuid" json:"order_id"`
	MemberID      *string   `gorm:"type:uuid;index" json:"member_id"`
	HandledBy     *string   `gorm:"type:uuid" json:"handled_by"`
	Note          string    `gorm:"type:text" json:"note"`
	CreatedAt     time.Time `gorm:"default:now()" json:"created_at,omitempty"`
	UpdatedAt     time.Time `gorm:"default:now()" json:"updated_at,omitempty"`
}
