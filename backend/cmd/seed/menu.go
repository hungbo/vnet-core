package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

// Thực đơn mẫu để thử nghiệm: 100 món chia bốn nhóm, giá theo mặt bằng quán net
// Việt Nam. Chỉ chạy khi gọi `go run ./cmd/seed -menu`, nên lệnh seed mặc định
// vẫn y như cũ.
//
// Ảnh được SINH RA lúc seed chứ không kèm sẵn trong repo: mỗi món một tệp SVG
// trong thư mục upload của máy chủ. Làm vậy vì hai lẽ — ảnh chụp thật của 100
// món là thứ không thể tự bịa cho đúng, và ảnh dẫn từ máy chủ ngoài sẽ chết ngay
// khi quán không có Internet, đúng lúc cần thử nghiệm nhất.

type menuItem struct {
	name  string
	price int64
	emoji string
	unit  string
}

type menuCategory struct {
	name string
	slug string
	icon string
	// Màu nền ảnh, dạng góc màu HSL. Mỗi món trong nhóm lệch đi một chút để
	// 100 ảnh không trông giống hệt nhau khi xếp cạnh nhau trong thực đơn.
	hue   int
	items []menuItem
}

var menuCategories = []menuCategory{
	{
		name: "Mì", slug: "mi", icon: "🍜", hue: 12,
		items: []menuItem{
			{"Mì Hảo Hảo tôm chua cay", 15000, "🍜", "goi"},
			{"Mì Hảo Hảo sa tế hành tím", 15000, "🍜", "goi"},
			{"Mì Omachi sườn hầm ngũ quả", 20000, "🍜", "goi"},
			{"Mì Omachi bò hầm", 20000, "🍜", "goi"},
			{"Mì 3 Miền tôm chua cay", 15000, "🍜", "goi"},
			{"Mì Kokomi đại tôm chua cay", 15000, "🍜", "goi"},
			{"Mì Gấu Đỏ thịt bằm", 15000, "🍜", "goi"},
			{"Mì Cung Đình lẩu Thái tôm", 25000, "🍲", "to"},
			{"Mì trứng xúc xích", 30000, "🍜", "to"},
			{"Mì bò tái", 40000, "🍲", "to"},
			{"Mì xào bò", 45000, "🍝", "dia"},
			{"Mì xào hải sản", 50000, "🍝", "dia"},
			{"Mì trứng ốp la", 30000, "🍳", "to"},
			{"Mì tôm thịt bằm", 35000, "🍜", "to"},
			{"Mì cay Hàn Quốc", 45000, "🌶️", "to"},
			{"Mì Ý sốt bò bằm", 50000, "🍝", "dia"},
			{"Miến trộn trứng", 35000, "🍲", "to"},
			{"Phở bò ăn liền", 25000, "🍜", "to"},
			{"Bún bò Huế ăn liền", 25000, "🍜", "to"},
			{"Hủ tiếu Nam Vang ăn liền", 25000, "🍜", "to"},
		},
	},
	{
		name: "Cơm", slug: "com", icon: "🍚", hue: 50,
		items: []menuItem{
			{"Cơm rang dưa bò", 45000, "🍛", "dia"},
			{"Cơm rang thập cẩm", 45000, "🍛", "dia"},
			{"Cơm rang trứng", 35000, "🍳", "dia"},
			{"Cơm rang hải sản", 55000, "🍤", "dia"},
			{"Cơm gà xối mỡ", 50000, "🍗", "dia"},
			{"Cơm sườn nướng", 50000, "🍖", "dia"},
			{"Cơm sườn trứng ốp la", 55000, "🍖", "dia"},
			{"Cơm thịt kho trứng", 45000, "🍚", "dia"},
			{"Cơm gà rán", 55000, "🍗", "dia"},
			{"Cơm bò lúc lắc", 60000, "🥩", "dia"},
			{"Cơm cá kho tộ", 50000, "🐟", "dia"},
			{"Cơm tôm rim", 55000, "🍤", "dia"},
			{"Cơm trứng chiên nước mắm", 35000, "🍳", "dia"},
			{"Cơm chiên Dương Châu", 50000, "🍛", "dia"},
			{"Cơm gà teriyaki", 55000, "🍗", "dia"},
			{"Cơm bò sốt tiêu đen", 60000, "🥩", "dia"},
			{"Cơm canh chua cá", 50000, "🐟", "suat"},
			{"Cơm sườn xào chua ngọt", 50000, "🍖", "dia"},
			{"Cơm rang kim chi", 45000, "🌶️", "dia"},
			{"Cơm cà ri gà", 55000, "🍛", "dia"},
		},
	},
	{
		name: "Nước uống", slug: "nuoc-uong", icon: "🥤", hue: 205,
		items: []menuItem{
			{"Nước suối Aquafina 500ml", 10000, "💧", "chai"},
			{"Nước suối Lavie 500ml", 10000, "💧", "chai"},
			{"Coca-Cola lon 330ml", 15000, "🥤", "lon"},
			{"Pepsi lon 330ml", 15000, "🥤", "lon"},
			{"7Up lon 330ml", 15000, "🥤", "lon"},
			{"Mirinda cam lon 330ml", 15000, "🍊", "lon"},
			{"Sprite lon 330ml", 15000, "🥤", "lon"},
			{"Sting dâu", 15000, "⚡", "chai"},
			{"Sting vàng", 15000, "⚡", "chai"},
			{"Red Bull", 20000, "🐂", "lon"},
			{"Number 1", 15000, "⚡", "chai"},
			{"Monster Energy", 30000, "😈", "lon"},
			{"Rockstar", 30000, "🎸", "lon"},
			{"Trà xanh Không Độ", 12000, "🍵", "chai"},
			{"Trà Ô Long Tea+", 12000, "🍵", "chai"},
			{"C2 chanh", 12000, "🍋", "chai"},
			{"Bò húc Thái", 20000, "🐂", "lon"},
			{"Sữa tươi Vinamilk 180ml", 12000, "🥛", "hop"},
			{"Sữa Milo hộp", 12000, "🥛", "hop"},
			{"Nước cam ép Twister", 15000, "🍊", "chai"},
			{"Nước ép ổi", 20000, "🥤", "ly"},
			{"Cà phê đen đá", 15000, "☕", "ly"},
			{"Cà phê sữa đá", 20000, "☕", "ly"},
			{"Bạc xỉu", 25000, "☕", "ly"},
			{"Cà phê lon Highlands", 20000, "☕", "lon"},
			{"Trà sữa trân châu", 30000, "🧋", "ly"},
			{"Trà đào cam sả", 30000, "🍑", "ly"},
			{"Trà chanh", 15000, "🍋", "ly"},
			{"Nước chanh muối", 15000, "🍋", "ly"},
			{"Soda chanh", 20000, "🥤", "ly"},
		},
	},
	{
		name: "Đồ ăn vặt", slug: "do-an-vat", icon: "🍿", hue: 320,
		items: []menuItem{
			{"Snack Oishi tôm cay", 10000, "🍤", "goi"},
			{"Snack Poca khoai tây", 12000, "🥔", "goi"},
			{"Snack Lay's vị tự nhiên", 20000, "🥔", "goi"},
			{"Snack O'Star bò nướng", 12000, "🥩", "goi"},
			{"Bim bim Toonies phô mai", 10000, "🧀", "goi"},
			{"Bánh gạo One One", 15000, "🍘", "goi"},
			{"Hướng dương Tam Tam", 15000, "🌻", "goi"},
			{"Hạt bí rang", 20000, "🌰", "goi"},
			{"Đậu phộng rang muối", 15000, "🥜", "goi"},
			{"Xúc xích Đức Việt nướng", 15000, "🌭", "cai"},
			{"Xúc xích phô mai", 20000, "🌭", "cai"},
			{"Chả cá viên chiên", 25000, "🍢", "phan"},
			{"Cá viên chiên", 25000, "🍡", "phan"},
			{"Khoai tây chiên", 30000, "🍟", "phan"},
			{"Gà rán 1 miếng", 35000, "🍗", "mieng"},
			{"Gà rán 3 miếng", 90000, "🍗", "phan"},
			{"Nem chua rán", 30000, "🍢", "phan"},
			{"Bánh mì trứng", 20000, "🥪", "cai"},
			{"Bánh mì pate", 20000, "🥖", "cai"},
			{"Bánh mì thịt nướng", 30000, "🥙", "cai"},
			{"Bánh bao nhân thịt", 20000, "🥟", "cai"},
			{"Trứng gà luộc", 8000, "🥚", "cai"},
			{"Trứng cút lộn", 15000, "🥚", "phan"},
			{"Ngô cay xào", 25000, "🌽", "phan"},
			{"Bánh Chocopie", 8000, "🍫", "cai"},
			{"Bánh Cosy marie", 15000, "🍪", "goi"},
			{"Bánh Custas", 10000, "🧁", "cai"},
			{"Kẹo Alpenliebe", 5000, "🍬", "goi"},
			{"Kem Merino ốc quế", 12000, "🍦", "cai"},
			{"Kem Celano", 15000, "🍨", "cai"},
		},
	},
}

