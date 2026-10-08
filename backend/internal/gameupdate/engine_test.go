package gameupdate

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"iter"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"github.com/anacrolix/torrent/types/infohash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The master serves its game root as a BEP19 webseed; the test counts the bytes it hands out to
// prove an update only transfers what changed.
type countingSeed struct {
	root   string
	served atomic.Int64
}

func (s *countingSeed) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.FileServer(http.Dir(s.root)).ServeHTTP(countingWriter{w, &s.served}, r)
}

type countingWriter struct {
	http.ResponseWriter
	n *atomic.Int64
}

func (c countingWriter) Write(b []byte) (int, error) {
	n, err := c.ResponseWriter.Write(b)
	c.n.Add(int64(n))
	return n, err
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o644))
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := NewEngine(EngineConfig{DataDir: t.TempDir(), NoDHT: true})
	require.NoError(t, err)
	t.Cleanup(e.Close)
	return e
}

// assertSameTree fails unless both folders hold exactly the same files with the same bytes.
func assertSameTree(t *testing.T, want, got string) {
	t.Helper()
	read := func(dir string) map[string][]byte {
		out := map[string][]byte{}
		require.NoError(t, filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			require.NoError(t, err)
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(dir, p)
			b, err := os.ReadFile(p)
			require.NoError(t, err)
			out[filepath.ToSlash(rel)] = b
			return nil
		}))
		return out
	}
	w, g := read(want), read(got)
	require.Equal(t, len(w), len(g), "file count differs")
	for name, b := range w {
		require.Contains(t, g, name)
		assert.True(t, bytes.Equal(b, g[name]), "content of %s differs", name)
	}
}

func publish(t *testing.T, dir string) Job {
	t.Helper()
	mi, err := BuildTorrent(dir, filepath.Base(dir))
	require.NoError(t, err)
	return Job{Name: filepath.Base(dir), MetaInfo: mi}
}

func TestInstall_UpdatesInPlaceAndOnlyFetchesChanges(t *testing.T) {
	masterRoot := t.TempDir()
	src := filepath.Join(masterRoot, "Audition")
	writeFile(t, filepath.Join(src, "game.exe"), randomBytes(t, 3<<20))
	writeFile(t, filepath.Join(src, "data", "maps.pak"), randomBytes(t, 6<<20))
	writeFile(t, filepath.Join(src, "data", "old.pak"), randomBytes(t, 1<<20))
	writeFile(t, filepath.Join(src, "patch.txt"), randomBytes(t, 2<<20))

	seed := &countingSeed{root: masterRoot}
	srv := httptest.NewServer(seed)
	defer srv.Close()

	e := newTestEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	dest := filepath.Join(t.TempDir(), "Online Games", "Audition")
	// A save file the cafe's players wrote next to the game; it must survive cleanup.
	writeFile(t, filepath.Join(dest, "saves", "slot1.sav"), []byte("progress"))

	// 1. Fresh install downloads everything.
	job := publish(t, src)
	job.Dest, job.Webseeds, job.Keep = dest, []string{srv.URL + "/"}, []string{"saves"}
	require.NoError(t, e.Install(ctx, job))
	firstServed := seed.served.Load()
	assert.GreaterOrEqual(t, firstServed, int64(12<<20))
	require.NoError(t, os.RemoveAll(filepath.Join(dest, "saves")))
	assertSameTree(t, src, dest)
	writeFile(t, filepath.Join(dest, "saves", "slot1.sav"), []byte("progress"))

	// 2. The launcher patches the master copy: one file rewritten, one shrunk, one removed,
	//    one added. The big unchanged maps.pak must not be fetched again.
	writeFile(t, filepath.Join(src, "game.exe"), randomBytes(t, 3<<20))
	// patch.txt keeps its first bytes and loses its tail: every piece still verifies, nothing is
	// written to it, so only an explicit truncate removes the stale tail.
	require.NoError(t, os.Truncate(filepath.Join(src, "patch.txt"), 1<<20+123))
	require.NoError(t, os.Remove(filepath.Join(src, "data", "old.pak")))
	writeFile(t, filepath.Join(src, "data", "new.pak"), randomBytes(t, 512<<10))

	seed.served.Store(0)
	job2 := publish(t, src)
	job2.Dest, job2.Webseeds, job2.Keep = dest, job.Webseeds, job.Keep
	require.NoError(t, e.Install(ctx, job2))

	served := seed.served.Load()
	t.Logf("first install served %d bytes, update served %d bytes", firstServed, served)
	// Changed data is ~3.5 MB plus at most a piece of overlap on each side of every changed file.
	assert.Less(t, served, int64(6<<20), "update re-downloaded unchanged data")

	saved, err := os.ReadFile(filepath.Join(dest, "saves", "slot1.sav"))
	require.NoError(t, err, "kept file was deleted")
	assert.Equal(t, "progress", string(saved))
	require.NoError(t, os.RemoveAll(filepath.Join(dest, "saves")))
	assertSameTree(t, src, dest) // old.pak gone, patch.txt truncated, new.pak present

	p, ok := e.Progress("Audition")
	require.True(t, ok)
	assert.True(t, p.Complete)
	assert.Equal(t, p.Length, p.BytesCompleted)
}

