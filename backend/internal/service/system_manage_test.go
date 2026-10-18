package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Danh sách menu là danh sách PHẲNG: mỗi mục đúng một dòng. Bản cũ vừa đưa mục
// con vào danh sách, vừa gắn nó vào Children của mục cha, nên el-table trải
// thêm một lần nữa — 33 mục hiện hai lần và Vue kêu "Duplicate keys".
func TestSystemManageService_GetMenuList_KhongLapMucCon(t *testing.T) {
	db, _ := newMockDB(t)
	svc := NewSystemManageService(db, NewAuditService(db))

	res, err := svc.GetMenuList(&SystemListParams{Current: 1, Size: 100})

	require.NoError(t, err)
	menus, ok := res.Records.([]*MenuManageResponse)
	require.True(t, ok)

	dem := map[string]int{}
	var duyet func(list []*MenuManageResponse)
	duyet = func(list []*MenuManageResponse) {
		for _, m := range list {
			dem[m.ID]++
			duyet(m.Children)
		}
	}
	duyet(menus)

	for id, n := range dem {
		assert.Equal(t, 1, n, "mục menu %q xuất hiện %d lần", id, n)
	}
	// Size 100 đủ chứa toàn bộ cây menu nên trang này là tất cả: tổng phải khớp.
	assert.Equal(t, int64(len(menus)), res.Total)
}