// seedMenu ghi ảnh rồi chèn nhóm và món. Chạy lại nhiều lần được: món đã có thì
// bỏ qua, món từng bị xoá mềm thì khôi phục.
func seedMenu(db *gorm.DB, uploadDir string) error {
	imgDir := filepath.Join(uploadDir, "seed")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		return fmt.Errorf("không tạo được thư mục ảnh %s: %w", imgDir, err)
	}

	var created, restored, skipped int
	for ci, cat := range menuCategories {
		catID, err := upsertCategory(db, cat, ci)
		if err != nil {
			return err
		}

		for i, item := range cat.items {
			file := fmt.Sprintf("%s-%02d.svg", cat.slug, i+1)
			// Lệch tối đa ±18 độ quanh màu của nhóm. Bản đầu tôi cho lệch i*11
			// độ, tức 30 món quét trọn vòng màu — món mì thứ hai mươi ra màu
			// xanh dương y hệt nước ngọt, mất luôn ý nghĩa "nhìn màu biết nhóm".
			hue := (cat.hue + (i%7-3)*6 + 360) % 360
			if err := os.WriteFile(filepath.Join(imgDir, file), []byte(menuImageSVG(item, hue)), 0o644); err != nil {
				return fmt.Errorf("không ghi được ảnh %s: %w", file, err)
			}

			outcome, err := upsertProduct(db, item, catID, "/uploads/seed/"+file, i)
			if err != nil {
				return err
			}
			switch outcome {
			case "created":
				created++
			case "restored":
				restored++
			default:
				skipped++
			}
		}
	}

	fmt.Printf("Thực đơn mẫu: %d món mới, %d khôi phục, %d đã có sẵn (bỏ qua)\n", created, restored, skipped)
	fmt.Printf("  Ảnh: %s\n", imgDir)
	return nil
}

