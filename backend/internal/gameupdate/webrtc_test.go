package gameupdate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTracker is the smallest WebTorrent tracker that lets two engines find each other: it forwards the offers of
// a downloader to a seed and the answers back. The Hub's tracker does the same plus authorization and load limits.
type testTracker struct {
	srv *httptest.Server

	mu     sync.Mutex
	swarms map[string]map[string]*trackerPeer // info_hash -> peer_id -> peer

	seedAnnounces atomic.Int32
}

type trackerPeer struct {
	conn *websocket.Conn
	wmu  sync.Mutex
	left int64
}

func (p *trackerPeer) send(v any) {
	p.wmu.Lock()
	defer p.wmu.Unlock()
	_ = p.conn.WriteJSON(v)
}

type trackerMsg struct {
	Action   string `json:"action"`
	InfoHash string `json:"info_hash"`
	PeerID   string `json:"peer_id"`
	Left     int64  `json:"left"`
	Offers   []struct {
		OfferID string          `json:"offer_id"`
		Offer   json.RawMessage `json:"offer"`
	} `json:"offers"`
	Answer   json.RawMessage `json:"answer"`
	ToPeerID string          `json:"to_peer_id"`
	OfferID  string          `json:"offer_id"`
}

func newTestTracker(t *testing.T) *testTracker {
	t.Helper()
	tr := &testTracker{swarms: map[string]map[string]*trackerPeer{}}
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	tr.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		me := &trackerPeer{conn: conn}
		var joined []struct{ ih, id string }
		defer func() {
			tr.mu.Lock()
			for _, j := range joined {
				delete(tr.swarms[j.ih], j.id)
			}
			tr.mu.Unlock()
		}()
		for {
			var m trackerMsg
			if conn.ReadJSON(&m) != nil {
				return
			}
			if m.Action != "announce" {
				continue
			}
			tr.mu.Lock()
			sw := tr.swarms[m.InfoHash]
			if sw == nil {
				sw = map[string]*trackerPeer{}
				tr.swarms[m.InfoHash] = sw
			}
			if sw[m.PeerID] != me {
				sw[m.PeerID] = me
				joined = append(joined, struct{ ih, id string }{m.InfoHash, m.PeerID})
			}
			me.left = m.Left
			var seeds, leechers int
			var seed *trackerPeer
			for id, p := range sw {
				if p.left == 0 {
					seeds++
					if id != m.PeerID {
						seed = p
					}
				} else {
					leechers++
				}
			}
			var to *trackerPeer
			if m.ToPeerID != "" {
				to = sw[m.ToPeerID]
			}
			tr.mu.Unlock()

			switch {
			case m.Answer != nil:
				if to != nil {
					to.send(map[string]any{"action": "announce", "info_hash": m.InfoHash, "peer_id": m.PeerID, "answer": m.Answer, "offer_id": m.OfferID})
				}
				continue
			case m.Left == 0:
				tr.seedAnnounces.Add(1)
			case seed != nil:
				for _, o := range m.Offers {
					seed.send(map[string]any{"action": "announce", "info_hash": m.InfoHash, "peer_id": m.PeerID, "offer": o.Offer, "offer_id": o.OfferID})
				}
			}
			me.send(map[string]any{"action": "announce", "info_hash": m.InfoHash, "interval": 120, "complete": seeds, "incomplete": leechers})
		}
	}))
	t.Cleanup(tr.srv.Close)
	return tr
}

func (tr *testTracker) url() string { return "ws" + strings.TrimPrefix(tr.srv.URL, "http") }

func newWebRTCEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := NewEngine(EngineConfig{DataDir: t.TempDir(), WebRTC: true})
	require.NoError(t, err)
	t.Cleanup(e.Close)
	return e
}

// WebRTC mode opens no BitTorrent port at all: the only way in is a tracker introduction.
func TestWebRTCEngine_OpensNoPort(t *testing.T) {
	e := newWebRTCEngine(t)
	assert.Empty(t, e.ListenAddrs())
}

