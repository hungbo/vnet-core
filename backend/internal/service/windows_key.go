package service

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
)

// Windows Keys (roadmap M4): quán ghi key bản quyền cho từng máy, nhập hàng loạt, và đối chiếu
// với bản quyền mà chính máy trạm đọc từ Windows. KHÔNG kích hoạt: xem model.WindowsKey.
//
// Key là bí mật của quán: danh sách chỉ trả bản che (5 ký tự cuối), key đầy đủ chỉ qua
// RevealKey có quyền riêng và để lại dòng nhật ký — ghi RẰNG ai đã xem key của máy nào, không
// ghi chính key.

var (
	ErrWindowsKeyInvalid     = errors.New("Windows key không đúng định dạng XXXXX-XXXXX-XXXXX-XXXXX-XXXXX")
	ErrWindowsKeyNoMachine   = errors.New("không tìm thấy máy")
	ErrWindowsKeyNotFound    = errors.New("máy này chưa có Windows key")
	ErrWindowsKeyEmptyImport = errors.New("chưa có dòng nào để nhập")
	ErrWindowsKeyTooMany     = errors.New("tối đa 2000 dòng mỗi lần nhập")
)

const windowsKeyImportMax = 2000

var (
	windowsKeyRe   = regexp.MustCompile(`^[A-Z0-9]{5}(-[A-Z0-9]{5}){4}$`)
	windowsKeyBare = regexp.MustCompile(`^[A-Z0-9]{25}$`)
	importSplitRe  = regexp.MustCompile(`[\s,;]+`)
)

// NormalizeWindowsKey đưa key về dạng chuẩn XXXXX-XXXXX-XXXXX-XXXXX-XXXXX, chữ hoa. Nhận cả
// dạng dán liền 25 ký tự (không dấu gạch) — nhiều nơi cấp key gửi như vậy.
func NormalizeWindowsKey(s string) (string, bool) {
	k := strings.ToUpper(strings.TrimSpace(s))
	if windowsKeyBare.MatchString(k) {
		k = k[0:5] + "-" + k[5:10] + "-" + k[10:15] + "-" + k[15:20] + "-" + k[20:25]
	}
	return k, windowsKeyRe.MatchString(k)
}

// MaskWindowsKey giữ 5 ký tự cuối: đủ để nhận ra key và so với máy, không đủ để dùng.
func MaskWindowsKey(k string) string {
	if len(k) < 5 {
		return ""
	}
	return "XXXXX-XXXXX-XXXXX-XXXXX-" + k[len(k)-5:]
}

type WindowsKeyService struct {
	db    *gorm.DB
	audit *AuditService
	hub   *hub.Hub
}

func NewWindowsKeyService(db *gorm.DB, audit *AuditService) *WindowsKeyService {
	return &WindowsKeyService{db: db, audit: audit}
}

func (s *WindowsKeyService) WithHub(h *hub.Hub) *WindowsKeyService {
	s.hub = h
	return s
}

// WindowsKeyRow là một dòng của bảng: MỘT MÁY, có key hay chưa.
type WindowsKeyRow struct {
	MachineID     string `json:"machine_id"`
	MachineCode   string `json:"machine_code"`
	MachineStatus string `json:"machine_status"`
	HasKey        bool   `json:"has_key"`
	KeyMasked     string `json:"key_masked"`
	Edition       string `json:"edition"`
	Note          string `json:"note"`
	// Bản quyền máy đọc từ Windows (rỗng = máy chưa báo).
	WinLicense string `json:"win_license"`
	WinKeyTail string `json:"win_key_tail"`
	// match: key quán ghi khớp key đang cài | mismatch: lệch | unknown: máy chưa báo đuôi key |
	// "" : máy chưa có key để so.
	Match     string     `json:"match"`
	UpdatedAt *time.Time `json:"updated_at"`
}