func TestInstall_RepairsCorruptedFile(t *testing.T) {
	masterRoot := t.TempDir()
	src := filepath.Join(masterRoot, "LoL")
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, 4<<20))
	seed := &countingSeed{root: masterRoot}
	srv := httptest.NewServer(seed)
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "LoL")
	// Same size as the real file but wrong bytes: only hashing can tell.
	writeFile(t, filepath.Join(dest, "a.bin"), make([]byte, 4<<20))

	e := newTestEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	job := publish(t, src)
	job.Dest, job.Webseeds = dest, []string{srv.URL + "/"}
	require.NoError(t, e.Install(ctx, job))
	assertSameTree(t, src, dest)
}

func TestSeedAndInstall_PeerToPeer(t *testing.T) {
	// No webseed: the cafe must get every byte from the master over BitTorrent.
	src := filepath.Join(t.TempDir(), "CS2")
	writeFile(t, filepath.Join(src, "bin", "cs2.exe"), randomBytes(t, 2<<20))
	writeFile(t, filepath.Join(src, "maps", "dust2.vpk"), randomBytes(t, 5<<20))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	master := newTestEngine(t)
	job := publish(t, src)
	job.Dest = src
	require.NoError(t, master.Seed(ctx, job))

	cafe := newTestEngine(t)
	dest := filepath.Join(t.TempDir(), "CS2")
	job.Dest, job.Peers = dest, master.ListenAddrs()
	require.NoError(t, cafe.Install(ctx, job))
	assertSameTree(t, src, dest)
}

func TestSeed_RejectsDataThatDoesNotMatch(t *testing.T) {
	src := filepath.Join(t.TempDir(), "Game")
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, 1<<20))
	job := publish(t, src)
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, 1<<20)) // changed after publishing

	e := newTestEngine(t)
	job.Dest = src
	err := e.Seed(context.Background(), job)
	assert.ErrorContains(t, err, "do not match")
}

func TestInstall_RefusesDangerousDestinations(t *testing.T) {
	src := filepath.Join(t.TempDir(), "Game")
	writeFile(t, filepath.Join(src, "a.bin"), []byte("x"))
	job := publish(t, src)
	e := newTestEngine(t)
	for _, dest := range []string{"", "relative/Game", string(filepath.Separator)} {
		job.Dest = dest
		assert.Error(t, e.Install(context.Background(), job), "dest %q", dest)
	}
}

