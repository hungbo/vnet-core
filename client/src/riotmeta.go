package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// Giữ metadata Riot đúng bản trên Game Disk cho máy diskless.
//
// Riot ghi trạng thái "đã cài" ở C:\ProgramData\Riot Games (RiotClientInstalls.json,
// Metadata\<game>\*.manifest...). Máy diskless về bản image sau mỗi lần khởi động
// nên sau một bản vá, manifest trong image lệch với tệp game và Riot đòi Repair.
// Thay vì lưu lại image sau mỗi bản vá, quán đặt một bản metadata cạnh tệp game
// (VNET đã đẩy cùng bản cập nhật game) và chỉ cho client biết thư mục đó
// (clientPolicy.RiotMetadataSource). Dịch vụ nền (SYSTEM) chép nó sang
// C:\ProgramData\Riot Games lúc bật và mỗi phút sau đó.
//
// Chỉ chép một tệp khi BẢN NGUỒN đổi so với lần chép trước (hoặc lần đầu từ lúc
// dịch vụ bật mà đích khác nguồn). Riot tự ghi lại tệp trong ProgramData (vá ngay
// trên máy, đổi ngôn ngữ, Repair) thì để nguyên: chép đè mỗi phút là giành nhau với
// Riot và chính nó làm Riot đòi Repair.
//
// Bản do master tự chụp nằm ở <X>\_vnet\ProgramData\Riot Games (X là thư mục game
// mà máy trạm thấy), kèm <X>\_vnet\master_game_dir.txt ghi thư mục game trên master.
// Hai đường dẫn khác nhau thì tệp .json/.yaml/.yml được đổi tiền tố master thành X
// trước khi ghi; ở bố cục này tệp chữ luôn so theo nội dung chứ không theo cỡ + giờ sửa.
// Bố cục khác thì chép nguyên văn: bản đặt ở đó phải đã ghi đúng đường dẫn mà máy
// trạm nhìn thấy (ví dụ D:/Games/Riot Games).
const (
	riotMetaEvery = time.Minute
	// Chưa đồng bộ được lần nào với nguồn hiện tại thì thử lại nhanh: chính sách tới
	// qua nhịp tim vài chục giây sau khi dịch vụ bật, ổ game có thể gắn muộn, mà Riot
	// mở trong lúc đó là đọc metadata cũ của image và đòi Repair.
	riotMetaRetry = 5 * time.Second
	// Metadata Riot thật chừng vài chục tệp, vài chục MB. Nguồn vượt mức này gần như
	// chắc là admin trỏ nhầm vào thư mục game: chép nó là đổ hàng chục GB vào ổ C:
	// (write-back) của mọi máy, nên từ chối cả lượt.
	maxRiotMetaFiles = 2000
	maxRiotMetaBytes = 256 << 20
	// Dài hơn đường dẫn Windows thường gặp; dài nữa chắc là gõ nhầm.
	maxRiotMetaSource = 260
	// Bản ghi thư mục game của master, nằm trong <X>\_vnet cạnh ProgramData (ngoài nguồn).
	riotMetaRecord = "master_game_dir.txt"
	// Tệp chữ lớn hơn thì chép nguyên văn: master không bao giờ đưa đường dẫn của nó vào
	// một tệp như vậy (nó từ chối chụp).
	maxRiotRewriteBytes = 1 << 20
)

// riotMetaSkipped: mục không bao giờ chép, ở mọi cấp (rel tính từ nguồn). Vanguard là
// của từng máy, machine.cfg gắn với máy đã tạo ra nó, lockfile và journal/wal/shm của
// SQLite là của lúc Riot đang chạy, *.vnet-tmp là tệp tạm, Update là bản cập nhật Riot
// Client đang tải dở. Giống hệt snapshotSkipped ở master (gameupdate/capture.go).
func riotMetaSkipped(rel string, isDir bool) bool {
	l := strings.ToLower(filepath.ToSlash(rel))
	n := path.Base(l)
	if isDir {
		return n == "vanguard" || n == "logs" || l == "metadata/riotclientservices/update"
	}
	return n == "machine.cfg" || n == "lockfile" || strings.HasSuffix(n, ".lockfile") ||
		strings.HasSuffix(n, "-journal") || strings.HasSuffix(n, "-wal") ||
		strings.HasSuffix(n, "-shm") || strings.HasSuffix(n, ".vnet-tmp")
}

