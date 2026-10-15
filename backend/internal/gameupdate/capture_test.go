package gameupdate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lolSettings = "Metadata/league_of_legends.live/x.product_settings.yaml"

// riotMaster lays out a master the way CaptureRiot expects it: gameDir holds the Riot Client and
// src is its %ProgramData%\Riot Games, whose every path points inside gameDir.
func riotMaster(t *testing.T) (src, gameDir string) {
	t.Helper()
	root := t.TempDir()
	gameDir = filepath.Join(root, "GameMaster", "Riot Games")
	src = filepath.Join(root, "ProgramData", "Riot Games")
	gs := filepath.ToSlash(gameDir)
	writeFile(t, filepath.Join(gameDir, "Riot Client", "RiotClientServices.exe"), []byte("exe"))
	writeSrc(t, src, "RiotClientInstalls.json", `{"rc_default": "`+gs+`/Riot Client/RiotClientServices.exe"}`)
	writeSrc(t, src, lolSettings, "product_install_full_path: \""+gs+"/League of Legends\"\nproduct_install_root: \""+gs+"/\"\nshould_repair: false\n")
	writeSrc(t, src, "Metadata/league_of_legends.live/x.manifest", "manifest")
	writeSrc(t, src, "Metadata/league_of_legends.live/x.db", "SQLite format 3\x00")
	return src, gameDir
}

func writeSrc(t *testing.T, dir, rel, data string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, filepath.FromSlash(rel)), []byte(data))
}

func snapshotDir(gameDir string) string {
	return filepath.Join(gameDir, "_vnet", "ProgramData", "Riot Games")
}

type fileState struct {
	data  string
	mtime int64
}

// treeState reads every file under dir with its mtime; a missing dir reads as empty.
func treeState(t *testing.T, dir string) map[string]fileState {
	t.Helper()
	out := map[string]fileState{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = fileState{string(b), fi.ModTime().UnixNano()}
		return nil
	})
	if !errors.Is(err, fs.ErrNotExist) {
		require.NoError(t, err)
	}
	return out
}

func TestIsRiotGame(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, IsRiotGame(dir), "no Riot Client")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "Riot Client", "RiotClientServices.exe"), 0o755))
	assert.False(t, IsRiotGame(dir), "a folder named like the exe")

	other := t.TempDir()
	writeFile(t, filepath.Join(other, "Riot Client", "RiotClientServices.exe"), []byte("exe"))
	assert.True(t, IsRiotGame(other))
}

func TestCaptureRiot_CopiesAndSkips(t *testing.T) {
	src, gameDir := riotMaster(t)
	for _, rel := range []string{"machine.cfg", "lockfile", "Metadata/vanguard/a.exe", "Metadata/vanguard.lockfile",
		"Metadata/x.db-shm", "Logs/a.log", "Metadata/RiotClientServices/Update/b.bin"} {
		writeSrc(t, src, rel, "per-machine or runtime")
	}
	writeSrc(t, src, "Metadata/y.db-journal", "")

	n, err := CaptureRiot(src, gameDir)
	require.NoError(t, err)
	assert.Equal(t, 5, n, "4 files and the record")

	got := treeState(t, snapshotDir(gameDir))
	kept := []string{"RiotClientInstalls.json", lolSettings,
		"Metadata/league_of_legends.live/x.manifest", "Metadata/league_of_legends.live/x.db"}
	require.Len(t, got, len(kept))
	for _, rel := range kept {
		b, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(rel)))
		require.NoError(t, err)
		assert.Equal(t, string(b), got[rel].data, rel)
	}
	rec, err := os.ReadFile(filepath.Join(gameDir, "_vnet", MasterDirRecord))
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(gameDir), string(rec), "no BOM, no newline")
}