// Peers find each other through the tracker alone, and while they deliver the webseed is never touched.
func TestInstall_WebRTCThroughTracker_WebseedStaysUnused(t *testing.T) {
	masterRoot := t.TempDir()
	src := filepath.Join(masterRoot, "CS2")
	writeFile(t, filepath.Join(src, "bin", "cs2.exe"), randomBytes(t, 2<<20))
	writeFile(t, filepath.Join(src, "maps", "dust2.vpk"), randomBytes(t, 5<<20))
	webseed := &countingSeed{root: masterRoot}
	ws := httptest.NewServer(webseed)
	defer ws.Close()
	tr := newTestTracker(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	job := publish(t, src)
	job.Trackers = []string{tr.url()}
	master := newWebRTCEngine(t)
	seedJob := job
	seedJob.Dest = src
	require.NoError(t, master.Seed(ctx, seedJob))
	require.Eventually(t, func() bool { return tr.seedAnnounces.Load() > 0 }, 10*time.Second, 20*time.Millisecond, "master must be on the tracker before the cafe asks")

	var fellBack atomic.Bool
	cafe := newWebRTCEngine(t)
	job.Dest = filepath.Join(t.TempDir(), "CS2")
	job.Webseeds = []string{ws.URL + "/"}
	job.WebseedDelay = 30 * time.Second
	job.OnWebseedFallback = func() { fellBack.Store(true) }
	require.NoError(t, cafe.Install(ctx, job))

	assertSameTree(t, src, job.Dest)
	assert.False(t, fellBack.Load())
	assert.Zero(t, webseed.served.Load(), "every byte came over WebRTC")
}

// No tracker answers: after WebseedDelay the webseed takes over and the install still completes.
func TestInstall_WebseedTakesOverWhenTrackerGivesNoPeers(t *testing.T) {
	masterRoot := t.TempDir()
	src := filepath.Join(masterRoot, "Game")
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, 3<<20))
	webseed := &countingSeed{root: masterRoot}
	ws := httptest.NewServer(webseed)
	defer ws.Close()
	tr := newTestTracker(t) // up, but nobody seeds: it introduces no one

	job := publish(t, src)
	job.Dest = filepath.Join(t.TempDir(), "Game")
	job.Trackers = []string{tr.url()}
	job.Webseeds = []string{ws.URL + "/"}
	job.WebseedDelay = time.Second
	var fallbacks atomic.Int32
	job.OnWebseedFallback = func() { fallbacks.Add(1) }

	e := newWebRTCEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	start := time.Now()
	require.NoError(t, e.Install(ctx, job))
	assert.GreaterOrEqual(t, time.Since(start), time.Second, "the webseed waits for WebseedDelay")
	assert.EqualValues(t, 1, fallbacks.Load())
	assert.Greater(t, webseed.served.Load(), int64(0))
	assertSameTree(t, src, job.Dest)
}

// Without Trackers (the old cafe-to-master path) the webseed is attached at once, WebseedDelay or not.
func TestInstall_WebseedDelayNeedsTrackers(t *testing.T) {
	masterRoot := t.TempDir()
	src := filepath.Join(masterRoot, "Game")
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, 1<<20))
	ws := httptest.NewServer(&countingSeed{root: masterRoot})
	defer ws.Close()

	job := publish(t, src)
	job.Dest = filepath.Join(t.TempDir(), "Game")
	job.Webseeds = []string{ws.URL + "/"}
	job.WebseedDelay = time.Hour
	e := newTestEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	require.NoError(t, e.Install(ctx, job))
	assertSameTree(t, src, job.Dest)
}

// A failed install must not leave the fallback goroutine behind to touch a dropped torrent.
func TestInstall_CanceledWhileWaitingForWebseedDelay(t *testing.T) {
	src := filepath.Join(t.TempDir(), "Game")
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, 1<<20))
	tr := newTestTracker(t)
	job := publish(t, src)
	job.Dest = filepath.Join(t.TempDir(), "Game")
	job.Trackers = []string{tr.url()}
	job.Webseeds = []string{"http://127.0.0.1:1/"}
	job.WebseedDelay = 400 * time.Millisecond

	e := newWebRTCEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, e.Install(ctx, job), context.DeadlineExceeded)
	_, still := e.Progress("Game")
	assert.False(t, still)
}