// chuanHoaNguonRiot nhận đường dẫn ổ đĩa tuyệt đối ("D:\Games\Riot Games\_meta")
// và trả lại nguyên văn đã cắt khoảng trắng, hoặc "" nếu không hợp lệ. Đường dẫn
// này được dịch vụ SYSTEM đem đi ĐỌC rồi ghi vào ProgramData, nên từ chối đường
// dẫn tương đối, UNC, ký tự điều khiển và mọi đoạn "..".
func chuanHoaNguonRiot(raw string) string {
	p := strings.TrimSpace(raw)
	if len(p) < 3 || len(p) > maxRiotMetaSource {
		return ""
	}
	drive := p[0]
	if !(drive >= 'A' && drive <= 'Z' || drive >= 'a' && drive <= 'z') || p[1] != ':' || (p[2] != '\\' && p[2] != '/') {
		return ""
	}
	for _, r := range p {
		if unicode.IsControl(r) || strings.ContainsRune(`*?<>"|`, r) {
			return ""
		}
	}
	for _, part := range strings.FieldsFunc(p[3:], func(r rune) bool { return r == '\\' || r == '/' }) {
		if part == ".." {
			return ""
		}
	}
	return p
}

// riotStamp là kích thước và giờ sửa của một tệp NGUỒN ở lần đồng bộ trước.
type riotStamp struct {
	size  int64
	mtime int64
}

// nguonRiotAnToan từ chối nguồn là gốc ổ đĩa, hoặc trùng / chứa / nằm trong thư
// mục đích: chép thư mục cha vào con của nó là mỗi phút lồng thêm một tầng.
func nguonRiotAnToan(src, dst string) error {
	s, d := filepath.Clean(src), filepath.Clean(dst)
	if filepath.Dir(s) == s {
		return errors.New("nguồn là gốc ổ đĩa: " + src)
	}
	ls, ld := strings.ToLower(s), strings.ToLower(d)
	sep := string(filepath.Separator)
	if ls == ld || strings.HasPrefix(ld, ls+sep) || strings.HasPrefix(ls, ld+sep) {
		return errors.New("nguồn trùng hoặc lồng với thư mục đích: " + src)
	}
	return nil
}

type riotFile struct {
	rel  string
	path string
	info fs.FileInfo
}

