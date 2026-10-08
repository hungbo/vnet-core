package gameupdate

import (
	"os"
	"path/filepath"
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

func TestFingerprint_MissingDir(t *testing.T) {
	_, err := Fingerprint(filepath.Join(t.TempDir(), "nope"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}
