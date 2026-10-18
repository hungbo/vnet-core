package gameupdate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	g "github.com/anacrolix/generics"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

// EngineConfig configures the BitTorrent client shared by every game.
type EngineConfig struct {
	// DataDir holds the client's own state. Game data never goes here.
	DataDir    string
	ListenPort int // 0 picks a free port
	NoDHT      bool
	Logger     *slog.Logger
}

// Engine downloads and seeds game folders. One torrent per game name; installing a new version
// of a game replaces the torrent for that name.
type Engine struct {
	client *torrent.Client

	mu     sync.Mutex
	games  map[string]*game
	closed bool
}

type game struct {
	t    *torrent.Torrent
	dest string
	// stop aborts the verification add() runs on this torrent, and added is closed once add() has
	// stopped touching it: Remove waits for both before dropping the torrent.
	stop  context.CancelFunc
	added chan struct{}
}

var (
	// ErrStalled is returned by Install when no new piece has verified for Job.StallTimeout.
	ErrStalled      = errors.New("gameupdate: download stalled")
	errEngineClosed = errors.New("gameupdate: engine is closed")
)

// settleTimeout bounds how long Seed waits for the library to finish recording hash results.
const settleTimeout = 30 * time.Second

// Job describes one game version to put on disk.
type Job struct {
	Name     string             // game identifier; one active torrent per name
	MetaInfo *metainfo.MetaInfo // the version to install
	Dest     string             // the game folder; files land directly under it
	Webseeds []string           // BEP19 base URLs, e.g. http://master/seed/
	Trackers []string
	Peers    []torrent.PeerInfo // known peers to dial straight away
	// Keep lists slash-separated glob patterns (relative to Dest) that cleanup must not delete,
	// e.g. "config/*.ini" for settings a game writes next to its binaries.
	Keep []string
	// StallTimeout makes Install give up with ErrStalled when no new piece has VERIFIED for this
	// long, 0 = wait forever. Verified, not received: a webseed that serves bytes the torrent
	// rejects (the master changed a file after publishing) keeps data flowing and never completes.
	StallTimeout time.Duration
}