// syncRiotMetadata chép tệp thường từ src sang dst. done giữ riotStamp của từng
// tệp nguồn (khoá là đường dẫn tương đối viết thường) ở lần đồng bộ trước: nguồn
// không đổi thì bỏ qua dù đích ra sao. Chưa có trong done thì chỉ chép khi đích khác
// nguồn (với tệp chữ ở bố cục _vnet: khác bản đã đổi đường dẫn). Không xoá gì trong
// dst, không đi theo liên kết/junction, bỏ riotMetaSkipped ở mọi cấp. Nguồn quá
// maxRiotMetaFiles/maxRiotMetaBytes hoặc bản ghi master hỏng thì không chép gì. Trả về
// số tệp đã chép; tệp lỗi không chặn tệp khác, lỗi đầu tiên được trả về.
func syncRiotMetadata(src, dst string, done map[string]riotStamp) (int, error) {
	if err := nguonRiotAnToan(src, dst); err != nil {
		return 0, err
	}
	if st, err := os.Lstat(src); err != nil || !st.IsDir() {
		if err == nil {
			err = errors.New("không phải thư mục")
		}
		return 0, err
	}
	if err := plainDirChain(dst, dst); err != nil {
		return 0, err
	}
	vnet, from, to, err := riotRewriteFor(src)
	if err != nil {
		return 0, err
	}
	var first error
	note := func(err error) {
		if first == nil {
			first = err
		}
	}
	// Lượt 1: liệt kê và đếm, chưa ghi gì.
	var files []riotFile
	var total int64
	tooBig := errors.New("nguồn quá lớn, không giống thư mục metadata Riot: " + src)
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			note(err)
			return nil
		}
		rel, rerr := filepath.Rel(src, path)
		if rerr != nil || rel == "." {
			return nil
		}
		if riotMetaSkipped(rel, d.IsDir()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			note(ierr)
			return nil
		}
		total += info.Size()
		if len(files) >= maxRiotMetaFiles || total > maxRiotMetaBytes {
			return tooBig
		}
		files = append(files, riotFile{rel: rel, path: path, info: info})
		return nil
	})
	if errors.Is(err, tooBig) {
		return 0, tooBig
	}
	if err != nil {
		note(err)
	}
	// Lượt 2: chép.
	copied := 0
	for _, f := range files {
		key := strings.ToLower(f.rel)
		stamp := riotStamp{f.info.Size(), f.info.ModTime().UnixNano()}
		if prev, ok := done[key]; ok && prev == stamp {
			continue
		}
		target := filepath.Join(dst, f.rel)
		var cerr error
		if vnet && riotTextFile(f.rel) && stamp.size <= maxRiotRewriteBytes {
			// Tệp chữ của bản _vnet: cỡ + giờ sửa trùng không có nghĩa là đích đã đúng
			// (bản chép nguyên văn cũ dài bằng bản đã đổi đường dẫn), nên so nội dung.
			data, rerr := readBounded(f.path, stamp.size)
			if rerr != nil {
				note(rerr)
				continue
			}
			if from != "" {
				data = rewriteRiotPaths(data, from, to)
			}
			if sameRegularFile(target, data) {
				done[key] = stamp
				continue
			}
			cerr = writeKeepTime(dst, target, bytes.NewReader(data), int64(len(data)), f.info.ModTime())
		} else {
			if cur, serr := os.Lstat(target); serr == nil && cur.Mode().IsRegular() &&
				cur.Size() == stamp.size && cur.ModTime().UnixNano() == stamp.mtime {
				done[key] = stamp
				continue
			}
			cerr = copyKeepTime(dst, f.path, target, f.info)
		}
		if cerr != nil {
			note(cerr)
			continue
		}
		done[key] = stamp
		copied++
	}
	return copied, first
}

