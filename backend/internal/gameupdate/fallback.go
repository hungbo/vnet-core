package gameupdate

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anacrolix/torrent"
)

// Transfer is how many payload bytes one finished Install received over each path. The three are measured the same
// way (piece data as delivered, wasted or not), so they can be compared and added. A cafe reports it to the Hub to show
// how much really went peer to peer, over the master's webseed or through the Hub's relay.
type Transfer struct {
	P2P     int64 // WebRTC / BitTorrent peers
	Webseed int64 // Job.Webseeds (HTTP straight to the master)
	Relay   int64 // Job.Relays (HTTP through the Hub)
}

func (t Transfer) Total() int64 { return t.P2P + t.Webseed + t.Relay }

// hasFallback reports whether Install has to watch for a stall to attach webseeds and/or relays.
func (j Job) hasFallback() bool { return j.webseedsAreFallback() || j.relaysAreFallback() }

// relaysAreFallback: relays are only ever a last resort, so without RelayDelay they are never attached.
func (j Job) relaysAreFallback() bool { return j.RelayDelay > 0 && len(j.Relays) > 0 }

// attachFallbacksWhenStuck adds the webseeds, then the relays, once no new piece has verified for their delay (counted
// from the last verified piece, so a path that starts delivering keeps the later ones detached). It returns when
// everything is attached, t completes or ctx ends.
func attachFallbacksWhenStuck(ctx context.Context, t *torrent.Torrent, job Job) {
	wsOn, rlOn := job.webseedsAreFallback(), job.relaysAreFallback()
	first := job.WebseedDelay
	if !wsOn || (rlOn && job.RelayDelay < first) {
		first = job.RelayDelay
	}
	tick := time.NewTicker(min(max(first/10, 10*time.Millisecond), time.Second))
	defer tick.Stop()
	best, since := t.Stats().PiecesComplete, time.Now()
	for wsOn || rlOn {
		select {
		case <-ctx.Done():
			return
		case <-t.Complete().On():
			return
		case now := <-tick.C:
			if n := t.Stats().PiecesComplete; n > best {
				best, since = n, now
				continue
			}
			stuck := now.Sub(since)
			if wsOn && stuck >= job.WebseedDelay {
				t.AddWebSeeds(job.Webseeds)
				wsOn = false
				if job.OnWebseedFallback != nil {
					job.OnWebseedFallback()
				}
			}
			if rlOn && stuck >= job.RelayDelay {
				t.AddWebSeeds(job.Relays)
				rlOn = false
				if job.OnRelayFallback != nil {
					job.OnRelayFallback()
				}
			}
		}
	}
}

// countRules attributes the bytes of HTTP responses to a path by URL prefix. The engine's HTTP transport is shared by
// every webseed of every game, so each Install registers the prefixes of its own webseeds and relays.
type countRules struct {
	mu   sync.Mutex
	list []*countRule
}

type countRule struct {
	scheme, host, path string
	n                  *atomic.Int64
}

// add registers base + "<game>/" (the URL BEP19 requests for that game) and returns the function that unregisters it.
func (c *countRules) add(base, game string, n *atomic.Int64) func() {
	u, err := url.Parse(base)
	if err != nil {
		return func() {}
	}
	r := &countRule{scheme: u.Scheme, host: u.Host, path: strings.TrimSuffix(u.Path, "/") + "/" + game + "/", n: n}
	c.mu.Lock()
	c.list = append(c.list, r)
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		for i, o := range c.list {
			if o == r {
				c.list = append(c.list[:i:i], c.list[i+1:]...)
				return
			}
		}
	}
}

func (c *countRules) match(u *url.URL) *atomic.Int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range c.list {
		if r.scheme == u.Scheme && r.host == u.Host && strings.HasPrefix(u.Path, r.path) {
			return r.n
		}
	}
	return nil
}

// countingTransport wraps the engine's HTTP transport and counts response body bytes per registered rule.
type countingTransport struct {
	inner http.RoundTripper
	rules *countRules
}

func (t countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}
	if n := t.rules.match(req.URL); n != nil {
		resp.Body = countBody{resp.Body, n}
	}
	return resp, nil
}

type countBody struct {
	io.ReadCloser
	n *atomic.Int64
}

func (b countBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.n.Add(int64(n))
	return n, err
}

// countJob starts counting the bytes of job's webseeds and relays; done stops it and returns what was counted.
func (e *Engine) countJob(job Job) (ws, rl *atomic.Int64, done func()) {
	ws, rl = new(atomic.Int64), new(atomic.Int64)
	var stops []func()
	for _, u := range job.Webseeds {
		stops = append(stops, e.rules.add(u, job.Name, ws))
	}
	for _, u := range job.Relays {
		stops = append(stops, e.rules.add(u, job.Name, rl))
	}
	return ws, rl, func() {
		for _, s := range stops {
			s()
		}
	}
}

// transferOf reads what t received from peers; webseed and relay bytes come from the HTTP counters.
func transferOf(t *torrent.Torrent, ws, rl *atomic.Int64) Transfer {
	st := t.Stats()
	return Transfer{P2P: st.AllConnStats.PeerConns.BytesReadData.Int64(), Webseed: ws.Load(), Relay: rl.Load()}
}