func NewEngine(cfg EngineConfig) (*Engine, error) {
	if cfg.DataDir == "" {
		return nil, errors.New("gameupdate: DataDir is required")
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	tc := torrent.NewDefaultClientConfig()
	tc.DataDir = cfg.DataDir
	// Never let the client create its default storage: that writes a piece-completion database
	// into DataDir and trusts it across restarts, which is wrong once files change underneath.
	tc.DefaultStorage = newStorage(cfg.DataDir)
	tc.ListenPort = cfg.ListenPort
	tc.NoDHT = cfg.NoDHT
	tc.Seed = true
	tc.DisableWebtorrent = true
	tc.NoDefaultPortForwarding = true
	if cfg.Logger != nil {
		tc.Slogger = cfg.Logger
	}
	cl, err := torrent.NewClient(tc)
	if err != nil {
		return nil, err
	}
	return &Engine{client: cl, games: map[string]*game{}}, nil
}

// newCompletion builds the in-memory piece-completion map of one game's storage. A variable only so
// tests can stretch the gap between "piece hashed" and "result recorded" long enough to observe it.
var newCompletion = storage.NewMapPieceCompletion

// newStorage writes files straight into dir (no "<torrent name>/" level, no .part files) and
// keeps piece completion in memory only. Every install re-verifies what is on disk, so a file
// edited by hand or half-written by a crash is detected rather than trusted.
func newStorage(dir string) storage.ClientImplCloser {
	return storage.NewFileOpts(storage.NewFileClientOpts{
		ClientBaseDir: dir,
		FilePathMaker: func(o storage.FilePathMakerOpts) string {
			return filepath.Join(o.File.BestPath()...)
		},
		PieceCompletion: newCompletion(),
		UsePartFiles:    g.Some(false),
	})
}

// Install brings job.Dest to exactly the version in job.MetaInfo and blocks until it is verified
// complete, ctx ends or the transfer stalls (see Job.StallTimeout). Pieces already correct on disk
// are kept; only the rest is downloaded. Afterwards files that are not part of the version are
// deleted, so the folder matches the master. The game keeps seeding to other cafes after Install
// returns; when it fails the torrent is dropped, so nothing keeps downloading in the background.
func (e *Engine) Install(ctx context.Context, job Job) (err error) {
	info, err := checkJob(job)
	if err != nil {
		return err
	}
	if err := trimOversizedFiles(info, job.Dest); err != nil {
		return err
	}
	t, err := e.add(ctx, job)
	if err != nil {
		e.Remove(job.Name)
		return err
	}
	defer func() {
		if err != nil {
			e.Remove(job.Name)
		}
	}()
	t.DownloadAll()
	if err = waitComplete(ctx, t, job.StallTimeout); err != nil {
		return err
	}
	return removeRedundant(info, job.Dest, job.Keep)
}

// waitComplete blocks until t is complete. With stall > 0 it fails with ErrStalled once the number
// of verified pieces has not grown for that long.
func waitComplete(ctx context.Context, t *torrent.Torrent, stall time.Duration) error {
	if stall <= 0 {
		select {
		case <-t.Complete().On():
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	tick := time.NewTicker(min(max(stall/10, 10*time.Millisecond), 5*time.Second))
	defer tick.Stop()
	best, since := t.Stats().PiecesComplete, time.Now()
	for {
		select {
		case <-t.Complete().On():
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case now := <-tick.C:
			if n := t.Stats().PiecesComplete; n > best {
				best, since = n, now
			} else if now.Sub(since) >= stall {
				return fmt.Errorf("%w: %d of %d pieces verified, none new for %s", ErrStalled, best, t.NumPieces(), stall)
			}
		}
	}
}

// Seed shares a folder that already holds the full version (the master's own copy). Nothing is
// downloaded or deleted; it fails if the data on disk does not match the torrent.
func (e *Engine) Seed(ctx context.Context, job Job) error {
	if _, err := checkJob(job); err != nil {
		return err
	}
	t, err := e.add(ctx, job)
	if err != nil {
		return err
	}
	if err := waitHashed(ctx, t); err != nil {
		return fmt.Errorf("seed %s: %w", job.Name, err)
	}
	if missing := t.BytesMissing(); missing != 0 {
		return fmt.Errorf("seed %s: %d bytes on disk do not match the torrent", job.Name, missing)
	}
	return nil
}

// waitHashed blocks until no piece is being hashed or having its result recorded. add() returns as
// soon as the last hash is computed, but the library records a passed piece only afterwards
// (pieceHashed releases its lock around the storage write), so BytesMissing read right then still
// counts pieces that are in fact fine. That reported ~800 MB games as "bytes do not match".
func waitHashed(ctx context.Context, t *torrent.Torrent) error {
	timeout := time.NewTimer(settleTimeout)
	defer timeout.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		busy := false
		for _, run := range t.PieceStateRuns() {
			if run.Checking || run.Marking {
				busy = true
				break
			}
		}
		if !busy {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("piece checks still running after %s", settleTimeout)
		case <-tick.C:
		}
	}
}

// add registers the torrent (replacing any older version of the same game) and re-hashes what is
// already on disk.
func (e *Engine) add(ctx context.Context, job Job) (*torrent.Torrent, error) {
	e.Remove(job.Name)

	spec, err := torrent.TorrentSpecFromMetaInfoErr(job.MetaInfo)
	if err != nil {
		return nil, err
	}
	store := newStorage(job.Dest)
	spec.Storage = store
	spec.Webseeds = job.Webseeds
	if len(job.Trackers) > 0 {
		spec.Trackers = [][]string{job.Trackers}
	}

	ctx, stop := context.WithCancel(ctx)
	defer stop()
	added := make(chan struct{})
	defer close(added)

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		_ = store.Close()
		return nil, errEngineClosed
	}
	t, _, err := e.client.AddTorrentSpec(spec)
	if err != nil {
		e.mu.Unlock()
		_ = store.Close()
		return nil, err
	}
	e.games[job.Name] = &game{t: t, dest: job.Dest, stop: stop, added: added}
	e.mu.Unlock()

	select {
	case <-t.GotInfo():
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := t.VerifyDataContext(ctx); err != nil {
		return nil, fmt.Errorf("verify %s: %w", job.Name, err)
	}
	if len(job.Peers) > 0 {
		t.AddPeers(job.Peers)
	}
	return t, nil
}

// Remove stops downloading and seeding a game. Files on disk are left alone.
func (e *Engine) Remove(name string) {
	e.mu.Lock()
	gm := e.games[name]
	delete(e.games, name)
	e.mu.Unlock()
	if gm == nil {
		return
	}
	// A verification still running on this torrent has to stop first: the library returns from a
	// piece check on a dropped torrent with its client lock still held, which wedges the whole engine.
	gm.stop()
	<-gm.added
	gm.t.Drop()
	// The storage is deliberately NOT closed. Its only resource is the in-memory piece-completion
	// map, and closing wipes that map while a hasher may still be recording a piece — the library
	// then panics in storage/file-piece.go on a goroutine nothing can recover, killing the process.
	// The map is garbage once the dropped torrent is.
}

// Progress is a snapshot of one game's transfer.
type Progress struct {
	BytesCompleted int64
	Length         int64
	Complete       bool
	Peers          int
}

func (e *Engine) Progress(name string) (Progress, bool) {
	e.mu.Lock()
	gm := e.games[name]
	e.mu.Unlock()
	if gm == nil || gm.t.Info() == nil {
		return Progress{}, false
	}
	return Progress{
		BytesCompleted: gm.t.BytesCompleted(),
		Length:         gm.t.Length(),
		Complete:       gm.t.Complete().Bool(),
		Peers:          gm.t.Stats().ActivePeers,
	}, true
}

// ListenAddrs lets a peer on the same machine (tests, or the master seeding to itself) dial in.
func (e *Engine) ListenAddrs() []torrent.PeerInfo {
	var out []torrent.PeerInfo
	for _, a := range e.client.ListenAddrs() {
		out = append(out, torrent.PeerInfo{Addr: a, Trusted: true})
	}
	return out
}

func (e *Engine) Close() {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	names := make([]string, 0, len(e.games))
	for n := range e.games {
		names = append(names, n)
	}
	e.mu.Unlock()
	for _, n := range names {
		e.Remove(n)
	}
	e.client.Close()
}

func checkJob(job Job) (*metainfo.Info, error) {
	if job.Name == "" || job.MetaInfo == nil {
		return nil, errors.New("gameupdate: job needs Name and MetaInfo")
	}
	if err := checkDest(job.Dest); err != nil {
		return nil, err
	}
	info, err := job.MetaInfo.UnmarshalInfo()
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("gameupdate: torrent is not a multi-file torrent")
	}
	for _, f := range info.UpvertedFiles() {
		if err := checkFilePath(f.BestPath()); err != nil {
			return nil, err
		}
	}
	return &info, nil
}

// checkFilePath rejects torrent paths that could leave the game folder. The torrent arrives over
// the network from the master, and Install truncates, deletes and writes at these paths — a ".."
// component or a drive letter would let a bad torrent touch any file on the cafe server.
func checkFilePath(parts []string) error {
	if len(parts) == 0 {
		return errors.New("gameupdate: torrent has a file with an empty path")
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, `/\:`) || strings.ContainsRune(p, 0) {
			return fmt.Errorf("gameupdate: unsafe file path %q in torrent", strings.Join(parts, "/"))
		}
	}
	return nil
}