// windowsKeyRecord là kết quả join thô; Key đầy đủ không bao giờ ra khỏi service.
type windowsKeyRecord struct {
	MachineID     string
	MachineCode   string
	MachineStatus string
	WinLicense    string
	WinKeyTail    string
	KeyID         *string
	FullKey       string
	Edition       string
	Note          string
	UpdatedAt     *time.Time
}

func (r windowsKeyRecord) row() WindowsKeyRow {
	out := WindowsKeyRow{
		MachineID: r.MachineID, MachineCode: r.MachineCode, MachineStatus: r.MachineStatus,
		WinLicense: r.WinLicense, WinKeyTail: r.WinKeyTail,
		HasKey: r.KeyID != nil, Edition: r.Edition, Note: r.Note, UpdatedAt: r.UpdatedAt,
	}
	if !out.HasKey {
		return out
	}
	out.KeyMasked = MaskWindowsKey(r.FullKey)
	switch {
	case r.WinKeyTail == "":
		out.Match = "unknown"
	case strings.HasSuffix(r.FullKey, r.WinKeyTail):
		out.Match = "match"
	default:
		out.Match = "mismatch"
	}
	return out
}

// WindowsKeyFilter: Filter ∈ "", no_key, mismatch, unlicensed.
type WindowsKeyFilter struct {
	Search string
	Filter string
}

const windowsKeyJoin = `FROM machines AS m LEFT JOIN windows_keys AS k ON k.machine_id = m.id`

func (f WindowsKeyFilter) apply(q *gorm.DB) *gorm.DB {
	q = q.Where("m.deleted_at IS NULL")
	if f.Search != "" {
		q = q.Where("unaccent(m.machine_code) ILIKE unaccent(?)", "%"+f.Search+"%")
	}
	switch f.Filter {
	case "no_key":
		q = q.Where("k.id IS NULL")
	case "mismatch":
		q = q.Where("k.id IS NOT NULL AND m.win_key_tail <> '' AND right(k.key, 5) <> m.win_key_tail")
	case "unlicensed":
		q = q.Where("m.win_license <> '' AND m.win_license <> 'licensed'")
	}
	return q
}

// List trả MỌI máy (có hoặc chưa có key), có phân trang.
func (s *WindowsKeyService) List(params pagination.Params, f WindowsKeyFilter) (*pagination.Result, error) {
	base := s.db.Table("machines AS m").Joins("LEFT JOIN windows_keys AS k ON k.machine_id = m.id")
	base = f.apply(base)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	var recs []windowsKeyRecord
	err := base.Select(`m.id AS machine_id, m.machine_code, m.status AS machine_status, m.win_license, m.win_key_tail,
			k.id AS key_id, k.key AS full_key, k.edition, k.note, k.updated_at`).
		Order("m.machine_code asc").
		Limit(params.PageSize).Offset((params.Page - 1) * params.PageSize).
		Scan(&recs).Error
	if err != nil {
		return nil, err
	}
	rows := make([]WindowsKeyRow, len(recs))
	for i, r := range recs {
		rows[i] = r.row()
	}
	return pagination.NewResult(rows, total, &params), nil
}

// WindowsKeySummary là các con số đầu trang.
type WindowsKeySummary struct {
	Total      int64 `json:"total"`
	WithKey    int64 `json:"with_key"`
	NoKey      int64 `json:"no_key"`
	Mismatch   int64 `json:"mismatch"`
	Unlicensed int64 `json:"unlicensed"`
}

func (s *WindowsKeyService) Summary() (WindowsKeySummary, error) {
	var out WindowsKeySummary
	err := s.db.Raw(`SELECT count(*) AS total,
			count(k.id) AS with_key,
			count(*) FILTER (WHERE k.id IS NULL) AS no_key,
			count(*) FILTER (WHERE k.id IS NOT NULL AND m.win_key_tail <> '' AND right(k.key, 5) <> m.win_key_tail) AS mismatch,
			count(*) FILTER (WHERE m.win_license <> '' AND m.win_license <> 'licensed') AS unlicensed
		` + windowsKeyJoin + ` WHERE m.deleted_at IS NULL`).Scan(&out).Error
	return out, err
}

