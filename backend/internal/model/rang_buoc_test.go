package model_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/model"
	"gorm.io/gorm/schema"
)

// Tag `constraint:` chỉ có tác dụng trên TRƯỜNG QUAN HỆ — một trường kiểu struct
// hoặc slice model, ví dụ `Member Member`. Viết nó trên trường ID vô hướng
// (`MemberID string`) thì GORM đọc xong rồi bỏ đi: không có REFERENCES nào được
// tạo, không có ON DELETE nào chạy.
//
// Package này từng mang 68 tag như thế. Chúng vô hại về mặt vận hành nhưng độc
// hại về mặt đọc hiểu: ai mở file ra cũng tin rằng xoá một Machine sẽ cascade
// sang machine_sessions, trong khi thực tế không có gì xảy ra và các hàm Delete
// thì không kiểm tra gì. Test này chặn việc đó quay lại.
//
// Muốn ràng buộc thật thì thêm trường quan hệ rồi đặt tag lên ĐÓ — test vẫn
// xanh, vì nó chỉ bắt tag nằm trên trường vô hướng.
func TestModel_KhongConTagConstraintChet(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)

	var viPham []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		cu, err := parser.ParseFile(fset, f, nil, 0)
		require.NoError(t, err, f)

		ast.Inspect(cu, func(n ast.Node) bool {
			st, ok := n.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range st.Fields.List {
				if field.Tag == nil {
					continue
				}
				raw, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					continue
				}
				gormTag := reflect.StructTag(raw).Get("gorm")
				if !strings.Contains(gormTag, "constraint:") {
					continue
				}
				// Trường quan hệ: kiểu là một struct/slice trong chính package
				// này, không phải string/*string/int.
				if laTruongQuanHe(field.Type) {
					continue
				}
				ten := "?"
				if len(field.Names) > 0 {
					ten = field.Names[0].Name
				}
				viPham = append(viPham, fset.Position(field.Pos()).String()+" "+ten)
			}
			return true
		})
	}

	assert.Empty(t, viPham,
		"tag constraint: nằm trên trường ID vô hướng thì GORM bỏ qua — hoặc gỡ nó đi, "+
			"hoặc thêm trường quan hệ và đặt tag lên trường đó")
}

// laTruongQuanHe nhận diện trường kiểu struct hoặc slice struct, tức là chỗ DUY
// NHẤT mà GORM đọc tag constraint.
func laTruongQuanHe(t ast.Expr) bool {
	switch v := t.(type) {
	case *ast.ArrayType:
		return laTruongQuanHe(v.Elt)
	case *ast.StarExpr:
		return laTruongQuanHe(v.X)
	case *ast.Ident:
		// Kiểu dựng sẵn (string, int, bool, float64...) bắt đầu bằng chữ thường.
		return v.Name != "" && v.Name[0] >= 'A' && v.Name[0] <= 'Z' && v.Name != "IntArray" && v.Name != "StringArray"
	}
	return false
}

// Chốt lại bức tranh khoá ngoại thật của hệ thống, để một thay đổi vô ý không
// lặng lẽ bật cascade ở tầng database lên.
//
// Chỉ user_roles và role_permissions có khoá ngoại, sinh từ các trường many2many
// trong user.go, và cả hai đều NO ACTION (chuỗi rỗng = mặc định của PostgreSQL)
// — tức là CHẶN xoá chứ không cascade. Đó là lý do DeleteRole phải tự dọn hai
// bảng nối này trước khi xoá vai trò.
func TestModel_ChiCoHaiKhoaNgoaiThat(t *testing.T) {
	cache := &sync.Map{}
	ns := schema.NamingStrategy{}

	// Những model mang nhiều cột khoá ngoại nhất: nếu ở đâu có quan hệ thật thì
	// phải lộ ra ở đây.
	khongCoQuanHe := []interface{}{
		&model.MachineSession{}, &model.Order{}, &model.OrderItem{},
		&model.Member{}, &model.Machine{}, &model.Product{},
		&model.MachineBooking{}, &model.ComboPurchase{}, &model.MemberTransaction{},
		&model.UserRole{}, &model.RolePermission{},
	}
	for _, m := range khongCoQuanHe {
		s, err := schema.Parse(m, cache, ns)
		require.NoError(t, err)
		assert.Empty(t, s.Relationships.Relations,
			"%s không khai trường quan hệ nào, nên GORM không tạo khoá ngoại cho nó", s.Table)
	}

	for _, m := range []interface{}{&model.User{}, &model.Role{}} {
		s, err := schema.Parse(m, cache, ns)
		require.NoError(t, err)
		require.NotEmpty(t, s.Relationships.Relations, "%s phải có quan hệ many2many", s.Table)
		for ten, rel := range s.Relationships.Relations {
			c := rel.ParseConstraint()
			require.NotNil(t, c, "%s.%s phải sinh khoá ngoại", s.Table, ten)
			assert.Empty(t, c.OnDelete,
				"%s.%s đang là NO ACTION (chặn xoá). Đổi sang CASCADE là đổi hành vi xoá vai trò "+
					"ở tầng database — phải sửa DeleteRole cho khớp trước", s.Table, ten)
		}
	}
}
