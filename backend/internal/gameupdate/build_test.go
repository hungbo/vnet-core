package gameupdate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTorrent_SingleFileIsStillMultiFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Tool")
	writeFile(t, filepath.Join(dir, "only.exe"), []byte("hello"))
	mi, err := BuildTorrent(dir, "Tool")
	require.NoError(t, err)

	b, err := bencode.Marshal(mi)
	require.NoError(t, err)
	_, info, err := LoadTorrent(b)
	require.NoError(t, err)
	assert.Equal(t, "Tool", info.Name)
	require.Len(t, info.Files, 1)
	assert.Equal(t, []string{"only.exe"}, info.Files[0].Path)
}

func TestBuildTorrent_SameContentSameInfoHash(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "G")
	writeFile(t, filepath.Join(dir, "b", "x.dat"), []byte("1"))
	writeFile(t, filepath.Join(dir, "a.dat"), []byte("2"))
	a, err := BuildTorrent(dir, "G")
	require.NoError(t, err)
	b, err := BuildTorrent(dir, "G")
	require.NoError(t, err)
	assert.Equal(t, a.HashInfoBytes(), b.HashInfoBytes())
}

func TestBuildTorrent_RejectsEmptyAndBadNames(t *testing.T) {
	_, err := BuildTorrent(t.TempDir(), "Empty")
	assert.Error(t, err)
	_, err = BuildTorrent(t.TempDir(), "a/b")
	assert.Error(t, err)
}

func TestPublishExcluded(t *testing.T) {
	for _, c := range []struct {
		rel   string
		isDir bool
		want  bool
	}{
		{"lockfile", false, true},
		{"League of Legends/lockfile", false, true},
		{"Riot Client/x.LOCKFILE", false, true},
		{"League of Legends/Logs", true, true},
		{"league of legends/logs", true, true},
		{"lockfile.txt", false, false},
		{"a/lockfiles", false, false},
		{"Logs", true, false},
		{"Riot Client/Logs", true, false},
		{"League of Legends/Logs", false, false},
		{"x.db-journal", false, false},
		{"vanguard", true, false},
		{"machine.cfg", false, false},
	} {
		assert.Equal(t, c.want, publishExcluded(c.rel, c.isDir), "%s (dir=%v)", c.rel, c.isDir)
	}
}

func TestBuildTorrent_SkipsRuntimeFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Riot Games")
	for _, rel := range []string{"a.bin", "League of Legends/Game/data.wad", "League of Legends/lockfile",
		"League of Legends/Logs/x.log", "Riot Client/riot.lockfile"} {
		writeFile(t, filepath.Join(dir, filepath.FromSlash(rel)), []byte(rel))
	}
	mi, err := BuildTorrent(dir, "Riot Games")
	require.NoError(t, err)
	info, err := mi.UnmarshalInfo()
	require.NoError(t, err)
	var got []string
	for _, f := range info.Files {
		got = append(got, strings.Join(f.Path, "/"))
	}
	assert.Equal(t, []string{"League of Legends/Game/data.wad", "a.bin"}, got)
}

func TestPieceLengthFor(t *testing.T) {
	assert.EqualValues(t, 256<<10, pieceLengthFor(10<<20))
	assert.EqualValues(t, 4<<20, pieceLengthFor(80<<30))
	assert.EqualValues(t, 4<<20, pieceLengthFor(500<<30)) // capped
}