// checkDest refuses destinations where deleting "files not in the torrent" would be a disaster:
// relative paths and filesystem roots such as E:\ or /.
func checkDest(dest string) error {
	if dest == "" || !filepath.IsAbs(dest) {
		return fmt.Errorf("gameupdate: destination %q must be an absolute path", dest)
	}
	clean := filepath.Clean(dest)
	if filepath.Dir(clean) == clean {
		return fmt.Errorf("gameupdate: destination %q is a filesystem root", dest)
	}
	return nil
}

// trimOversizedFiles truncates files that shrank in the new version. Piece checks only cover the
// bytes the torrent describes, so a longer old file would verify as complete and keep its stale
// tail — a corrupt game that every check calls healthy.
func trimOversizedFiles(info *metainfo.Info, dest string) error {
	for _, f := range info.UpvertedFiles() {
		p := filepath.Join(append([]string{dest}, f.BestPath()...)...)
		fi, err := os.Stat(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if fi.IsDir() {
			// The new version has a file where the old one had a folder.
			if err := os.RemoveAll(p); err != nil {
				return err
			}
			continue
		}
		if fi.Size() > f.Length {
			if err := os.Truncate(p, f.Length); err != nil {
				return err
			}
		}
	}
	return nil
}

// removeRedundant deletes files under dest that the version does not contain, then any folders
// left empty. Paths matching keep survive.
func removeRedundant(info *metainfo.Info, dest string, keep []string) error {
	want := map[string]bool{}
	for _, f := range info.UpvertedFiles() {
		want[pathKey(strings.Join(f.BestPath(), "/"))] = true
	}
	var dirs []string
	err := filepath.WalkDir(dest, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dest {
			return nil
		}
		rel, err := filepath.Rel(dest, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if kept(rel, keep) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		if !want[pathKey(rel)] {
			return os.Remove(path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Deepest first; os.Remove fails harmlessly on folders that still hold files.
	for i := len(dirs) - 1; i >= 0; i-- {
		_ = os.Remove(dirs[i])
	}
	return nil
}

func kept(rel string, keep []string) bool {
	for _, pattern := range keep {
		if ok, _ := filepath.Match(pathKey(pattern), pathKey(rel)); ok {
			return true
		}
	}
	return false
}

// pathKey compares paths the way the filesystem does: Windows (NTFS) ignores case, so "Game.EXE"
// on disk is the torrent's "game.exe" and must not be deleted as redundant.
func pathKey(p string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(p)
	}
	return p
}