// A torrent comes from the network; a path that climbs out of the game folder must be refused
// before Install truncates, deletes or writes anything.
func TestInstall_RefusesUnsafeTorrentPaths(t *testing.T) {
	for _, bad := range [][]string{{"..", "evil.dll"}, {"a", "..", "..", "x"}, {"C:", "x"}, {`a\b`}, {""}, {"."}} {
		info := metainfo.Info{Name: "Game", PieceLength: 16384, Files: []metainfo.FileInfo{{Length: 1, Path: bad}}}
		require.NoError(t, info.GeneratePieces(func(metainfo.FileInfo) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader([]byte("x"))), nil
		}))
		infoBytes, err := bencode.Marshal(info)
		require.NoError(t, err)
		job := Job{Name: "Game", MetaInfo: &metainfo.MetaInfo{InfoBytes: infoBytes}, Dest: filepath.Join(t.TempDir(), "Game")}
		assert.ErrorContains(t, newTestEngine(t).Install(context.Background(), job), "unsafe file path", "path %q", bad)
	}
}

// slowCompletion makes the library take `delay` to record each passed piece (Set) and to read the
// state back when it checks whether a whole file is complete (GetRange). Both sit AFTER the moment
// VerifyDataContext returns, so they are the window in which a naive caller sees a half-recorded
// torrent; on a busy disk that window is whatever the file flush costs, here it is made visible.
type slowCompletion struct {
	storage.PieceCompletion
	delay time.Duration
}

func (s slowCompletion) Set(pk metainfo.PieceKey, complete bool) error {
	if complete {
		time.Sleep(s.delay)
	}
	return s.PieceCompletion.Set(pk, complete)
}

func (s slowCompletion) GetRange(ih infohash.T, begin, end int) iter.Seq[storage.Completion] {
	inner := s.PieceCompletion.(storage.PieceCompletionGetRanger).GetRange(ih, begin, end)
	return func(yield func(storage.Completion) bool) {
		time.Sleep(s.delay)
		inner(yield)
	}
}

func useSlowCompletion(t *testing.T, delay time.Duration) {
	t.Helper()
	orig := newCompletion
	newCompletion = func() storage.PieceCompletion { return slowCompletion{orig(), delay} }
	t.Cleanup(func() { newCompletion = orig })
}

// Seeding a folder that matches must succeed even when the library is slow to record the hashes:
// a ~800 MB game on a busy disk was reported as "bytes do not match" and never got seeded.
func TestSeed_WaitsForHashResultsToBeRecorded(t *testing.T) {
	useSlowCompletion(t, 150*time.Millisecond)
	src := filepath.Join(t.TempDir(), "Game")
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, 1<<20))
	job := publish(t, src)
	job.Dest = src
	require.NoError(t, newTestEngine(t).Seed(context.Background(), job))
}

// Closing the engine right after a failed Seed used to crash the whole process: the library was
// still recording the pieces that did match, and Remove had already wiped its completion map
// ("panic: false" in storage/file-piece.go, raised on a goroutine nothing can recover).
func TestSeed_CloseAfterFailureDoesNotCrash(t *testing.T) {
	useSlowCompletion(t, 150*time.Millisecond)
	src := filepath.Join(t.TempDir(), "Game")
	data := randomBytes(t, 2<<20)
	writeFile(t, filepath.Join(src, "a.bin"), data)
	job := publish(t, src)
	data[0] ^= 0xff // first piece no longer matches; the rest still do
	writeFile(t, filepath.Join(src, "a.bin"), data)

	e := newTestEngine(t)
	job.Dest = src
	require.ErrorContains(t, e.Seed(context.Background(), job), "do not match")
	time.Sleep(220 * time.Millisecond) // the library is between recording a piece and reading it back
	e.Close()
	time.Sleep(500 * time.Millisecond) // let any straggling hasher goroutine run
}

// Deleting a game (or shutting down) while its folder is still being verified must neither hang
// nor crash: the library leaves its client lock held when a piece check meets a dropped torrent.
func TestRemove_DuringSeedDoesNotWedgeEngine(t *testing.T) {
	useSlowCompletion(t, 100*time.Millisecond)
	src := filepath.Join(t.TempDir(), "Game")
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, 96<<20))
	job := publish(t, src)
	job.Dest = src

	e := newTestEngine(t)
	seeded := make(chan error, 1)
	go func() { seeded <- e.Seed(context.Background(), job) }()
	time.Sleep(200 * time.Millisecond) // verification of ~400 pieces is well under way

	done := make(chan struct{})
	go func() {
		e.Remove("Game")
		e.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("Remove/Close hung while a Seed was verifying")
	}
	select {
	case err := <-seeded:
		assert.Error(t, err, "the interrupted Seed must report failure, not success")
	case <-time.After(15 * time.Second):
		t.Fatal("Seed never returned after its game was removed")
	}
}