func upsertCategory(db *gorm.DB, cat menuCategory, sortOrder int) (string, error) {
	var c model.Category
	// Unscoped: chỉ mục duy nhất trên name tính cả bản ghi đã xoá mềm, nên tìm
	// bằng truy vấn mặc định sẽ không thấy rồi chèn mới và vỡ chỉ mục.
	// Find chứ không First: không tìm thấy là đường đi bình thường ở đây, mà
	// First coi đó là lỗi và ghi 100 dòng log đỏ cho một lần seed sạch.
	res := db.Unscoped().Where("name = ?", cat.name).Limit(1).Find(&c)
	if res.Error != nil {
		return "", res.Error
	}
	switch {
	case res.RowsAffected == 0:
		c = model.Category{Name: cat.name, Icon: cat.icon, SortOrder: sortOrder + 1, IsActive: true}
		if err := db.Create(&c).Error; err != nil {
			return "", fmt.Errorf("không tạo được nhóm %s: %w", cat.name, err)
		}
	case c.DeletedAt.Valid:
		if err := db.Unscoped().Model(&c).Updates(map[string]any{"deleted_at": nil, "is_active": true}).Error; err != nil {
			return "", fmt.Errorf("không khôi phục được nhóm %s: %w", cat.name, err)
		}
	}
	return c.ID, nil
}

