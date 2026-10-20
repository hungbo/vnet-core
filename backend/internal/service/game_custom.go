package service

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

// Trường theo từng game ở quán: lệnh chạy trước (RunFirst), và phần tuỳ chỉnh riêng của quán (tên hiển thị, icon,
// gốc game riêng, lệnh chạy trước riêng) mà catalog của Hub không bao giờ ghi đè.

// maxGameRunFirst: một game hiếm khi cần hơn vài bước chuẩn bị. Máy trạm chạy lần lượt và chờ từng bước, nên danh sách
// dài là khách chờ lâu.
const maxGameRunFirst = 5

// kiemTraBuocChayTruoc kiểm một bước: có tệp chạy tương đối trong thư mục game, tham số hợp lệ.
func kiemTraBuocChayTruoc(st model.GameRunStep) (model.GameRunStep, error) {
	l, a := strings.TrimSpace(st.Launcher), strings.TrimSpace(st.LaunchArgs)
	if l == "" {
		return st, errors.New("thiếu tệp chạy")
	}
	if err := kiemTraChuGame("tệp chạy", l); err != nil {
		return st, err
	}
	if err := checkLauncher(l); err != nil {
		return st, err
	}
	if err := kiemTraChuGame("tham số", a); err != nil {
		return st, err
	}
	if len(a) > 500 {
		return st, errors.New("tham số dài quá 500 ký tự")
	}
	return model.GameRunStep{Launcher: l, LaunchArgs: a}, nil
}

// chuanHoaRunFirst kiểm và cắt khoảng trắng các lệnh chạy trước do người dùng nhập. Rỗng trả nil.
func chuanHoaRunFirst(in []model.GameRunStep) (model.GameRunSteps, error) {
	if len(in) > maxGameRunFirst {
		return nil, fmt.Errorf("tối đa %d lệnh chạy trước", maxGameRunFirst)
	}
	var out model.GameRunSteps
	for i, st := range in {
		ok, err := kiemTraBuocChayTruoc(st)
		if err != nil {
			return nil, fmt.Errorf("lệnh chạy trước %d: %w", i+1, err)
		}
		out = append(out, ok)
	}
	return out, nil
}

// runFirstHopLe giữ các bước dùng được trong dữ liệu từ xa (catalog). Máy trạm sẽ chạy những lệnh này nên một bước hỏng
// bị bỏ riêng, chứ không bỏ cả game.
func runFirstHopLe(game string, in []model.GameRunStep) model.GameRunSteps {
	var out model.GameRunSteps
	for _, st := range in {
		ok, err := kiemTraBuocChayTruoc(st)
		if err != nil || len(out) >= maxGameRunFirst {
			log.Printf("[game] %s: bỏ lệnh chạy trước không hợp lệ từ catalog", game)
			continue
		}
		out = append(out, ok)
	}
	return out
}

// runFirstOf là lệnh chạy trước hiệu lực của game: phần tuỳ chỉnh của quán thay hẳn game gốc khi có.
func runFirstOf(g *model.Game) model.GameRunSteps {
	if g.Custom.RunFirst != nil {
		return *g.Custom.RunFirst
	}
	return g.RunFirst
}

// GameCustomRequest: con trỏ nil là giữ nguyên, chuỗi rỗng là bỏ tuỳ chỉnh đó (quay về game gốc).
type GameCustomRequest struct {
	DisplayName *string `json:"display_name"`
	Icon        *string `json:"icon"`
	ClientRoot  *string `json:"client_root"`
	// RunFirst có mặt (kể cả []) thì thay hẳn lệnh chạy trước của game gốc. ResetRunFirst bỏ phần thay, theo game gốc.
	RunFirst      *[]model.GameRunStep `json:"run_first"`
	ResetRunFirst bool                 `json:"reset_run_first"`
}

// SetCustom đặt phần tuỳ chỉnh riêng của quán trên một game (của Hub hay cục bộ). Master phát hành game nên sửa thẳng
// game, không có tuỳ chỉnh riêng.
func (s *GameService) SetCustom(id string, req *GameCustomRequest) (*model.Game, error) {
	if s.conf().Role == config.GameRoleMaster {
		return nil, ErrGameCafeOnly
	}
	if !laUUID(id) {
		return nil, gorm.ErrRecordNotFound
	}
	var g model.Game
	if err := s.db.First(&g, "id = ?", id).Error; err != nil {
		return nil, err
	}
	c := g.Custom
	if req.DisplayName != nil {
		if err := kiemTraCacTruongChuGame(req.DisplayName, nil, nil, nil); err != nil {
			return nil, err
		}
		c.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.Icon != nil {
		if err := kiemTraIcon(*req.Icon); err != nil {
			return nil, err
		}
		c.Icon = strings.TrimSpace(*req.Icon)
	}
	if req.ClientRoot != nil {
		root := strings.TrimSpace(*req.ClientRoot)
		if root != "" {
			p, err := parseGameClientRoot(root)
			if err != nil {
				return nil, fmt.Errorf("gốc game trên máy trạm %q: %w", root, err)
			}
			root = p
		}
		c.ClientRoot = root
	}
	switch {
	case req.ResetRunFirst:
		c.RunFirst = nil
	case req.RunFirst != nil:
		steps, err := chuanHoaRunFirst(*req.RunFirst)
		if err != nil {
			return nil, err
		}
		if steps == nil {
			steps = model.GameRunSteps{} // rỗng mà có mặt: bỏ hết lệnh chạy trước của game gốc
		}
		c.RunFirst = &steps
	}
	if err := s.db.Model(&g).Updates(map[string]interface{}{"custom": c}).Error; err != nil {
		return nil, err
	}
	if err := s.db.First(&g, "id = ?", id).Error; err != nil {
		return nil, err
	}
	s.baoDoi(g.ID)
	return &g, nil
}