func (s *WindowsKeyService) machine(id string) (*model.Machine, error) {
	var m model.Machine
	if err := s.db.Where("id = ?", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWindowsKeyNoMachine
		}
		return nil, err
	}
	return &m, nil
}

func (s *WindowsKeyService) log(actorID, action, machineID string, meta map[string]interface{}) {
	req := &LogAuditRequest{Action: action, EntityType: "machine", EntityID: machineID, Metadata: meta}
	if actorID != "" {
		req.UserID = &actorID
	}
	_ = s.audit.Log(req)
}

func (s *WindowsKeyService) changed(machineID string) {
	s.hub.Changed("windows-keys", map[string]interface{}{"id": machineID})
}

// SetKey ghi hoặc thay key của một máy.
func (s *WindowsKeyService) SetKey(machineID, key, edition, note, actorID string) error {
	norm, ok := NormalizeWindowsKey(key)
	if !ok {
		return ErrWindowsKeyInvalid
	}
	m, err := s.machine(machineID)
	if err != nil {
		return err
	}
	row := model.WindowsKey{
		MachineID: machineID, Key: norm,
		Edition: truncateRunes(strings.TrimSpace(edition), 40),
		Note:    truncateRunes(strings.TrimSpace(note), 255),
	}
	if err := s.upsert([]model.WindowsKey{row}); err != nil {
		return err
	}
	s.log(actorID, "windows_key_set", machineID, map[string]interface{}{"machine_code": m.MachineCode, "tail": norm[len(norm)-5:]})
	s.changed(machineID)
	return nil
}

func (s *WindowsKeyService) upsert(rows []model.WindowsKey) error {
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "machine_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"key", "edition", "note", "updated_at"}),
	}).Create(&rows).Error
}

// DeleteKey bỏ key của máy (máy vẫn còn).
func (s *WindowsKeyService) DeleteKey(machineID, actorID string) error {
	m, err := s.machine(machineID)
	if err != nil {
		return err
	}
	res := s.db.Where("machine_id = ?", machineID).Delete(&model.WindowsKey{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrWindowsKeyNotFound
	}
	s.log(actorID, "windows_key_delete", machineID, map[string]interface{}{"machine_code": m.MachineCode})
	s.changed(machineID)
	return nil
}

// RevealKey trả key đầy đủ và ghi nhật ký người xem.
func (s *WindowsKeyService) RevealKey(machineID, actorID string) (string, error) {
	m, err := s.machine(machineID)
	if err != nil {
		return "", err
	}
	var k model.WindowsKey
	if err := s.db.Where("machine_id = ?", machineID).First(&k).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrWindowsKeyNotFound
		}
		return "", err
	}
	s.log(actorID, "windows_key_reveal", machineID, map[string]interface{}{"machine_code": m.MachineCode})
	return k.Key, nil
}

// ImportRow là kết quả của một dòng trong lần nhập hàng loạt.
type ImportRow struct {
	Line        int    `json:"line"`
	MachineCode string `json:"machine_code"`
	KeyMasked   string `json:"key_masked"`
	// ok | replaced (đã có key, ghi đè) | exists (đã có key, bỏ qua) | unknown_machine |
	// invalid_line | invalid_key | duplicate (máy này đã xuất hiện ở dòng trên)
	Status string `json:"status"`
}

type ImportResult struct {
	Rows    []ImportRow `json:"rows"`
	Applied int         `json:"applied"`
	DryRun  bool        `json:"dry_run"`
}

type parsedImport struct {
	line    int
	code    string
	key     string
	invalid string // lý do khi dòng hỏng
}