// riotRewriteFor: vnet=true khi src là <X>\_vnet\ProgramData\Riot Games. from/to (dạng `\`) chỉ
// có khi có bản ghi và đường dẫn master khác X; from=="" nghĩa là giữ nguyên chữ nhưng vẫn so nội dung.
// Bản ghi hỏng là lỗi: chép bản chụp mà không đổi đường dẫn là đưa đường dẫn master vào
// ProgramData, và Riot đòi Repair.
func riotRewriteFor(src string) (vnet bool, from, to string, err error) {
	c := filepath.Clean(src)
	vnetDir := filepath.Dir(filepath.Dir(c))
	if !strings.EqualFold(filepath.Base(c), "Riot Games") ||
		!strings.EqualFold(filepath.Base(filepath.Dir(c)), "ProgramData") ||
		!strings.EqualFold(filepath.Base(vnetDir), "_vnet") {
		return false, "", "", nil
	}
	x := filepath.Dir(vnetDir)
	// X là gốc ổ đĩa: không đổi được tiền tố master thành gốc ổ, mà chép nguyên văn là
	// đưa đường dẫn master vào ProgramData (Riot đòi Repair), nên từ chối như bản ghi hỏng.
	if filepath.Dir(x) == x {
		return false, "", "", errors.New("bản chụp _vnet nằm ngay gốc ổ đĩa, không đổi được đường dẫn master: " + src)
	}
	rec := filepath.Join(vnetDir, riotMetaRecord)
	st, err := os.Lstat(rec)
	if errors.Is(err, fs.ErrNotExist) {
		return true, "", "", nil
	}
	var data []byte
	switch {
	case err != nil:
	case !st.Mode().IsRegular():
		err = errors.New("không phải tệp thường")
	case st.Size() > 1024:
		err = errors.New("dài quá 1024 byte")
	default:
		data, err = readBounded(rec, st.Size())
	}
	// Ít nhất hai cấp: "D:" hay "D:\" trơn thì đổi tiền tố là đổi cả ổ đĩa.
	from = strings.TrimSuffix(strings.ReplaceAll(chuanHoaNguonRiot(string(data)), "/", `\`), `\`)
	if err == nil && len(strings.FieldsFunc(from, func(r rune) bool { return r == '\\' })) < 2 {
		err = errors.New(`không phải thư mục trên ổ đĩa (ví dụ D:\GameMaster\Riot Games)`)
	}
	if err != nil {
		return false, "", "", fmt.Errorf("bản ghi đường dẫn master (%s) không hợp lệ: %v", rec, err)
	}
	to = strings.ReplaceAll(x, "/", `\`)
	if asciiEqualFold([]byte(from), []byte(to)) {
		return true, "", "", nil
	}
	return true, from, to, nil
}

// Thay mọi chỗ thư mục game của master (from) bằng thư mục game máy trạm thấy (to), giữ kiểu dấu
// của từng chỗ: "D:/A/B", `D:\A\B`, JSON `D:\\A\\B`. Không phân biệt hoa thường với chữ ASCII.
// Ranh giới ĐẦU: byte trước không phải chữ/số. KHÔNG có ranh giới cuối: "<from> 2\x" là thư mục anh
// em trong GAME_ROOT của master, và cũng là anh em trong client_root của máy trạm, nên đổi tiền tố là đúng.
func rewriteRiotPaths(data []byte, from, to string) []byte {
	out := data
	for _, sep := range []string{"/", `\`, `\\`} {
		needle := []byte(strings.ReplaceAll(from, `\`, sep))
		repl := []byte(strings.ReplaceAll(to, `\`, sep))
		var b []byte
		last := 0
		for i := 0; i+len(needle) <= len(out); {
			if asciiEqualFold(out[i:i+len(needle)], needle) && (i == 0 || !isASCIIAlnum(out[i-1])) {
				b = append(append(b, out[last:i]...), repl...)
				i += len(needle)
				last = i
				continue
			}
			i++
		}
		if b != nil {
			out = append(b, out[last:]...)
		}
	}
	return out
}

// riotTextFile: tệp mà Riot ghi đường dẫn ở dạng chữ (.json/.yaml/.yml, mọi kiểu hoa thường).
func riotTextFile(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".json", ".yaml", ".yml":
		return true
	}
	return false
}

// asciiEqualFold so hai dãy byte, chỉ bỏ qua hoa thường của chữ ASCII; byte khác phải trùng hẳn.
func asciiEqualFold(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

func isASCIIAlnum(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
}

// plainDirChain kiểm root và mọi thư mục từ root xuống tới dir đều là thư mục
// thật: symlink, junction hay điểm phân tích khác (Go báo ModeSymlink/ModeIrregular)
// đều bị từ chối. Cấp chưa tồn tại thì bỏ qua. Vẫn còn một khe giữa lúc kiểm và lúc
// ghi; khe đó chỉ đóng được bằng ACL của thư mục (xem ghi chú bàn giao).
func plainDirChain(root, dir string) error {
	parts, err := dirParts(root, dir)
	if err != nil {
		return err
	}
	cur := root
	for _, part := range parts {
		cur = filepath.Join(cur, part)
		st, err := os.Lstat(cur)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !st.IsDir() || st.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			return errors.New("không phải thư mục thật (liên kết?): " + cur)
		}
	}
	return nil
}

// dirParts: các cấp từ root xuống dir, bắt đầu bằng "" (chính root).
func dirParts(root, dir string) ([]string, error) {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("đích nằm ngoài thư mục Riot: " + dir)
	}
	parts := []string{""}
	if rel != "." {
		parts = append(parts, strings.Split(rel, string(filepath.Separator))...)
	}
	return parts, nil
}

// mkdirPlain tạo từng cấp từ root xuống dir, và trước khi xuống cấp sau thì kiểm cấp
// vừa có là thư mục thật. MkdirAll đi xuyên junction: dịch vụ SYSTEM sẽ tạo thư mục
// ở nơi junction trỏ tới trước khi kịp kiểm.
func mkdirPlain(root, dir string) error {
	parts, err := dirParts(root, dir)
	if err != nil {
		return err
	}
	cur := root
	for _, part := range parts {
		cur = filepath.Join(cur, part)
		st, err := os.Lstat(cur)
		if errors.Is(err, fs.ErrNotExist) {
			if err = os.Mkdir(cur, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}
			st, err = os.Lstat(cur)
		}
		if err != nil {
			return err
		}
		if !st.IsDir() || st.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			return errors.New("không phải thư mục thật (liên kết?): " + cur)
		}
	}
	return nil
}

// copyKeepTime chép src (đúng như lúc liệt kê: tệp thường, cỡ info.Size()) sang dst
// bằng writeKeepTime. Khách có thể làm tệp nguồn trên ổ game phình ra giữa hai lượt.
func copyKeepTime(root, src, dst string, info fs.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if st, err := in.Stat(); err != nil {
		return err
	} else if !st.Mode().IsRegular() || st.Size() != info.Size() {
		return errors.New("tệp nguồn đổi giữa chừng: " + src)
	}
	return writeKeepTime(root, dst, in, info.Size(), info.ModTime())
}

// writeKeepTime ghi đúng size byte từ r qua tệp tạm cùng thư mục rồi đổi tên, để Riot
// không bao giờ đọc phải nửa tệp, và đặt giờ sửa bằng mtime của bản gốc (khoá để lần
// sau nhận ra là đã chép). r dài hơn hay ngắn hơn size thì không ghi gì.
func writeKeepTime(root, dst string, r io.Reader, size int64, mtime time.Time) error {
	// Khách là user thường vẫn tạo được junction/symlink trong ProgramData: dịch vụ
	// SYSTEM không được ghi xuyên qua chúng ra chỗ khác.
	if err := mkdirPlain(root, filepath.Dir(dst)); err != nil {
		return err
	}
	tmp := dst + ".vnet-tmp"
	// Xoá tệp tạm cũ (nếu là liên kết thì xoá chính liên kết) rồi tạo mới bằng
	// O_EXCL: không bao giờ mở lại và cắt một tên đoán trước được.
	os.Remove(tmp)
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, io.LimitReader(r, size+1))
	if err == nil && n != size {
		err = fmt.Errorf("tệp nguồn đổi cỡ giữa chừng: %s", dst)
	}
	if err == nil {
		err = out.Close()
	} else {
		out.Close()
	}
	if err == nil {
		err = os.Chtimes(tmp, mtime, mtime)
	}
	if err == nil {
		err = os.Rename(tmp, dst)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// readBounded đọc cả tệp name khi nó vẫn là tệp thường đúng size byte, và không bao
// giờ đọc quá size+1 byte.
func readBounded(name string, size int64) ([]byte, error) {
	in, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	if st, err := in.Stat(); err != nil {
		return nil, err
	} else if !st.Mode().IsRegular() || st.Size() != size {
		return nil, errors.New("tệp nguồn đổi giữa chừng: " + name)
	}
	data, err := io.ReadAll(io.LimitReader(in, size+1))
	if err == nil && int64(len(data)) != size {
		err = errors.New("tệp nguồn đổi cỡ giữa chừng: " + name)
	}
	return data, err
}

// sameRegularFile: name là tệp thường (không phải liên kết) có đúng các byte data.
func sameRegularFile(name string, data []byte) bool {
	st, err := os.Lstat(name)
	if err != nil || !st.Mode().IsRegular() || st.Size() != int64(len(data)) {
		return false
	}
	cur, err := readBounded(name, st.Size())
	return err == nil && bytes.Equal(cur, data)
}

// runRiotMetaKeeper đọc chính sách mỗi nhịp nên admin đổi đường dẫn là áp được
// ngay. Không có thư mục đích (không phải Windows) hoặc chưa đặt nguồn thì không làm gì.
func runRiotMetaKeeper(ctx context.Context) {
	lastErr, lastSrc := "", ""
	synced := false
	done := map[string]riotStamp{}
	for {
		if src, dst := chuanHoaNguonRiot(currentPolicy().RiotMetadataSource), riotMetaDest(); src != "" && dst != "" {
			if src != lastSrc {
				lastSrc, done, synced = src, map[string]riotStamp{}, false
			}
			n, err := syncRiotMetadata(src, dst, done)
			synced = synced || err == nil
			if n > 0 {
				log.Printf("[riot] đã chép %d tệp metadata từ %s", n, src)
			}
			switch {
			case err == nil:
				lastErr = ""
			case err.Error() != lastErr:
				lastErr = err.Error()
				log.Printf("[riot] chép metadata từ %s lỗi: %v", src, err)
			}
		}
		wait := riotMetaEvery
		if !synced {
			wait = riotMetaRetry
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}
