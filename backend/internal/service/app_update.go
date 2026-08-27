package service

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

// Cập nhật máy khách.
//
// Máy chủ công bố phiên bản kèm đường tải và BĂM của tệp; máy trạm so phiên bản
// rồi tải về, kiểm băm, mới chạy.
//
// Băm là bắt buộc. Tải một tệp thực thi về rồi chạy mà không kiểm băm nghĩa là
// ai đứng giữa đường truyền cũng chạy được mã tuỳ ý trên toàn bộ máy trạm — một
// bản cập nhật hỏng còn đỡ hơn một bản cập nhật bị tráo.

type AppUpdateService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewAppUpdateService(db *gorm.DB, audit *AuditService) *AppUpdateService {
	return &AppUpdateService{db: db, audit: audit}
}

var (
	versionRe  = regexp.MustCompile(`^\d+(\.\d+){0,3}$`)
	checksumRe = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
	platformRe = regexp.MustCompile(`^[a-z0-9]+-[a-z0-9]+$`)
)

type CreateAppUpdateRequest struct {
	Version    string `json:"version" binding:"required"`
	Platform   string `json:"platform" binding:"required"`
	FileURL    string `json:"file_url" binding:"required"`
	Checksum   string `json:"checksum" binding:"required"`
	FileSize   int64  `json:"file_size"`
	Changelog  string `json:"changelog"`
	IsRequired bool   `json:"is_required"`
}

func (s *AppUpdateService) Create(req *CreateAppUpdateRequest, actorID string) (*model.AppUpdate, error) {
	version := strings.TrimSpace(strings.TrimPrefix(req.Version, "v"))
	if !versionRe.MatchString(version) {
		return nil, errors.New("phiên bản phải có dạng số, ví dụ 1.2.0")
	}
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if !platformRe.MatchString(platform) {
		return nil, errors.New("nền tảng phải có dạng hệ-kiếntrúc, ví dụ windows-amd64")
	}
	checksum := strings.ToLower(strings.TrimSpace(req.Checksum))
	if !checksumRe.MatchString(checksum) {
		return nil, errors.New("băm phải là SHA-256 dạng hex 64 ký tự")
	}
	url := strings.TrimSpace(req.FileURL)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, errors.New("đường tải phải bắt đầu bằng http:// hoặc https://")
	}

	var dup int64
	s.db.Model(&model.AppUpdate{}).
		Where("version = ? AND platform = ?", version, platform).Count(&dup)
	if dup > 0 {
		return nil, fmt.Errorf("phiên bản %s cho %s đã tồn tại", version, platform)
	}

	up := model.AppUpdate{
		Version: version, Platform: platform, FileURL: url,
		Checksum: checksum, FileSize: req.FileSize,
		Changelog: req.Changelog, IsRequired: req.IsRequired, IsActive: true,
	}
	if err := s.db.Create(&up).Error; err != nil {
		return nil, err
	}
	s.log("publish_app_update", up.ID, actorID, map[string]interface{}{
		"version": up.Version, "platform": up.Platform, "required": up.IsRequired,
	})
	return &up, nil
}

func (s *AppUpdateService) SetActive(id string, active bool, actorID string) error {
	var up model.AppUpdate
	if err := s.db.First(&up, "id = ?", id).Error; err != nil {
		return errors.New("không tìm thấy bản cập nhật")
	}
	if err := s.db.Model(&up).Update("is_active", active).Error; err != nil {
		return err
	}
	s.log("set_app_update_active", id, actorID, map[string]interface{}{"active": active})
	return nil
}

// Delete xoá một bản cập nhật máy trạm.
//
// Không bảng nào tham chiếu tới app_updates, nhưng MÁY TRẠM thì có: Latest()
// đọc bản đang bật để quyết định có tự cập nhật hay không. Xoá thẳng bản đang
// phát hành giữa chừng làm những máy đang tải dở mất nguồn.
func (s *AppUpdateService) Delete(id, actorID string) error {
	var up model.AppUpdate
	if err := s.db.First(&up, "id = ?", id).Error; err != nil {
		return errors.New("không tìm thấy bản cập nhật")
	}
	if up.IsActive {
		return chanVi("bản cập nhật đang phát hành — hãy tắt phát hành trước khi xoá")
	}
	if err := s.db.Delete(&up).Error; err != nil {
		return err
	}
	s.log("delete_app_update", id, actorID, nil)
	return nil
}

type AppUpdateListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Platform string `form:"platform"`
}

func (s *AppUpdateService) List(req *AppUpdateListRequest) (*pagination.Result, error) {
	page, size := req.Page, req.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	q := s.db.Model(&model.AppUpdate{})
	if req.Platform != "" {
		q = q.Where("platform = ?", strings.ToLower(req.Platform))
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []model.AppUpdate
	if err := q.Order("created_at desc").Offset((page - 1) * size).Limit(size).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return &pagination.Result{Items: rows, Total: total, Page: page, PageSize: size}, nil
}

type LatestUpdateResult struct {
	HasUpdate  bool   `json:"has_update"`
	Version    string `json:"version,omitempty"`
	FileURL    string `json:"file_url,omitempty"`
	Checksum   string `json:"checksum,omitempty"`
	FileSize   int64  `json:"file_size,omitempty"`
	Changelog  string `json:"changelog,omitempty"`
	IsRequired bool   `json:"is_required"`
	Current    string `json:"current"`
}

// Latest cho biết máy trạm ở phiên bản current có bản mới hơn không.
func (s *AppUpdateService) Latest(platform, current string) (*LatestUpdateResult, error) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return nil, errors.New("thiếu nền tảng")
	}
	current = strings.TrimSpace(strings.TrimPrefix(current, "v"))

	var rows []model.AppUpdate
	if err := s.db.Where("platform = ? AND is_active = ?", platform, true).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	res := &LatestUpdateResult{Current: current}
	var best *model.AppUpdate
	for i := range rows {
		if best == nil || compareVersions(rows[i].Version, best.Version) > 0 {
			best = &rows[i]
		}
	}
	if best == nil || compareVersions(best.Version, current) <= 0 {
		return res, nil
	}

	res.HasUpdate = true
	res.Version = best.Version
	res.FileURL = best.FileURL
	res.Checksum = best.Checksum
	res.FileSize = best.FileSize
	res.Changelog = best.Changelog
	res.IsRequired = best.IsRequired
	return res, nil
}

// compareVersions so hai phiên bản dạng số chấm. Trả -1, 0, 1.
//
// So chuỗi thẳng là sai: "1.10.0" < "1.9.0" theo thứ tự chữ cái, nên máy đang ở
// 1.9.0 sẽ không bao giờ thấy bản 1.10.0.
func compareVersions(a, b string) int {
	pa := versionParts(a)
	pb := versionParts(b)
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	if v == "" {
		return nil
	}
	fields := strings.Split(v, ".")
	out := make([]int, 0, len(fields))
	for _, f := range fields {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			// Phần không phải số (ví dụ "1.2.0-beta") coi như 0 thay vì làm
			// hỏng cả phép so.
			n = 0
		}
		out = append(out, n)
	}
	return out
}

func (s *AppUpdateService) log(action, id, actorID string, meta map[string]interface{}) {
	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	s.audit.Log(&LogAuditRequest{
		Action: action, EntityType: "app_update", EntityID: id,
		UserID: actor, Metadata: meta,
	})
}