// parseImport đọc từng dòng "mã-máy key" (hoặc "key mã-máy"; dấu cách, tab, phẩy, chấm phẩy đều
// được). Dòng trống và dòng bắt đầu bằng # bị bỏ. Trả lại tối đa windowsKeyImportMax dòng.
func parseImport(text string) ([]parsedImport, error) {
	var out []parsedImport
	for i, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(out) == windowsKeyImportMax {
			return nil, ErrWindowsKeyTooMany
		}
		p := parsedImport{line: i + 1}
		fields := importSplitRe.Split(line, -1)
		if len(fields) != 2 {
			p.invalid = "invalid_line"
			out = append(out, p)
			continue
		}
		// Token nào đọc được thành key thì là key; token còn lại là mã máy.
		if k, ok := NormalizeWindowsKey(fields[0]); ok {
			p.key, p.code = k, fields[1]
		} else if k, ok := NormalizeWindowsKey(fields[1]); ok {
			p.key, p.code = k, fields[0]
		} else {
			p.code, p.invalid = fields[0], "invalid_key"
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, ErrWindowsKeyEmptyImport
	}
	return out, nil
}

// Import nhập hàng loạt. dryRun=true chỉ báo từng dòng sẽ ra sao, không ghi gì: giao diện cho
// người dùng xem trước rồi mới bấm nhập thật. overwrite=false thì máy đã có key bị bỏ qua.
func (s *WindowsKeyService) Import(text string, overwrite, dryRun bool, actorID string) (*ImportResult, error) {
	items, err := parseImport(text)
	if err != nil {
		return nil, err
	}

	var codes []string
	asked := map[string]bool{}
	for _, it := range items {
		if c := strings.ToLower(it.code); it.invalid == "" && !asked[c] {
			asked[c] = true
			codes = append(codes, c)
		}
	}
	type mrow struct{ ID, MachineCode string }
	byCode := map[string]mrow{}
	if len(codes) > 0 {
		var ms []mrow
		if err := s.db.Table("machines").Select("id, machine_code").
			Where("lower(machine_code) IN ? AND deleted_at IS NULL", codes).Scan(&ms).Error; err != nil {
			return nil, err
		}
		for _, m := range ms {
			byCode[strings.ToLower(m.MachineCode)] = m
		}
	}
	has := map[string]bool{}
	if len(byCode) > 0 {
		var ids []string
		for _, m := range byCode {
			ids = append(ids, m.ID)
		}
		var existing []string
		if err := s.db.Model(&model.WindowsKey{}).Where("machine_id IN ?", ids).Pluck("machine_id", &existing).Error; err != nil {
			return nil, err
		}
		for _, id := range existing {
			has[id] = true
		}
	}

	res := &ImportResult{DryRun: dryRun, Rows: make([]ImportRow, 0, len(items))}
	var toWrite []model.WindowsKey
	seen := map[string]bool{}
	for _, it := range items {
		row := ImportRow{Line: it.line, MachineCode: it.code}
		if it.key != "" {
			row.KeyMasked = MaskWindowsKey(it.key)
		}
		switch {
		case it.invalid != "":
			row.Status = it.invalid
		default:
			m, ok := byCode[strings.ToLower(it.code)]
			switch {
			case !ok:
				row.Status = "unknown_machine"
			case seen[m.ID]:
				row.Status = "duplicate"
			case has[m.ID] && !overwrite:
				row.Status = "exists"
				seen[m.ID] = true
			default:
				row.MachineCode = m.MachineCode
				row.Status = "ok"
				if has[m.ID] {
					row.Status = "replaced"
				}
				seen[m.ID] = true
				toWrite = append(toWrite, model.WindowsKey{MachineID: m.ID, Key: it.key})
			}
		}
		res.Rows = append(res.Rows, row)
	}

	if dryRun || len(toWrite) == 0 {
		return res, nil
	}
	// Ghi cả lô trong MỘT câu lệnh: hoặc cả lô vào, hoặc không dòng nào.
	if err := s.upsert(toWrite); err != nil {
		return nil, err
	}
	res.Applied = len(toWrite)
	s.log(actorID, "windows_key_import", "", map[string]interface{}{"count": res.Applied, "overwrite": overwrite})
	s.hub.Changed("windows-keys", map[string]interface{}{"import": res.Applied})
	return res, nil
}