func TestCaptureRiot_NoChurn(t *testing.T) {
	src, gameDir := riotMaster(t)
	_, err := CaptureRiot(src, gameDir)
	require.NoError(t, err)
	vnet := filepath.Join(gameDir, "_vnet")
	before := treeState(t, vnet)
	fp, err := Fingerprint(gameDir)
	require.NoError(t, err)

	n, err := CaptureRiot(src, gameDir)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Equal(t, before, treeState(t, vnet), "nothing rewritten, no mtime moved")
	again, err := Fingerprint(gameDir)
	require.NoError(t, err)
	assert.Equal(t, fp, again, "an unchanged ProgramData must not become a new version")
}

func TestCaptureRiot_OnlyChangedFileWritten(t *testing.T) {
	src, gameDir := riotMaster(t)
	_, err := CaptureRiot(src, gameDir)
	require.NoError(t, err)
	// Age the whole snapshot so a rewrite shows in its mtime.
	vnet := filepath.Join(gameDir, "_vnet")
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	for rel := range treeState(t, vnet) {
		require.NoError(t, os.Chtimes(filepath.Join(vnet, filepath.FromSlash(rel)), old, old))
	}
	writeSrc(t, src, lolSettings, "product_install_root: \""+filepath.ToSlash(gameDir)+"/\"\nlocale: vi_VN\n")

	n, err := CaptureRiot(src, gameDir)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	for rel, st := range treeState(t, vnet) {
		assert.Equal(t, rel == "ProgramData/Riot Games/"+lolSettings, st.mtime != old.UnixNano(), rel)
	}
}

func TestCaptureRiot_DeletesStaleAndSkipped(t *testing.T) {
	src, gameDir := riotMaster(t)
	dst := snapshotDir(gameDir)
	stale := []string{"machine.cfg", "Metadata/vanguard/x", "old.yaml", "y.json.vnet-tmp"}
	for _, rel := range stale {
		writeSrc(t, dst, rel, "stale")
	}
	roblox := filepath.Join(gameDir, "_vnet", "ProgramData", "Roblox", "z")
	writeFile(t, roblox, []byte("another launcher"))

	n, err := CaptureRiot(src, gameDir)
	require.NoError(t, err)
	assert.Equal(t, 4+len(stale)+1, n, "written, deleted, the record")
	got := treeState(t, dst)
	assert.Len(t, got, 4)
	for _, rel := range stale {
		assert.NotContains(t, got, rel)
	}
	b, err := os.ReadFile(roblox)
	require.NoError(t, err)
	assert.Equal(t, "another launcher", string(b), "only the Riot snapshot is reconciled")
}

func assertBusy(t *testing.T, rel, data string) {
	t.Helper()
	src, gameDir := riotMaster(t)
	writeSrc(t, src, rel, data)
	n, err := CaptureRiot(src, gameDir)
	require.Error(t, err)
	assert.Zero(t, n)
	assert.Contains(t, err.Error(), "Riot đang ghi")
	assert.Contains(t, err.Error(), path.Base(rel))
	assert.NoDirExists(t, filepath.Join(gameDir, "_vnet"), "no snapshot, no record")
}

func TestCaptureRiot_HotJournal(t *testing.T) {
	assertBusy(t, "Metadata/a/a.db-journal", string(sqliteJournalHeader)+"\x00\x00\x00\x01")
}

func TestCaptureRiot_NonEmptyWAL(t *testing.T) {
	assertBusy(t, "Metadata/a/a.db-wal", "0123456789")
}

func TestCaptureRiot_SettledJournalAndWALAreNotBusy(t *testing.T) {
	src, gameDir := riotMaster(t)
	writeSrc(t, src, "Metadata/a/a.db-journal", "")
	writeSrc(t, src, "Metadata/b/b.db-journal", strings.Repeat("\x00", 8)) // TRUNCATE / PERSIST mode
	writeSrc(t, src, "Metadata/a/a.db-wal", "")

	_, err := CaptureRiot(src, gameDir)
	require.NoError(t, err)
	got := treeState(t, snapshotDir(gameDir))
	assert.Len(t, got, 4)
	assert.NotContains(t, got, "Metadata/a/a.db-journal")
	assert.NotContains(t, got, "Metadata/b/b.db-journal")
	assert.NotContains(t, got, "Metadata/a/a.db-wal")
}