func upsertProduct(db *gorm.DB, item menuItem, categoryID, imageURL string, sortOrder int) (string, error) {
	var p model.Product
	res := db.Unscoped().Where("name = ?", item.name).Limit(1).Find(&p)
	if res.Error != nil {
		return "", res.Error
	}
	switch {
	case res.RowsAffected == 0:
		unit := item.unit
		p = model.Product{
			CategoryID: &categoryID,
			Name:       item.name,
			Price:      item.price,
			ImageURL:   imageURL,
			UnitID:     &unit,
			IsRetail:   true,
			IsActive:   true,
			// Có tồn kho và còn hàng, nếu không thì đặt món nào cũng trôi qua mà
			// không đụng tới kho — mất luôn nhánh cần thử nhất.
			HasStock:     true,
			CurrentStock: 100,
			MinStock:     10,
			SortOrder:    sortOrder + 1,
		}
		if err := db.Create(&p).Error; err != nil {
			return "", fmt.Errorf("không tạo được món %s: %w", item.name, err)
		}
		return "created", nil
	case p.DeletedAt.Valid:
		unit := item.unit
		if err := db.Unscoped().Model(&p).Updates(map[string]any{
			"deleted_at":  nil,
			"category_id": categoryID,
			"price":       item.price,
			"image_url":   imageURL,
			"unit_id":     unit,
			"is_active":   true,
		}).Error; err != nil {
			return "", fmt.Errorf("không khôi phục được món %s: %w", item.name, err)
		}
		return "restored", nil
	}
	// Món đang dùng thì không đụng vào: giá và ảnh có thể đã được sửa tay.
	return "skipped", nil
}

// menuImageSVG dựng một ảnh vuông: nền chuyển sắc theo nhóm, emoji lớn ở giữa,
// tên món ngắt tối đa hai dòng bên dưới.
func menuImageSVG(item menuItem, hue int) string {
	l1, l2 := wrapTwoLines(item.name, 22)

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 400" width="400" height="400">`)
	fmt.Fprintf(&b, `<defs><linearGradient id="g" x1="0" y1="0" x2="0" y2="1">`+
		`<stop offset="0" stop-color="hsl(%d,62%%,52%%)"/><stop offset="1" stop-color="hsl(%d,58%%,34%%)"/>`+
		`</linearGradient></defs>`, hue, (hue+18)%360)
	fmt.Fprintf(&b, `<rect width="400" height="400" rx="28" fill="url(#g)"/>`)
	fmt.Fprintf(&b, `<circle cx="200" cy="158" r="96" fill="#ffffff" fill-opacity="0.14"/>`)
	fmt.Fprintf(&b, `<text x="200" y="200" font-size="118" text-anchor="middle">%s</text>`, xmlEscape(item.emoji))
	fmt.Fprintf(&b, `<text x="200" y="308" font-size="30" font-weight="600" text-anchor="middle" `+
		`fill="#ffffff" font-family="Segoe UI,Helvetica,Arial,sans-serif">%s</text>`, xmlEscape(l1))
	if l2 != "" {
		fmt.Fprintf(&b, `<text x="200" y="344" font-size="30" font-weight="600" text-anchor="middle" `+
			`fill="#ffffff" font-family="Segoe UI,Helvetica,Arial,sans-serif">%s</text>`, xmlEscape(l2))
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// wrapTwoLines ngắt tên món thành tối đa hai dòng theo ranh giới từ. Phần thừa
// bị cắt bằng dấu ba chấm — tên dài hơn thì tràn ra ngoài khung ảnh.
func wrapTwoLines(s string, limit int) (string, string) {
	if len([]rune(s)) <= limit {
		return s, ""
	}
	words := strings.Fields(s)
	var l1, l2 []string
	for _, w := range words {
		if len([]rune(strings.Join(append(l1, w), " "))) <= limit {
			l1 = append(l1, w)
			continue
		}
		if len([]rune(strings.Join(append(l2, w), " "))) <= limit {
			l2 = append(l2, w)
			continue
		}
		return strings.Join(l1, " "), strings.Join(l2, " ") + "…"
	}
	return strings.Join(l1, " "), strings.Join(l2, " ")
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