func TestStabilizer_WaitsForQuietPeriod(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a"), []byte("v1"))
	v1, err := Fingerprint(dir)
	require.NoError(t, err)

	s := NewStabilizer(10*time.Minute, v1)
	t0 := time.Now()
	assert.False(t, s.Observe(v1, t0), "unchanged folder is not a new version")

	writeFile(t, filepath.Join(dir, "a"), []byte("v2-longer"))
	v2, err := Fingerprint(dir)
	require.NoError(t, err)
	require.NotEqual(t, v1, v2)

	assert.False(t, s.Observe(v2, t0.Add(time.Minute)), "just changed")
	assert.False(t, s.Observe(v2, t0.Add(5*time.Minute)), "still inside the quiet period")
	assert.True(t, s.Observe(v2, t0.Add(11*time.Minute)), "settled")

	// The launcher touches another file mid-wait: the clock restarts.
	writeFile(t, filepath.Join(dir, "b"), []byte("v3"))
	v3, err := Fingerprint(dir)
	require.NoError(t, err)
	assert.False(t, s.Observe(v3, t0.Add(12*time.Minute)))
	assert.False(t, s.Observe(v3, t0.Add(20*time.Minute)))
	assert.True(t, s.Observe(v3, t0.Add(23*time.Minute)))

	s.MarkPublished(v3)
	assert.False(t, s.Observe(v3, t0.Add(60*time.Minute)))
}

func TestFingerprint_IgnoresRuntimeFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "League of Legends", "Game", "data.wad"), []byte("wad"))
	before, err := Fingerprint(dir)
	require.NoError(t, err)

	for _, rel := range []string{"League of Legends/lockfile", "League of Legends/Logs/x.log", "Riot Client/riot.lockfile"} {
		writeFile(t, filepath.Join(dir, filepath.FromSlash(rel)), []byte(rel))
	}
	after, err := Fingerprint(dir)
	require.NoError(t, err)
	assert.Equal(t, before, after, "playing on the master is not a new version")

	writeFile(t, filepath.Join(dir, "Logs", "x.log"), []byte("log"))
	changed, err := Fingerprint(dir)
	require.NoError(t, err)
	assert.NotEqual(t, before, changed, "only League of Legends/Logs is excluded")
}

// Fingerprints already stored for a game must stay valid: a change of the record format would
// republish every game at once.
func TestFingerprint_FormatUnchanged(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.bin")
	writeFile(t, p, []byte("hello"))
	mt := time.Unix(1700000000, 0)
	require.NoError(t, os.Chtimes(p, mt, mt))
	fp, err := Fingerprint(dir)
	require.NoError(t, err)
	// sha256("a.bin\x005\x001700000000000000000\n")
	assert.Equal(t, "dbf1457015ff4e6597c8c4d107fe523b9a51546517331ca8bb22df70d23c1581", fp)
}

// Seeding a published game re-creates its empty files (the torrent storage opens them with
// O_TRUNC), which gives them a new write time. That must not look like a new version: the master
// republished LibreOffice (657 empty files) every ~15 min, forever.
func TestFingerprint_EmptyFileTimeIgnored(t *testing.T) {
	src := filepath.Join(t.TempDir(), "Office")
	writeFile(t, filepath.Join(src, "program", "soffice.bin"), randomBytes(t, 1<<16))
	empty := filepath.Join(src, "share", "help", "en.done")
	writeFile(t, empty, nil)
	old := time.Unix(1700000000, 0)
	require.NoError(t, os.Chtimes(empty, old, old))
	before, err := Fingerprint(src)
	require.NoError(t, err)

	job := publish(t, src)
	job.Dest = src
	require.NoError(t, newTestEngine(t).Seed(context.Background(), job))
	after, err := Fingerprint(src)
	require.NoError(t, err)
	assert.Equal(t, before, after, "seeding is not a new version")

	now := time.Now()
	require.NoError(t, os.Chtimes(empty, now, now))
	touched, err := Fingerprint(src)
	require.NoError(t, err)
	assert.Equal(t, before, touched, "an empty file's write time does not count")

	require.NoError(t, os.WriteFile(empty, []byte("x"), 0o644))
	grown, err := Fingerprint(src)
	require.NoError(t, err)
	assert.NotEqual(t, before, grown, "an empty file getting content is a change")

	require.NoError(t, os.WriteFile(empty, nil, 0o644))
	writeFile(t, filepath.Join(src, "share", "help", "vi.done"), nil)
	added, err := Fingerprint(src)
	require.NoError(t, err)
	assert.NotEqual(t, before, added, "a new empty file is a change")
}

func TestFingerprint_MissingDir(t *testing.T) {
	_, err := Fingerprint(filepath.Join(t.TempDir(), "nope"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}