// captured runs a first, successful capture and returns the state of _vnet it left.
func captured(t *testing.T, src, gameDir string) map[string]fileState {
	t.Helper()
	_, err := CaptureRiot(src, gameDir)
	require.NoError(t, err)
	return treeState(t, filepath.Join(gameDir, "_vnet"))
}

func TestCaptureRiot_MissingSourceKeepsSnapshot(t *testing.T) {
	src, gameDir := riotMaster(t)
	before := captured(t, src, gameDir)
	require.NoError(t, os.RemoveAll(src))

	n, err := CaptureRiot(src, gameDir)
	require.Error(t, err)
	assert.Zero(t, n)
	assert.Contains(t, err.Error(), "không thấy")
	assert.Equal(t, before, treeState(t, filepath.Join(gameDir, "_vnet")))
}

func TestCaptureRiot_NoInstallsJSON(t *testing.T) {
	src, gameDir := riotMaster(t)
	before := captured(t, src, gameDir)
	require.NoError(t, os.Remove(filepath.Join(src, "RiotClientInstalls.json")))

	_, err := CaptureRiot(src, gameDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "thiếu RiotClientInstalls.json")
	assert.Equal(t, before, treeState(t, filepath.Join(gameDir, "_vnet")), "a half-present source never wipes the snapshot")
}

func TestCaptureRiot_RcDefaultOutside(t *testing.T) {
	const outside = `{"rc_default": "E:/GameMaster/Riot Games/Riot Client/RiotClientServices.exe"}`

	t.Run("snapshot exists", func(t *testing.T) {
		src, gameDir := riotMaster(t)
		before := captured(t, src, gameDir)
		writeSrc(t, src, "RiotClientInstalls.json", outside)
		assert.NoError(t, RiotElsewhere(src, gameDir), "the refusal below is CaptureRiot's to give")
		_, err := CaptureRiot(src, gameDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "E:/GameMaster/Riot Games/Riot Client")
		assert.Contains(t, err.Error(), filepath.Clean(gameDir))
		assert.Equal(t, before, treeState(t, filepath.Join(gameDir, "_vnet")))
	})
	t.Run("no snapshot yet: not the folder Riot runs from", func(t *testing.T) {
		src, gameDir := riotMaster(t)
		writeSrc(t, src, "RiotClientInstalls.json", outside)
		n, err := CaptureRiot(src, gameDir)
		assert.ErrorIs(t, err, ErrRiotElsewhere)
		assert.Zero(t, n)
		assert.NoDirExists(t, filepath.Join(gameDir, "_vnet"))
	})
	// The folder is a plain game then: the state of the install Riot does run from must not refuse it.
	t.Run("no snapshot yet, source busy", func(t *testing.T) {
		src, gameDir := riotMaster(t)
		writeSrc(t, src, "RiotClientInstalls.json", outside)
		writeSrc(t, src, "Metadata/a/a.db-journal", string(sqliteJournalHeader)+"\x00\x00\x00\x01")
		assert.ErrorIs(t, RiotElsewhere(src, gameDir), ErrRiotElsewhere)
		n, err := CaptureRiot(src, gameDir)
		assert.ErrorIs(t, err, ErrRiotElsewhere)
		assert.Zero(t, n)
		assert.NoDirExists(t, filepath.Join(gameDir, "_vnet"))
	})
	t.Run("other spellings of the game folder", func(t *testing.T) {
		src, gameDir := riotMaster(t)
		gs := filepath.ToSlash(gameDir)
		for _, rc := range []string{
			strings.ToUpper(gs) + "/Riot Client/RiotClientServices.exe",
			strings.ReplaceAll(gs, "/", `\\`) + `\\Riot Client\\RiotClientServices.exe`, // JSON-escaped backslashes
		} {
			writeSrc(t, src, "RiotClientInstalls.json", `{"rc_default": "`+rc+`"}`)
			_, err := CaptureRiot(src, gameDir)
			assert.NoError(t, err, rc)
		}
	})
	t.Run("corrupt", func(t *testing.T) {
		src, gameDir := riotMaster(t)
		writeSrc(t, src, "RiotClientInstalls.json", `{"rc_default": `)
		assert.NoError(t, RiotElsewhere(src, gameDir), "cannot tell: CaptureRiot reports it")
		_, err := CaptureRiot(src, gameDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "không đọc được")
		assert.NoDirExists(t, filepath.Join(gameDir, "_vnet"))
	})
}

