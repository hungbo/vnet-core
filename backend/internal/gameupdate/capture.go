package gameupdate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf16"
)

// Riot Client keeps its "installed" state outside the game folder, in %ProgramData%\Riot Games
// (RiotClientInstalls.json, Metadata\<product>\...). A workstation that boots from an image has a
// stale copy of it after every patch and Riot then asks for a Repair. So before fingerprinting a
// Riot game folder the master mirrors that state into the folder itself, where it travels to every
// cafe like any other file of the game:
//
//	<game>\_vnet\ProgramData\Riot Games   the snapshot; the VNET client copies it to C:\ProgramData
//	<game>\_vnet\master_game_dir.txt      the master's game folder, for the client's path rewrite
//
// Workstations see the game under another folder than the master does, so the client rewrites the
// master's game folder in the snapshot's text files (rewriteRiotPaths). The master refuses to
// capture anything that rewrite could not fix.
const (
	// MasterDirRecord is the record's file name, next to the snapshot in <game>\_vnet.
	MasterDirRecord = "master_game_dir.txt"
	// The client's own caps (maxRiotMetaFiles, maxRiotMetaBytes, maxRiotRewriteBytes): a snapshot it
	// would refuse, or a text file it would copy without rewriting, must not be published.
	maxSnapshotFiles = 2000
	maxSnapshotBytes = 256 << 20
	maxRewriteBytes  = 1 << 20
)

// beforeRecheck runs between validating the source and walking it a second time. A variable only so
// tests can play Riot writing in the middle of a capture.
var beforeRecheck = func() {}

// sqliteJournalHeader opens a rollback journal that still has to be replayed ("hot"). SQLite deletes,
// empties or zeroes the header of a journal it is done with.
var sqliteJournalHeader = []byte{0xd9, 0xd5, 0x05, 0xf9, 0x20, 0xa1, 0x63, 0xd7}

