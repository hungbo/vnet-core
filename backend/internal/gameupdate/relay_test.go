package gameupdate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hangServer accepts requests and never answers: a webseed nobody can reach.
func hangServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	t.Cleanup(func() { s.CloseClientConnections(); s.Close() })
	return s
}

func relayGame(t *testing.T, size int) (masterRoot, src string) {
	t.Helper()
	masterRoot = t.TempDir()
	src = filepath.Join(masterRoot, "Game")
	writeFile(t, filepath.Join(src, "a.bin"), randomBytes(t, size))
	return masterRoot, src
}

// The webseed is dead, the tracker introduces nobody: after WebseedDelay and then RelayDelay the relay is attached and
// carries the whole install, and Transfer says so.
func TestInstall_RelayIsTheLastResort(t *testing.T) {
	root, src := relayGame(t, 3<<20)
	relaySeed := &countingSeed{root: root}
	relaySrv := httptest.NewServer(relaySeed)
	defer relaySrv.Close()
	dead := hangServer(t)
	tr := newTestTracker(t)

	job := publish(t, src)
	job.Dest = filepath.Join(t.TempDir(), "Game")
	job.Trackers = []string{tr.url()}
	job.Webseeds = []string{dead.URL + "/"}
	job.WebseedDelay = 500 * time.Millisecond
	job.Relays = []string{relaySrv.URL + "/"}
	job.RelayDelay = 1500 * time.Millisecond
	var order []string
	job.OnWebseedFallback = func() { order = append(order, "webseed") }
	job.OnRelayFallback = func() { order = append(order, "relay") }
	var got Transfer
	job.OnTransfer = func(tr Transfer) { got = tr }

	e := newWebRTCEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	start := time.Now()
	require.NoError(t, e.Install(ctx, job))
	assert.GreaterOrEqual(t, time.Since(start), 1500*time.Millisecond, "relay waits for RelayDelay")
	assert.Equal(t, []string{"webseed", "relay"}, order)
	assertSameTree(t, src, job.Dest)
	assert.EqualValues(t, relaySeed.served.Load(), got.Relay, "bytes counted equal bytes the relay sent")
	assert.GreaterOrEqual(t, got.Relay, int64(3<<20))
	assert.Zero(t, got.Webseed)
	assert.Zero(t, got.P2P)
}

// A webseed that delivers keeps the relay out: it is attached only when everything else stalls.
func TestInstall_RelayStaysOutWhenWebseedDelivers(t *testing.T) {
	root, src := relayGame(t, 3<<20)
	webSeed := &countingSeed{root: root}
	webSrv := httptest.NewServer(webSeed)
	defer webSrv.Close()
	relaySeed := &countingSeed{root: root}
	relaySrv := httptest.NewServer(relaySeed)
	defer relaySrv.Close()
	tr := newTestTracker(t)

	job := publish(t, src)
	job.Dest = filepath.Join(t.TempDir(), "Game")
	job.Trackers = []string{tr.url()}
	job.Webseeds = []string{webSrv.URL + "/"}
	job.WebseedDelay = 500 * time.Millisecond
	job.Relays = []string{relaySrv.URL + "/"}
	job.RelayDelay = 20 * time.Second
	var relayed atomic.Bool
	job.OnRelayFallback = func() { relayed.Store(true) }
	var got Transfer
	job.OnTransfer = func(tr Transfer) { got = tr }

	e := newWebRTCEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	require.NoError(t, e.Install(ctx, job))
	assertSameTree(t, src, job.Dest)
	assert.False(t, relayed.Load())
	assert.Zero(t, relaySeed.served.Load())
	assert.Zero(t, got.Relay)
	assert.GreaterOrEqual(t, got.Webseed, int64(3<<20))
	assert.EqualValues(t, webSeed.served.Load(), got.Webseed)
}

// RelayDelay 0 never uses the relay, whatever else happens.
func TestInstall_RelayNeverWithoutDelay(t *testing.T) {
	root, src := relayGame(t, 1<<20)
	relaySeed := &countingSeed{root: root}
	relaySrv := httptest.NewServer(relaySeed)
	defer relaySrv.Close()

	job := publish(t, src)
	job.Dest = filepath.Join(t.TempDir(), "Game")
	job.Relays = []string{relaySrv.URL + "/"}
	job.StallTimeout = time.Second
	e := newTestEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.ErrorIs(t, e.Install(ctx, job), ErrStalled)
	assert.Zero(t, relaySeed.served.Load())
}

// Over WebRTC every byte is peer data: Transfer.P2P covers the file, the HTTP paths are zero.
func TestInstall_TransferCountsPeerBytes(t *testing.T) {
	root, src := relayGame(t, 4<<20)
	webSeed := &countingSeed{root: root}
	webSrv := httptest.NewServer(webSeed)
	defer webSrv.Close()
	tr := newTestTracker(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	job := publish(t, src)
	job.Trackers = []string{tr.url()}
	master := newWebRTCEngine(t)
	seedJob := job
	seedJob.Dest = src
	require.NoError(t, master.Seed(ctx, seedJob))
	require.Eventually(t, func() bool { return tr.seedAnnounces.Load() > 0 }, 10*time.Second, 20*time.Millisecond)

	job.Dest = filepath.Join(t.TempDir(), "Game")
	job.Webseeds = []string{webSrv.URL + "/"}
	job.WebseedDelay = 30 * time.Second
	var got Transfer
	job.OnTransfer = func(tr Transfer) { got = tr }
	require.NoError(t, newWebRTCEngine(t).Install(ctx, job))
	assertSameTree(t, src, job.Dest)
	assert.GreaterOrEqual(t, got.P2P, int64(4<<20))
	assert.Less(t, got.P2P, int64(8<<20), "no more than the file plus small overhead")
	assert.Zero(t, got.Webseed)
	assert.Zero(t, got.Relay)
	assert.EqualValues(t, got.P2P, got.Total())
}