func TestCaptureRiot_ProductRootOutsideAndShouldRepair(t *testing.T) {
	for _, c := range []struct {
		yaml, wantErr string
	}{
		{"product_install_root: \"C:/Riot Games\"\n", "nằm ngoài"},
		{"product_install_full_path: 'C:/Riot Games/VALORANT/live'\r\n", "nằm ngoài"},
		{"should_repair: true\n", "should_repair: true"},
		{"should_repair: false\n", ""},
		// An empty value never takes the next line as its value.
		{"product_install_root:\nsettings:\n  a: 1\n", ""},
		{"product_install_full_path:\r\nlocale: vi_VN\r\n", ""},
		{"should_repair:\ntrue: x\n", ""},
	} {
		src, gameDir := riotMaster(t)
		writeSrc(t, src, "Metadata/valorant.live/valorant.live.product_settings.yaml", c.yaml)
		_, err := CaptureRiot(src, gameDir)
		if c.wantErr == "" {
			assert.NoError(t, err, c.yaml)
			continue
		}
		if assert.Error(t, err, c.yaml) {
			assert.Contains(t, err.Error(), c.wantErr)
		}
		assert.NoDirExists(t, filepath.Join(gameDir, "_vnet"))
	}
}

func TestCaptureRiot_ResidualMasterPath(t *testing.T) {
	for _, c := range []struct {
		name, rel string
		data      func(gs string) string // gs: the game folder with forward slashes
		ok        bool
	}{
		{"binary file", "Metadata/x/x.db", func(gs string) string { return "SQLite\x00" + gs + "\x00" }, false},
		{`\/ form`, "Metadata/x/a.yaml", func(gs string) string { return `p: "` + strings.ReplaceAll(gs, "/", `\/`) + `"` }, false},
		{"mixed separators", "Metadata/x/a.yaml", func(gs string) string { return "p: " + path.Dir(gs) + `\` + path.Base(gs) }, false},
		{"UTF-16LE", "Metadata/x/x.bin", func(gs string) string { return string(utf16le(gs)) }, false},
		{"text too big to rewrite", "Metadata/x/big.yaml", func(gs string) string {
			return strings.Repeat(" ", maxRewriteBytes) + "\np: " + gs + "\n"
		}, false},
		{"letter right before", "Metadata/x/a.yaml", func(gs string) string { return "p: X" + gs }, false},
		{"doubled slashes", "Metadata/x/d.yaml", func(gs string) string { return "p: " + strings.ToUpper(strings.ReplaceAll(gs, "/", "//")) + "//x\n" }, false},
		{"JSON double-escaped", "Metadata/x/d.json", func(gs string) string { return `{"p": "` + strings.ReplaceAll(gs, "/", `\\\\`) + `"}` }, false},
		{"forms 1-3 in json", "Metadata/x/p.json", func(gs string) string {
			return `{"a": "` + gs + `/x", "b": "` + strings.ReplaceAll(gs, "/", `\\`) + `\\x"}`
		}, true},
		{"form 2 in yaml", "Metadata/x/q.yaml", func(gs string) string { return "p: '" + strings.ReplaceAll(gs, "/", `\`) + `\x'` }, true},
		{"sibling folder", "Metadata/x/s.yaml", func(gs string) string { return "p: " + gs + " 2/x\n" }, true},
		{"file starts with the game folder", "Metadata/x/b.yml", func(gs string) string { return gs + "/League of Legends\n" }, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			src, gameDir := riotMaster(t)
			writeSrc(t, src, c.rel, c.data(filepath.ToSlash(gameDir)))
			_, err := CaptureRiot(src, gameDir)
			if c.ok {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "vẫn chứa đường dẫn master")
			assert.Contains(t, err.Error(), path.Base(c.rel))
			assert.NoDirExists(t, filepath.Join(gameDir, "_vnet"))
		})
	}
}

func TestCaptureRiot_TooBig(t *testing.T) {
	src, gameDir := riotMaster(t)
	for i := 0; i < maxSnapshotFiles-3; i++ { // the fixture has 4 files: 2001 in all
		writeFile(t, filepath.Join(src, "Metadata", "many", fmt.Sprintf("f%04d.bin", i)), []byte("x"))
	}
	_, err := CaptureRiot(src, gameDir)
	assert.ErrorIs(t, err, errSnapshotTooBig)
	assert.NoDirExists(t, filepath.Join(gameDir, "_vnet"))
}

func TestCaptureRiot_Links(t *testing.T) {
	symlink := func(t *testing.T, target, link string) {
		t.Helper()
		require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("cannot create a symlink here: %v", err)
		}
	}
	// Files the snapshot skips: a delete pass running inside src would remove them.
	withSkipped := func(t *testing.T) (src, gameDir string) {
		src, gameDir = riotMaster(t)
		writeSrc(t, src, "machine.cfg", "this machine")
		writeSrc(t, src, "Metadata/vanguard/vgc.exe", "vanguard")
		return src, gameDir
	}

	t.Run("linked source", func(t *testing.T) {
		src, gameDir := withSkipped(t)
		actual := src + " real"
		require.NoError(t, os.Rename(src, actual))
		symlink(t, actual, src)
		before := treeState(t, actual)
		_, err := CaptureRiot(src, gameDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "liên kết")
		assert.NoDirExists(t, filepath.Join(gameDir, "_vnet"))
		assert.Equal(t, before, treeState(t, actual))
	})
	// A link between the game folder and the snapshot that makes the snapshot the live source.
	for _, c := range []struct {
		link string
		up   int // how many levels above src the link points
	}{{"_vnet/ProgramData", 1}, {"_vnet", 2}} {
		t.Run(c.link+" linked back", func(t *testing.T) {
			src, gameDir := withSkipped(t)
			target := src
			for i := 0; i < c.up; i++ {
				target = filepath.Dir(target)
			}
			symlink(t, target, filepath.Join(gameDir, filepath.FromSlash(c.link)))
			before := treeState(t, src)
			_, err := CaptureRiot(src, gameDir)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "không phải thư mục thật")
			assert.Equal(t, before, treeState(t, src), "every source file still exists, unchanged")
		})
	}
	// A link below the snapshot: reading through it, every file behind it looks already in place.
	t.Run("snapshot folder linked back to the source", func(t *testing.T) {
		src, gameDir := withSkipped(t)
		symlink(t, filepath.Join(src, "Metadata"), filepath.Join(snapshotDir(gameDir), "Metadata"))
		before := treeState(t, src)
		_, err := CaptureRiot(src, gameDir)
		require.Error(t, err, "this pass must not be fingerprinted")
		assert.Contains(t, err.Error(), "đã gỡ liên kết")
		assert.Equal(t, before, treeState(t, src), "every source file still exists, unchanged")

		_, err = CaptureRiot(src, gameDir)
		require.NoError(t, err)
		assert.Len(t, treeState(t, snapshotDir(gameDir)), 4, "the next pass writes the real files")
	})
	// WalkDir does not go into a link inside the source: without a refusal the delete pass would
	// empty the snapshot behind it, and that snapshot would be published.
	t.Run("linked folder inside the source", func(t *testing.T) {
		src, gameDir := riotMaster(t)
		before := captured(t, src, gameDir)
		meta := filepath.Join(src, "Metadata")
		require.NoError(t, os.Rename(meta, src+" Metadata"))
		symlink(t, src+" Metadata", meta)
		n, err := CaptureRiot(src, gameDir)
		require.Error(t, err)
		assert.Zero(t, n)
		assert.Contains(t, err.Error(), "liên kết")
		assert.Contains(t, err.Error(), "Metadata")
		assert.Equal(t, before, treeState(t, filepath.Join(gameDir, "_vnet")))
	})
	t.Run("linked folder the snapshot skips anyway", func(t *testing.T) {
		src, gameDir := riotMaster(t)
		logs := filepath.Join(t.TempDir(), "Logs")
		writeSrc(t, logs, "a.log", "log")
		symlink(t, logs, filepath.Join(src, "Logs"))
		_, err := CaptureRiot(src, gameDir)
		require.NoError(t, err)
		assert.Len(t, treeState(t, snapshotDir(gameDir)), 4)
	})
	t.Run("source inside the game folder", func(t *testing.T) {
		_, gameDir := riotMaster(t)
		inner := filepath.Join(gameDir, "ProgramData", "Riot Games")
		writeSrc(t, inner, "RiotClientInstalls.json", "{}")
		_, err := CaptureRiot(inner, gameDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "trùng hoặc lồng")
	})
	// The source walk would take in the snapshot itself, one level deeper on every capture.
	t.Run("game folder inside the source", func(t *testing.T) {
		src, _ := riotMaster(t)
		inner := filepath.Join(src, "Games", "Riot Games")
		writeFile(t, filepath.Join(inner, "Riot Client", "RiotClientServices.exe"), []byte("exe"))
		_, err := CaptureRiot(src, inner)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "trùng hoặc lồng")
		assert.NoDirExists(t, filepath.Join(inner, "_vnet"))
	})
}