var (
	// Spaces and tabs only, never \s: a key with an empty value must not take the next line as its value.
	installRootLine  = regexp.MustCompile(`(?m)^[ \t]*(product_install_root|product_install_full_path)[ \t]*:[ \t]*([^\r\n]*?)[ \t]*\r?$`)
	shouldRepairLine = regexp.MustCompile(`(?mi)^[ \t]*should_repair[ \t]*:[ \t]*true\b`)
	// Riot writes one path as D:/x, D:\x, JSON's D:\\x and, rarely, D:\/x.
	riotSeps = strings.NewReplacer(`\\`, "/", `\/`, "/", `\`, "/")
	// Every run of separators, so residualMasterPath also finds "D://x" and JSON's "D:\\\\x".
	sepRun = regexp.MustCompile(`[/\\]+`)

	errSnapshotTooBig = errors.New(`ProgramData\Riot Games quá lớn (>2000 tệp hoặc >256 MB): máy trạm sẽ từ chối cả bản chụp; kiểm lại thư mục`)

	// ErrRiotElsewhere is not a failure: Riot runs from another folder and this one has no snapshot
	// yet. CaptureRiot touched nothing, and the caller publishes the folder like any other game.
	ErrRiotElsewhere = errors.New(`Riot Client chạy từ thư mục khác, chưa có bản chụp: không chụp ProgramData\Riot Games, máy trạm sẽ đòi Repair sau mỗi bản vá`) //nolint:staticcheck // ST1005: "Riot" is a name
)

// IsRiotGame reports whether gameDir holds the Riot Client (Riot Client\RiotClientServices.exe as a
// regular file), the only kind of game CaptureRiot is for.
func IsRiotGame(gameDir string) bool {
	st, err := os.Lstat(filepath.Join(gameDir, "Riot Client", "RiotClientServices.exe"))
	return err == nil && st.Mode().IsRegular()
}

// riotEntry is one source file as a walk saw it; two walks that agree mean Riot left it alone.
type riotEntry struct {
	rel   string
	size  int64
	mtime int64
}

type pendingFile struct {
	rel  string
	data []byte
}

// CaptureRiot checks src (...\ProgramData\Riot Games), mirrors it into
// gameDir\_vnet\ProgramData\Riot Games and writes gameDir\_vnet\master_game_dir.txt. Only files whose
// bytes differ are written, and their mtime is left as the write sets it, so a capture that changes
// nothing leaves the game's fingerprint alone. It returns how many files it wrote plus deleted.
//
// On an error the caller must not fingerprint or publish this pass. Nothing has been written then,
// except after a failed write or delete, or a link removed from the snapshot, which the next
// successful capture finishes before anything is fingerprinted. When RiotClientInstalls.json
// names a Riot Client outside gameDir and there is no snapshot yet, this is not the folder Riot
// runs from: it returns 0 and an error wrapping ErrRiotElsewhere, and touches nothing. The caller
// has checked IsRiotGame.
func CaptureRiot(src, gameDir string) (int, error) {
	gc := filepath.Clean(gameDir)
	dst := filepath.Join(gc, "_vnet", "ProgramData", "Riot Games")

	// 1. Never read through a link, never write through one.
	if err := checkRiotPaths(src, gc, dst); err != nil {
		return 0, err
	}
	// 1b. A folder Riot does not run from is a plain game: nothing below may refuse it for the state
	// of the Riot install it does not hold (a busy journal, a link, the size cap).
	if err := RiotElsewhere(src, gc); err != nil {
		return 0, err
	}

	// 2. List what belongs in the snapshot.
	entries, err := walkRiotSource(src)
	if err != nil {
		return 0, err
	}

	// 3. Without the install index the source is half there: keep the snapshot as it is.
	installs := -1
	for i, e := range entries {
		if strings.EqualFold(e.rel, "RiotClientInstalls.json") {
			installs = i
		}
	}
	if installs < 0 {
		return 0, fmt.Errorf("thiếu RiotClientInstalls.json trong %s", src)
	}

	// 4a. Riot must run from this game folder, checked again on the bytes the recheck vouches for.
	index, err := readEntry(src, entries[installs])
	if err != nil {
		return 0, err
	}
	if err := checkRcDefault(index, gc, dst); err != nil {
		return 0, err
	}

	// 4b-4e. Read every file once, validate it, and hold on only to what has to be written.
	var pending []pendingFile
	for i, e := range entries {
		data := index
		if i != installs {
			if data, err = readEntry(src, e); err != nil {
				return 0, err
			}
		}
		if err := checkRiotFile(filepath.Join(src, e.rel), e.rel, data, gc); err != nil {
			return 0, err
		}
		if !sameFile(filepath.Join(dst, e.rel), data) {
			pending = append(pending, pendingFile{e.rel, data})
		}
	}

	// 5. Riot must not have written anything while we read: PublishNow has no Stabilizer to wait for
	// a quiet folder.
	beforeRecheck()
	again, err := walkRiotSource(src)
	if err != nil {
		return 0, err
	}
	if rel := firstChange(entries, again); rel != "" {
		return 0, errRiotWrote(filepath.Join(src, rel))
	}

	// 7. Delete every other file: gone from the source, skipped now (an old hand-made copy of
	// Metadata\vanguard, machine.cfg) or a stale temp file. Before writing, because a stale file
	// where the source now has a folder would keep that folder from ever being created. A link is
	// removed as the link itself. Empty folders stay; they are neither published nor fingerprinted.
	// Not a folder where the source now has a file: it would stand in the way of that file's rename.
	n := 0
	keep := make(map[string]bool, len(entries))
	for _, e := range entries {
		keep[strings.ToLower(filepath.ToSlash(e.rel))] = true
	}
	link := ""
	var inTheWay []string
	err = filepath.WalkDir(dst, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if p == dst && errors.Is(walkErr, fs.ErrNotExist) {
				return nil // the first capture: nothing to delete
			}
			return fmt.Errorf("đọc %s: %v", p, walkErr)
		}
		rel, err := filepath.Rel(dst, p)
		if err != nil {
			return err
		}
		kept := keep[strings.ToLower(filepath.ToSlash(rel))]
		if d.IsDir() {
			if kept {
				inTheWay = append(inTheWay, p)
			}
			return nil
		}
		if kept {
			return nil
		}
		if err := os.Remove(p); err != nil {
			return fmt.Errorf("xoá %s: %v", p, err)
		}
		n++
		if !d.Type().IsRegular() && link == "" {
			link = p
		}
		return nil
	})
	if err != nil {
		return n, err
	}
	// No source file sits under another one, so the walk has emptied these of files: only empty
	// folders are left, removed deepest first.
	for _, dir := range inTheWay {
		removed, err := removeEmptyDirs(dir)
		n += removed
		if err != nil {
			return n, fmt.Errorf("xoá %s: %v", dir, err)
		}
	}
	// Step 4e read through that link and found the files behind it already in place, so none of
	// them is pending. This pass must not be fingerprinted; the next one writes them for real.
	if link != "" {
		return n, fmt.Errorf("đã gỡ liên kết %s trong bản chụp: các tệp sau nó chưa được chép, publish lại", link)
	}

	// 8. Write.
	for _, f := range pending {
		target := filepath.Join(dst, f.rel)
		if err := writeSnapshotFile(gc, target, f.data); err != nil {
			return n, fmt.Errorf("ghi %s: %v", target, err)
		}
		n++
	}

	// 9. The record.
	record := filepath.Join(gc, "_vnet", MasterDirRecord)
	if !sameFile(record, []byte(gc)) {
		if err := writeSnapshotFile(gc, record, []byte(gc)); err != nil {
			return n, fmt.Errorf("ghi %s: %v", record, err)
		}
		n++
	}
	return n, nil
}

// RiotElsewhere is CaptureRiot's rc_default check on its own, reading only. It returns an error
// wrapping ErrRiotElsewhere when RiotClientInstalls.json names a Riot Client outside gameDir and
// there is no snapshot yet, and nil otherwise, including whenever it cannot tell (a link, the index
// missing, being written or corrupt): CaptureRiot then reports the real problem. A caller runs it
// before any refusal that a plain game would not get.
func RiotElsewhere(src, gameDir string) error {
	gc := filepath.Clean(gameDir)
	dst := filepath.Join(gc, "_vnet", "ProgramData", "Riot Games")
	if checkRiotPaths(src, gc, dst) != nil {
		return nil
	}
	st, err := os.Lstat(filepath.Join(src, "RiotClientInstalls.json"))
	if err != nil || !st.Mode().IsRegular() || st.Size() > maxSnapshotBytes {
		return nil
	}
	index, err := readEntry(src, riotEntry{rel: "RiotClientInstalls.json", size: st.Size()})
	if err == nil {
		err = checkRcDefault(index, gc, dst)
	}
	if errors.Is(err, ErrRiotElsewhere) {
		return err
	}
	return nil
}

// checkRiotPaths is step 1 of CaptureRiot: never read through a link, never write through one. A
// junction at _vnet pointing back at C:\ProgramData would make the delete pass remove Riot's live
// files.
func checkRiotPaths(src, gc, dst string) error {
	st, err := os.Lstat(src)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("không thấy %s: mở Riot Client trên master một lần để nó ghi trạng thái cài", src)
	case err != nil:
		return fmt.Errorf("đọc %s: %v", src, err)
	case st.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0:
		return fmt.Errorf("%s là liên kết (junction?): bỏ liên kết, VNET tự chụp thư mục này", src)
	case !st.IsDir():
		return fmt.Errorf("%s không phải thư mục", src)
	}
	if err := plainDirChain(gc, dst); err != nil {
		return err
	}
	if under(src, gc) || under(gc, src) {
		return fmt.Errorf("%s trùng hoặc lồng với thư mục game %s", src, gc)
	}
	return nil
}

// checkRcDefault: Riot must run from the game folder gc (dst is its snapshot). An empty or missing
// rc_default is not checked, so a Riot format change cannot freeze publishing.
func checkRcDefault(index []byte, gc, dst string) error {
	var installsJSON struct {
		RcDefault string `json:"rc_default"`
	}
	if err := json.Unmarshal(index, &installsJSON); err != nil {
		// Riot sometimes corrupts this file; shipping a corrupt index is worse than waiting.
		return fmt.Errorf("RiotClientInstalls.json không đọc được (%v): mở rồi thoát hẳn Riot để nó ghi lại", err)
	}
	if rc := installsJSON.RcDefault; rc != "" && !under(rc, gc) {
		if _, err := os.Lstat(dst); errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: rc_default = %s", ErrRiotElsewhere, rc)
		}
		return fmt.Errorf("RiotClientInstalls.json trỏ Riot Client ở %s, không nằm trong %s: cài lại Riot vào thư mục game (research 3.1)", rc, gc)
	}
	return nil
}

// walkRiotSource lists the regular files of src that belong in the snapshot. It refuses while
// Riot is in the middle of a SQLite write (riotBusy), past the client's caps, and on a link or
// other odd entry it does not skip: links are never followed.
func walkRiotSource(src string) ([]riotEntry, error) {
	var out []riotEntry
	var total int64
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("đọc %s: %v", p, walkErr)
		}
		rel, err := filepath.Rel(src, p)
		if err != nil || rel == "." {
			return err
		}
		var fi fs.FileInfo
		if d.Type().IsRegular() {
			// os.Lstat, not d.Info(): on Windows d.Info() is the folder listing, whose size and mtime
			// NTFS updates only when the writer closes the file. A journal Riot still has open would
			// read as empty, and a database rewritten in place as unchanged.
			if fi, err = os.Lstat(p); err != nil {
				return fmt.Errorf("đọc %s: %v", p, err)
			}
			// Before the skip list: the journal itself is never copied, but the database next to it is.
			if riotBusy(p, fi) {
				return fmt.Errorf("Riot đang ghi %s (journal SQLite dở): mở rồi thoát hẳn Riot (khay > Exit), rồi publish lại", p) //nolint:staticcheck // ST1005: "Riot" is a name
			}
		}
		// A link (or junction) may lead to a folder: one named like a skipped folder is skipped too.
		other := fi == nil && !d.IsDir()
		if snapshotSkipped(rel, d.IsDir()) || other && snapshotSkipped(rel, true) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if other {
			// WalkDir does not go into it: the delete pass would empty the snapshot behind it, and that
			// snapshot would be published.
			return fmt.Errorf("%s là liên kết (junction?) hoặc tệp lạ trong nguồn: đặt thư mục/tệp thật rồi publish lại", p)
		}
		if fi == nil {
			return nil // a folder to walk into
		}
		total += fi.Size()
		if len(out) >= maxSnapshotFiles || total > maxSnapshotBytes {
			return errSnapshotTooBig
		}
		out = append(out, riotEntry{rel, fi.Size(), fi.ModTime().UnixNano()})
		return nil
	})
	return out, err
}

// riotBusy: a rollback journal that still has to be replayed, or a WAL with pages not yet
// checkpointed into its database. Copying the database now would ship a state Riot has not settled
// on. An empty journal, one with a zeroed header (TRUNCATE / PERSIST mode) or an empty WAL is done.
func riotBusy(p string, fi fs.FileInfo) bool {
	n := strings.ToLower(fi.Name())
	switch {
	case strings.HasSuffix(n, "-wal"):
		return fi.Size() > 0
	case strings.HasSuffix(n, "-journal") && fi.Size() >= int64(len(sqliteJournalHeader)):
		f, err := os.Open(p)
		if err != nil {
			return true
		}
		head := make([]byte, len(sqliteJournalHeader))
		_, err = io.ReadFull(f, head)
		_ = f.Close()
		return err != nil || bytes.Equal(head, sqliteJournalHeader)
	}
	return false
}

// snapshotSkipped: what never goes into the snapshot, at any depth. Vanguard and machine.cfg belong
// to one machine, logs and Riot Client's own update cache are noise, the rest are runtime locks and
// SQLite side files. The client skips the same list (riotMetaSkipped).
func snapshotSkipped(rel string, isDir bool) bool {
	l := strings.ToLower(filepath.ToSlash(rel))
	n := path.Base(l)
	if isDir {
		return n == "vanguard" || n == "logs" || l == "metadata/riotclientservices/update"
	}
	return n == "machine.cfg" || n == "lockfile" || strings.HasSuffix(n, ".lockfile") ||
		strings.HasSuffix(n, "-journal") || strings.HasSuffix(n, "-wal") ||
		strings.HasSuffix(n, "-shm") || strings.HasSuffix(n, ".vnet-tmp")
}

// readEntry reads one source file whole, provided it still has the size the walk saw: a file that
// grew is Riot writing, and reading on would also slip past the size cap.
func readEntry(src string, e riotEntry) ([]byte, error) {
	p := filepath.Join(src, e.rel)
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("đọc %s: %v", p, err)
	}
	data, err := io.ReadAll(io.LimitReader(f, e.size+1))
	_ = f.Close()
	if err != nil {
		return nil, fmt.Errorf("đọc %s: %v", p, err)
	}
	if int64(len(data)) != e.size {
		return nil, errRiotWrote(p)
	}
	return data, nil
}

// errRiotWrote: Riot changed a source file while it was being captured.
func errRiotWrote(p string) error {
	return fmt.Errorf("Riot vừa ghi %s trong lúc chụp: thoát hẳn Riot rồi publish lại", p) //nolint:staticcheck // ST1005: "Riot" is a name
}

// checkRiotFile refuses a file that shows Riot is not fully installed inside gameDir, or that
// still names gameDir in a form the client's rewrite cannot fix. p is only for the message.
func checkRiotFile(p, rel string, data []byte, gameDir string) error {
	if strings.HasSuffix(strings.ToLower(rel), ".product_settings.yaml") {
		// Every product must live inside the game folder too, or workstations look for it where the
		// cafe never put it.
		for _, m := range installRootLine.FindAllSubmatch(data, -1) {
			v := string(m[2])
			if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
				v = v[1 : len(v)-1]
			}
			if v != "" && !under(v, gameDir) {
				return fmt.Errorf("%s: %s = %s nằm ngoài %s: cài sản phẩm này vào thư mục game hoặc gỡ nó trên master", p, m[1], v, gameDir)
			}
		}
		if shouldRepairLine.Match(data) {
			return fmt.Errorf("%s có should_repair: true: mở Riot, vá tới khi mọi game hiện Play, thoát hẳn rồi publish lại", p)
		}
	}
	if residualMasterPath(rel, data, strings.ReplaceAll(gameDir, "/", `\`)) {
		return fmt.Errorf(`%s vẫn chứa đường dẫn master %s ở dạng máy trạm không sửa được (tệp nhị phân, \/, lẫn dấu, UTF-16 hoặc tệp chữ >1 MB): báo chủ dự án (research M-2)`, p, gameDir)
	}
	return nil
}

// sameFile reports whether target is a regular file holding exactly data.
func sameFile(target string, data []byte) bool {
	st, err := os.Lstat(target)
	if err != nil || !st.Mode().IsRegular() || st.Size() != int64(len(data)) {
		return false
	}
	cur, err := os.ReadFile(target)
	return err == nil && bytes.Equal(cur, data)
}

// writeSnapshotFile replaces target with data through a temp file in the same folder, so a reader
// never sees half a file. Folders from gameDir down are created one level at a time and must be
// real folders (mkdirPlain). The mtime is whatever the write sets: it moves only with the bytes.
func writeSnapshotFile(gameDir, target string, data []byte) error {
	if err := mkdirPlain(gameDir, filepath.Dir(target)); err != nil {
		return err
	}
	tmp := target + ".vnet-tmp"
	// Remove an old temp file (a link: the link itself), then create with O_EXCL: never reopen and
	// truncate a predictable name.
	_ = os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Close()
	} else {
		_ = f.Close()
	}
	if err == nil {
		// A robocopy-made file can be read-only, and Windows will not rename over one.
		if st, serr := os.Lstat(target); serr == nil && st.Mode().IsRegular() {
			_ = os.Chmod(target, 0o644)
		}
		err = os.Rename(tmp, target)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

// removeEmptyDirs removes dir and every folder under it, deepest first, with os.Remove only (never
// RemoveAll): a folder that still holds anything stays, as an error. It returns how many it removed.
func removeEmptyDirs(dir string) (int, error) {
	var dirs []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr == nil && d.IsDir() {
			dirs = append(dirs, p)
		}
		return walkErr
	})
	n := 0
	for i := len(dirs) - 1; err == nil && i >= 0; i-- {
		if err = os.Remove(dirs[i]); err == nil {
			n++
		}
	}
	return n, err
}

// firstChange names the first entry that differs between two walks of the source, "" if none does.
func firstChange(a, b []riotEntry) string {
	for i := 0; i < len(a) || i < len(b); i++ {
		switch {
		case i >= len(a):
			return b[i].rel
		case i >= len(b) || a[i] != b[i]:
			return a[i].rel
		}
	}
	return ""
}

// plainDirChain checks that root and every folder from root down to dir is a real folder: a
// symlink, junction or other reparse point (Go reports ModeSymlink/ModeIrregular) is refused.
// Levels that do not exist yet are fine. Ported from the client (client/src/riotmeta.go).
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

// dirParts: the levels from root down to dir, starting with "" (root itself).
func dirParts(root, dir string) ([]string, error) {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("đích nằm ngoài thư mục game: " + dir)
	}
	parts := []string{""}
	if rel != "." {
		parts = append(parts, strings.Split(rel, string(filepath.Separator))...)
	}
	return parts, nil
}

// mkdirPlain creates the levels from root down to dir one at a time and checks each is a real
// folder before going further down. MkdirAll walks through junctions: it would create folders
// wherever one points before anything could check. Ported from the client.
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

// under reports whether value names g or something inside it, ignoring ASCII case and treating
// every separator form Riot writes alike.
func under(value, g string) bool {
	v, r := normRiotPath(value), normRiotPath(g)
	return v == r || strings.HasPrefix(v, r+"/")
}

func normRiotPath(p string) string {
	return strings.TrimRight(riotSeps.Replace(string(asciiLower([]byte(p)))), "/")
}

// residualMasterPath reports whether data would still name the master's game folder g (backslash
// form) after the client's rewrite: in a binary file, a text file too big to rewrite, or a form
// the rewrite does not know (\/, mixed or doubled separators, a letter or digit right before it,
// UTF-16). A workstation would then look for the game where the master keeps it, and ask for a Repair.
func residualMasterPath(rel string, data []byte, g string) bool {
	if riotTextFile(rel) && len(data) <= maxRewriteBytes {
		data = rewriteRiotPaths(data, g, "") // what the client rewrites, removed
	}
	low := asciiLower(data)
	slash := string(asciiLower([]byte(strings.ReplaceAll(g, `\`, "/"))))
	// Each run of separators as one "/": every form riotSeps maps, and doubled ones too.
	if bytes.Contains(sepRun.ReplaceAll(low, []byte("/")), sepRun.ReplaceAll([]byte(slash), []byte("/"))) {
		return true
	}
	return bytes.Contains(low, utf16le(slash)) || bytes.Contains(low, utf16le(strings.ReplaceAll(slash, "/", `\`)))
}

// rewriteRiotPaths replaces every occurrence of the master's game folder (from, backslash form) with
// the folder workstations see (to), keeping each occurrence's separator style: "D:/A/B", `D:\A\B`,
// JSON's `D:\\A\\B`. ASCII letters match in either case. An occurrence must start the path: the
// byte before it is not a letter or digit. There is no end boundary: "<from> 2\x" is a sibling
// folder in the master's GAME_ROOT and the same sibling in the workstation's client_root, so
// rewriting the prefix is right.
//
// Keep this function byte-for-byte the same as rewriteRiotPaths in client/src/riotmeta.go: the
// master refuses what the client would not rewrite (residualMasterPath).
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
		if last > 0 {
			out = append(b, out[last:]...)
		}
	}
	return out
}

// riotTextFile: the files the client rewrites paths in.
func riotTextFile(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".json", ".yaml", ".yml":
		return true
	}
	return false
}

// asciiLower returns a copy of b with only A-Z lowered, so byte offsets never move.
func asciiLower(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return out
}

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

// utf16le encodes s the way Windows stores text in UTF-16 files.
func utf16le(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := make([]byte, 0, 2*len(u))
	for _, c := range u {
		b = append(b, byte(c), byte(c>>8))
	}
	return b
}
