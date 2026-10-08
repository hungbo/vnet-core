package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// Tự cập nhật, do DỊCH VỤ NỀN làm.
//
// Bản cũ chỉ tải tệp về thư mục tạm của người dùng rồi bảo "đóng ứng dụng rồi
// chạy tệp này". Không làm được: tệp trong Program Files đang bị dịch vụ giữ,
// giao diện bị dịch vụ bật lại sau năm giây, và tài khoản khách không có quyền
// ghi vào Program Files. Cả quán phải đi cài tay từng máy.
//
// Dịch vụ chạy dưới SYSTEM nên làm được trọn: hỏi máy chủ, tải, kiểm băm, đợi
// lúc máy KHÔNG có khách, đổi tệp, rồi tự thoát để Windows bật lại bằng bản mới.
const (
	autoUpdateFirstCheck = 90 * time.Second
	autoUpdateEvery      = 10 * time.Minute
)

func runAutoUpdate(ctx context.Context, cfg *Config) {
	donTepCu()
	t := time.NewTimer(autoUpdateFirstCheck)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		donTepCu() // lần trước tệp .old có thể còn bị giữ
		if err := thuTuCapNhat(cfg); err != nil {
			log.Printf("[cập nhật] %v", err)
		}
		t.Reset(autoUpdateEvery)
	}
}

func thuTuCapNhat(cfg *Config) error {
	info, err := checkUpdate(cfg)
	if err != nil || info == nil || !info.HasUpdate {
		return err
	}
	if info.MachineInUse {
		log.Printf("[cập nhật] có bản %s nhưng máy đang có khách — để lần sau", info.Version)
		return nil
	}
	path, err := downloadAndVerify(info)
	if err != nil {
		return err
	}
	// Hỏi lại ngay trước khi đổi tệp: tải mất vài chục giây, khách có thể đã
	// ngồi vào trong lúc đó.
	lai, err := checkUpdate(cfg)
	if err != nil || lai == nil || lai.MachineInUse || lai.Version != info.Version {
		return fmt.Errorf("hoãn cài bản %s: máy vừa có khách hoặc bản phát hành đổi", info.Version)
	}
	return apDungBanMoi(path, info.Version)
}

// apDungBanMoi thay tệp đang chạy bằng bản mới rồi thoát tiến trình.
//
// Windows cho ĐỔI TÊN tệp .exe đang chạy nhưng không cho ghi đè nó, nên: đổi
// tên bản cũ sang .old, chép bản mới vào đúng chỗ, tắt mọi tiến trình giao diện
// đang chạy bản cũ, rồi thoát với mã lỗi. Chế độ khôi phục của dịch vụ (đặt lúc
// cài) bật lại dịch vụ sau năm giây — lần này là bản mới — và dịch vụ bật lại
// giao diện.
func apDungBanMoi(newPath, ver string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// Bản .old của lần trước có thể còn bị giữ (giao diện bản cũ chưa tắt hẳn)
	// nên không xoá được; khi đó đổi sang một tên chưa ai dùng thay vì bỏ cuộc.
	old := exe + ".old"
	if err := os.Remove(old); err != nil && !os.IsNotExist(err) {
		old = fmt.Sprintf("%s.old-%d", exe, time.Now().Unix())
	}
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("không đổi tên được bản đang chạy: %w", err)
	}
	if err := chepTep(newPath, exe); err != nil {
		_ = os.Remove(exe)
		_ = os.Rename(old, exe)
		return fmt.Errorf("không chép được bản mới, đã trả lại bản cũ: %w", err)
	}
	log.Printf("[cập nhật] đã cài bản %s — khởi động lại máy trạm", ver)

	tatGiaoDienCu()
	os.Exit(3)
	return nil
}

func chepTep(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// tatGiaoDienCu tắt mọi tiến trình vnet-client khác (giao diện, cửa sổ phụ).
func tatGiaoDienCu() {
	procs, err := process.Processes()
	if err != nil {
		return
	}
	self := int32(os.Getpid())
	for _, p := range procs {
		if p.Pid == self {
			continue
		}
		name, err := p.Name()
		if err != nil {
			continue
		}
		// So theo TIỀN TỐ: tệp đang chạy vừa bị đổi tên sang .exe.old, và
		// Windows báo tên tiến trình theo đường dẫn hiện tại của tệp — so khớp
		// đúng "vnet-client.exe" là bỏ sót chính giao diện bản cũ.
		if strings.HasPrefix(strings.ToLower(name), "vnet-client") {
			if err := p.Kill(); err != nil {
				log.Printf("[cập nhật] không tắt được %s (PID %d): %v", name, p.Pid, err)
			}
		}
	}
}

// donTepCu xoá bản .old còn lại sau lần cập nhật trước. Lúc đó nó vẫn đang
// chạy nên không xoá được; giờ thì được.
func donTepCu() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cu, _ := filepath.Glob(exe + ".old*")
	for _, f := range cu {
		_ = os.Remove(f)
	}
	// Các bản đã tải về: cài xong thì vô dụng, để lại thì mỗi lần cập nhật
	// thêm một tệp 13 MB. Thư mục tạm cũ là chỗ bản trước từng tải vào.
	if dir, err := updateDir(); err == nil {
		_ = os.RemoveAll(dir)
	}
	_ = os.RemoveAll(filepath.Join(os.TempDir(), "vnet-update"))
}