// Work started after Close is refused instead of touching a closed client.
func TestEngine_RefusesWorkAfterClose(t *testing.T) {
	src := filepath.Join(t.TempDir(), "Game")
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, 1<<20))
	job := publish(t, src)
	job.Dest = src
	e := newTestEngine(t)
	e.Close()
	assert.Error(t, e.Seed(context.Background(), job))
	assert.Error(t, e.Install(context.Background(), job))
}

// slowWriter throttles a response to roughly 32 KB per `delay`.
type slowWriter struct {
	http.ResponseWriter
	delay time.Duration
}

func (w slowWriter) Write(b []byte) (int, error) {
	time.Sleep(w.delay)
	return w.ResponseWriter.Write(b)
}

func installStallJob(t *testing.T, src string, webseed string, stall time.Duration) Job {
	t.Helper()
	job := publish(t, src)
	job.Dest = filepath.Join(t.TempDir(), "Game")
	job.Webseeds = []string{webseed + "/"}
	job.StallTimeout = stall
	return job
}

// The webseed accepts the connection and never answers (master wedged, firewall black-holing):
// Install blocked until the process restarted, so the whole sync loop sat on that one game.
func TestInstall_GivesUpWhenWebseedNeverAnswers(t *testing.T) {
	src := filepath.Join(t.TempDir(), "Game")
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, 2<<20))
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer hang.Close()
	defer hang.CloseClientConnections()

	e := newTestEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	err := e.Install(ctx, installStallJob(t, src, hang.URL, 600*time.Millisecond))
	require.ErrorIs(t, err, ErrStalled)
	assert.Less(t, time.Since(start), 10*time.Second, "must give up around the stall timeout, not at ctx expiry")
	_, still := e.Progress("Game")
	assert.False(t, still, "a failed install must not keep downloading in the background")
}

// The torrent client only asks a webseed for data on a 5 s timer that starts with the client, so a
// test that needs the webseed to actually deliver must outlast ~5 s before any stall timeout counts.
const webseedWarmup = 5 * time.Second

// The reported case: the master changed a file after publishing, so the webseed streams bytes the
// torrent rejects. Data flows, hashes fail, progress stays 0 — bytes received is the wrong signal.
func TestInstall_GivesUpWhenWebseedServesBytesTheTorrentRejects(t *testing.T) {
	t.Parallel()
	masterRoot := t.TempDir()
	src := filepath.Join(masterRoot, "Game")
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, 2<<20))
	seed := &countingSeed{root: masterRoot}
	srv := httptest.NewServer(seed)
	defer srv.Close()
	job := installStallJob(t, src, srv.URL, webseedWarmup+2*time.Second)
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, 2<<20)) // changed after publish

	e := newTestEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err := e.Install(ctx, job)
	require.ErrorIs(t, err, ErrStalled)
	assert.Greater(t, seed.served.Load(), int64(0), "the webseed really did send data")
}

// A slow link must not be mistaken for a dead one: the clock restarts whenever a piece verifies.
func TestInstall_SlowButProgressingIsNotStalled(t *testing.T) {
	t.Parallel()
	masterRoot := t.TempDir()
	src := filepath.Join(masterRoot, "Game")
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, 2<<20))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.FileServer(http.Dir(masterRoot)).ServeHTTP(slowWriter{w, 100 * time.Millisecond}, r)
	}))
	defer srv.Close()
	stall := webseedWarmup + 2500*time.Millisecond // one 256 KB piece takes ~1 s; the whole file takes longer than this
	job := installStallJob(t, src, srv.URL, stall)

	e := newTestEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	start := time.Now()
	require.NoError(t, e.Install(ctx, job))
	assert.Greater(t, time.Since(start), stall, "the download was meant to outlast the stall timeout")
	assertSameTree(t, src, job.Dest)
}
