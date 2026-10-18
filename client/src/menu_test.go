package main

import (
	"encoding/json"
	"testing"
)

// Giao diện máy khách chạy từ máy chủ tài nguyên nhúng của Wails, không phải từ
// máy chủ VNET, nên "/uploads/..." mà máy chủ trả về trỏ nhầm chỗ và mọi ảnh
// món ăn đều hỏng. Đây là chỗ duy nhất chữa việc đó.
func TestAbsoluteImageURLs(t *testing.T) {
	a := &App{cfg: &Config{ServerURL: "http://192.168.1.10:8080/"}}

	in := json.RawMessage(`[
		{"name":"Mì xào bò","image_url":"/uploads/seed/mi-11.svg"},
		{"name":"Đã tuyệt đối","image_url":"http://cdn.example.com/a.png"},
		{"name":"Không ảnh","image_url":""},
		{"name":"Thiếu trường"}
	]`)

	var got []map[string]any
	if err := json.Unmarshal(a.absoluteImageURLs(in), &got); err != nil {
		t.Fatalf("kết quả không phải JSON hợp lệ: %v", err)
	}

	// Dấu / thừa ở cuối ServerURL không được đẻ ra "//uploads".
	if got[0]["image_url"] != "http://192.168.1.10:8080/uploads/seed/mi-11.svg" {
		t.Errorf("đường dẫn tương đối: %v", got[0]["image_url"])
	}
	if got[1]["image_url"] != "http://cdn.example.com/a.png" {
		t.Errorf("đường dẫn tuyệt đối bị sửa: %v", got[1]["image_url"])
	}
	if got[2]["image_url"] != "" {
		t.Errorf("ảnh rỗng bị biến thành địa chỉ máy chủ: %v", got[2]["image_url"])
	}
	if _, ok := got[3]["image_url"]; ok {
		t.Errorf("món không có trường image_url bị thêm trường mới")
	}
	if len(got) != 4 {
		t.Errorf("mất món: còn %d/4", len(got))
	}
}

// Máy chủ trả lỗi hoặc đổi kiểu dữ liệu thì thà giữ nguyên còn hơn nuốt mất
// thực đơn — người dùng sẽ thấy lỗi thật thay vì một danh sách rỗng.
func TestAbsoluteImageURLs_GiuNguyenKhiKhongPhaiDanhSach(t *testing.T) {
	a := &App{cfg: &Config{ServerURL: "http://localhost:8080"}}

	in := json.RawMessage(`{"code":1,"message":"lỗi"}`)
	if string(a.absoluteImageURLs(in)) != string(in) {
		t.Errorf("dữ liệu không phải danh sách đã bị đổi: %s", a.absoluteImageURLs(in))
	}
}