// A stale file where the source now has a folder (an old hand-made snapshot, or Riot turning a file
// into a folder of the same name) must not stop every capture after it.
func TestCaptureRiot_FileWhereSourceHasFolder(t *testing.T) {
	src, gameDir := riotMaster(t)
	writeSrc(t, snapshotDir(gameDir), "Metadata", "stale")
	_, err := CaptureRiot(src, gameDir)
	require.NoError(t, err)
	assert.Len(t, treeState(t, snapshotDir(gameDir)), 4)
}

// The other way round: a stale folder where the source now has a file. The delete pass empties it;
// it must not stay in the way of the rename on every capture after.
func TestCaptureRiot_FolderWhereSourceHasFile(t *testing.T) {
	src, gameDir := riotMaster(t)
	dst := snapshotDir(gameDir)
	writeSrc(t, dst, "Metadata/league_of_legends.live/x.manifest/old", "stale")
	writeSrc(t, dst, "Metadata/league_of_legends.live/x.manifest/sub/old", "stale")

	n, err := CaptureRiot(src, gameDir)
	require.NoError(t, err)
	assert.Equal(t, 4+2+2+1, n, "written, 2 files and 2 folders deleted, the record")
	got := treeState(t, dst)
	assert.Len(t, got, 4)
	assert.Equal(t, "manifest", got["Metadata/league_of_legends.live/x.manifest"].data)

	n, err = CaptureRiot(src, gameDir)
	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestCaptureRiot_SourceChangedDuringCapture(t *testing.T) {
	src, gameDir := riotMaster(t)
	before := captured(t, src, gameDir)
	yaml := filepath.Join(src, filepath.FromSlash(lolSettings))
	writeFile(t, yaml, []byte("locale: vi_VN\n"))
	beforeRecheck = func() {
		// Same size, so only the mtime tells.
		writeFile(t, yaml, []byte("locale: en_US\n"))
		later := time.Now().Add(time.Hour)
		require.NoError(t, os.Chtimes(yaml, later, later))
	}
	t.Cleanup(func() { beforeRecheck = func() {} })

	_, err := CaptureRiot(src, gameDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Riot vừa ghi")
	assert.Contains(t, err.Error(), "x.product_settings.yaml")
	assert.Equal(t, before, treeState(t, filepath.Join(gameDir, "_vnet")))
}

// Golden table: the same cases sit in client/src/riotmeta_test.go (TestRewriteRiotPaths); both
// sides must rewrite alike.
func TestRewriteRiotPaths(t *testing.T) {
	const from, to = `D:\GameMaster\Riot Games`, `E:\Games\Riot Games`
	cases := []struct{ from, to, in, want string }{
		{from, to,
			`"rc_default": "D:/GameMaster/Riot Games/Riot Client/RiotClientServices.exe"`,
			`"rc_default": "E:/Games/Riot Games/Riot Client/RiotClientServices.exe"`},
		{from, to,
			`"associated_client": {"D:/GameMaster/Riot Games/League of Legends/": "D:/GameMaster/Riot Games/Riot Client/RiotClientServices.exe"}`,
			`"associated_client": {"E:/Games/Riot Games/League of Legends/": "E:/Games/Riot Games/Riot Client/RiotClientServices.exe"}`},
		{from, to, `product_install_root: "d:/gamemaster/riot games/"`, `product_install_root: "E:/Games/Riot Games/"`},
		{from, to, "product_install_full_path: D:/GameMaster/Riot Games\nx: 1\n", "product_install_full_path: E:/Games/Riot Games\nx: 1\n"},
		{from, to, `product_install_root: 'D:\GameMaster\Riot Games\Riot Client'`, `product_install_root: 'E:\Games\Riot Games\Riot Client'`},
		{from, to, `{"p": "D:\\GameMaster\\Riot Games\\League of Legends\\"}`, `{"p": "E:\\Games\\Riot Games\\League of Legends\\"}`},
		{from, to, `x: D:/GameMaster/Riot Games`, `x: E:/Games/Riot Games`},
		{from, to, `[D:/GameMaster/Riot Games, 1]`, `[E:/Games/Riot Games, 1]`},
		{from, to, `{a: D:/GameMaster/Riot Games}`, `{a: E:/Games/Riot Games}`},
		{from, to, `D:/GameMaster/Riot Games 2/x`, `E:/Games/Riot Games 2/x`},
		{from, to, `XD:/GameMaster/Riot Games`, `XD:/GameMaster/Riot Games`},
		{from, to, `C:/GameMaster/Riot Games`, `C:/GameMaster/Riot Games`},
		{from, to, `D:/GameMaster/Other`, `D:/GameMaster/Other`},
		{from, to, `D:/GameMaster\Riot Games`, `D:/GameMaster\Riot Games`},
		{from, to, `D:\/GameMaster\/Riot Games`, `D:\/GameMaster\/Riot Games`},
		{from, to, "\xef\xbb\xbf{\"a\": \"D:/GameMaster/Riot Games\"}", "\xef\xbb\xbf{\"a\": \"E:/Games/Riot Games\"}"},
		{from, to, `{"a": 1}`, `{"a": 1}`},
		{from, `E:\GameDisk01\Riot Games`, `"D:/GameMaster/Riot Games/x"`, `"E:/GameDisk01/Riot Games/x"`},
		{from, "", "D:/GameMaster/Riot Games/League of Legends\n", "/League of Legends\n"}, // residualMasterPath's to==""
	}
	for _, c := range cases {
		assert.Equal(t, c.want, string(rewriteRiotPaths([]byte(c.in), c.from, c.to)), c.in)
	}
}
