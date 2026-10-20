package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
)

// Phần "meta" của menu game ở quán: dấu version rẻ để máy trạm biết khi nào cần kéo lại danh sách (L5), và nhãn Hot/New
// cùng thứ tự theo lượt mở (L4).

const (
	// Nhãn hiện trên thẻ game ở menu máy trạm.
	MenuLabelHot = "hot"
	MenuLabelNew = "new"

	// menuHotWindow: lượt "Chơi" trong khoảng này mới tính cho xếp hạng và nhãn Hot.
	menuHotWindow = 30 * 24 * time.Hour
	// menuHotTop: tối đa bấy nhiêu game được nhãn Hot, và mỗi game phải có ít nhất menuHotMin lượt (quán mới mở chưa có số liệu
	// thì không gắn nhãn cho game nào).
	menuHotTop = 5
	menuHotMin = 5
	// menuNewWindow: game về quán trong khoảng này được nhãn New.
	menuNewWindow = 14 * 24 * time.Hour
	// menuStatsTTL: xếp hạng lượt mở đếm lại nhiều nhất mỗi lần này; menu được hỏi liên tục, còn lượt chơi đổi chậm.
	menuStatsTTL = 5 * time.Minute
)

// MenuVersion là dấu hiệu rẻ của dữ liệu menu: đổi khi một game, một cài đặt game, hay tên quán và logo (nhóm general) đổi
// (kể cả thêm/xoá). Tiến độ đang
// tải nằm ở engine chứ không ở DB nên không có mặt ở đây; máy trạm vẫn kéo lại danh sách khi còn game đang tải.
// Xếp hạng lượt mở đổi chậm và có hạn dùng riêng (menuStatsTTL), máy trạm cũng kéo lại định kỳ.
func (s *GameService) MenuVersion() (string, error) {
	var g struct {
		N   int64
		Max *time.Time
	}
	if err := s.db.Model(&model.Game{}).Select("COUNT(*) AS n, MAX(updated_at) AS max").Scan(&g).Error; err != nil {
		return "", err
	}
	var st struct{ Max *time.Time }
	if err := s.db.Model(&model.SystemSetting{}).Where("group_name IN ?", []string{GamesGroup, GeneralGroup}).Select("MAX(updated_at) AS max").Scan(&st).Error; err != nil {
		return "", err
	}
	ms := func(t *time.Time) int64 {
		if t == nil {
			return 0
		}
		return t.UnixMilli()
	}
	return fmt.Sprintf("%d.%d.%d", g.N, ms(g.Max), ms(st.Max)), nil
}

// menuStats giữ xếp hạng lượt mở đã đếm, để mỗi lần máy trạm hỏi menu không phải đếm lại bảng game_launches.
type menuStats struct {
	at    time.Time
	count map[string]int64 // game_id -> số lượt "Chơi" trong menuHotWindow
}

// launchCounts trả số lượt "Chơi" của từng game trong menuHotWindow, dùng lại kết quả trong menuStatsTTL. Lỗi (hay không có
// dữ liệu) trả nil: nhãn và thứ tự chỉ là phần thêm, không được làm hỏng menu.
func (s *GameService) launchCounts(now time.Time) map[string]int64 {
	s.mu.Lock()
	if s.stats != nil && now.Sub(s.stats.at) < menuStatsTTL {
		c := s.stats.count
		s.mu.Unlock()
		return c
	}
	s.mu.Unlock()

	var rows []struct {
		GameID string
		N      int64
	}
	if err := s.db.Model(&model.GameLaunch{}).Where("created_at > ?", now.Add(-menuHotWindow)).
		Select("game_id, COUNT(*) AS n").Group("game_id").Find(&rows).Error; err != nil {
		return nil
	}
	count := make(map[string]int64, len(rows))
	for _, r := range rows {
		count[r.GameID] = r.N
	}
	s.mu.Lock()
	s.stats = &menuStats{at: now, count: count}
	s.mu.Unlock()
	return count
}

// hotGames chọn các game được nhãn Hot: nhiều lượt nhất, tối đa menuHotTop, mỗi game phải đạt menuHotMin. Hoà thì theo tên.
func hotGames(games []model.Game, count map[string]int64) map[string]bool {
	type gc struct {
		id, name string
		n        int64
	}
	var list []gc
	for _, g := range games {
		if n := count[g.ID]; n >= menuHotMin {
			list = append(list, gc{g.ID, g.Name, n})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return strings.ToLower(list[i].name) < strings.ToLower(list[j].name)
	})
	hot := map[string]bool{}
	for i, x := range list {
		if i >= menuHotTop {
			break
		}
		hot[x.id] = true
	}
	return hot
}

// menuLabel là nhãn của một game: Hot thắng New.
func menuLabel(g *model.Game, hot map[string]bool, now time.Time) string {
	switch {
	case hot[g.ID]:
		return MenuLabelHot
	case !g.CreatedAt.IsZero() && now.Sub(g.CreatedAt) < menuNewWindow:
		return MenuLabelNew
	}
	return ""
}

// sortByLaunches xếp lại danh sách đã theo (ưu tiên giảm dần, tên): trong cùng mức ưu tiên, game nhiều lượt mở hơn lên
// trước. Ổn định nên cùng số lượt thì giữ thứ tự theo tên của truy vấn.
func sortByLaunches(games []model.Game, count map[string]int64) {
	if len(count) == 0 {
		return
	}
	sort.SliceStable(games, func(i, j int) bool {
		if games[i].Priority != games[j].Priority {
			return games[i].Priority > games[j].Priority
		}
		return count[games[i].ID] > count[games[j].ID]
	})
}
