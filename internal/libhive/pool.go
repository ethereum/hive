package libhive

import (
	"bytes"
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LabelHivePoolKey marks pool-managed containers in their docker labels.
const LabelHivePoolKey = "hive.pool.key"

// poolResetTimeout caps each debug_setHead call. The unwind itself is
// sub-millisecond inside Erigon; this is a generous safety net for HTTP.
const poolResetTimeout = 5 * time.Second

// poolProbeTimeout caps the TCP liveness probe Acquire runs against a
// parked entry before handing it back. The daemon's listen socket is
// either up or it isn't — this isn't waiting on app logic.
const poolProbeTimeout = 500 * time.Millisecond

// PoolEntry is a single parked container.
//
// LogFile carries the JSON-relative log path that was set when the container
// was first created. The docker attach goroutine wrote the stream to that
// file once at first start; subsequent reuses must reuse the same path so
// per-test LogOffsets refer to the file that actually receives output.
//
// ResetPort is the JSON-RPC port the reset RPC targets — the same value the
// cold path puts in options.CheckLive (HIVE_CHECK_LIVE_PORT, default 8545).
//
// Key is the bucket the entry is filed under (sha256 of image/env/files/
// networks); callers must populate it before calling Release.
type PoolEntry struct {
	ID        string
	IP        string
	LogFile   string
	ResetPort uint16
	Key       string
}

// ClientPool retains running client containers across tests. After a
// test ends, hive sends the client a JSON-RPC `debug_setHead(0)` to
// revert chain state to genesis and parks the entry under its
// (image, env, files, networks) key. The next matching test reuses
// the already-running daemon — no docker create/start, no client
// init, no daemon boot.
//
// Suitability: `debug_setHead(0)` rewinds the canonical chain head only.
// It does NOT clear txpool entries, RPC subscriptions/filters, miner
// state, peer lists, or in-memory caches. Pooling is therefore safe for
// near-stateless workloads (e.g. EEST consume-engine, where each test
// is a single newPayload from genesis) and unsafe for tests that mutate
// any of the above. Workloads that don't fit should keep
// --client.pool.size=0.
//
// Entries are stored in a single global LRU list rather than per-key
// buckets: with pool.size bounded (~24 in practice) the linear scan
// in Acquire is trivially fast vs the docker calls it replaces. Eviction
// when full pops the LRU front (oldest); insertion appends to the back.
//
// Pool is opt-in via --client.pool.size. With size <= 0 every method is
// a cheap no-op and Acquire returns nothing.
type ClientPool struct {
	backend ContainerBackend
	maxIdle int
	// reset performs the inter-test chain reset. Production uses
	// defaultReset (HTTP debug_setHead); tests inject a stub.
	reset func(ip string, port uint16) error
	// probe is the cheap liveness check Acquire runs before handing an
	// entry back; defaultProbe does a short TCP dial. Tests inject a stub.
	probe func(ip string, port uint16) error

	mu     sync.Mutex
	list   *list.List // *PoolEntry, oldest at front
	closed bool

	// deleting tracks async DeleteContainer goroutines spawned by
	// eviction or stale-probe rejection; Drain waits on it so shutdown
	// sees all cleanup through.
	deleting sync.WaitGroup

	// Counters for the end-of-run summary log line.
	hits, misses, evicted, resetFailed, staleRejected uint64
}

// NewClientPool returns a pool that holds at most maxIdle entries
// globally. maxIdle <= 0 disables the pool.
func NewClientPool(backend ContainerBackend, maxIdle int) *ClientPool {
	p := &ClientPool{
		backend: backend,
		maxIdle: maxIdle,
		list:    list.New(),
	}
	p.reset = p.defaultReset
	p.probe = defaultProbe
	return p
}

// Enabled reports whether the pool is active.
func (p *ClientPool) Enabled() bool {
	return p != nil && p.maxIdle > 0
}

// ComputePoolKey hashes the inputs that determine "this container is
// interchangeable with another for the next test": the image name, the
// sanitized HIVE_* environment, the contents of every file passed in
// (notably /genesis.json), and the set of docker networks the container
// is initially connected to. Two tests with different network sets are
// not interchangeable — the cold path connects networks at startClient
// time and the warm path skips that step, so they must hash differently.
func ComputePoolKey(image string, env map[string]string, files map[string]*multipart.FileHeader, networks []string) (string, error) {
	h := sha256.New()
	io.WriteString(h, image)
	io.WriteString(h, "\x00")

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(h, "%s=%s\x00", k, env[k])
	}

	// Networks are tagged with a "net=" prefix so they can't collide with
	// HIVE_*-prefixed env entries above. Sorted for iteration-order independence.
	netNames := append([]string(nil), networks...)
	sort.Strings(netNames)
	for _, n := range netNames {
		fmt.Fprintf(h, "net=%s\x00", n)
	}

	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		io.WriteString(h, p)
		io.WriteString(h, "\x00")
		f, err := files[p].Open()
		if err != nil {
			return "", fmt.Errorf("pool key: open %s: %w", p, err)
		}
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return "", fmt.Errorf("pool key: read %s: %w", p, err)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Acquire returns a parked entry whose key matches and whose daemon
// answers a TCP probe, or nil if neither is true. On a hit the daemon
// is already running with chain state reset to genesis (done at
// Release) so the caller can use ID/IP directly.
//
// If a candidate fails the probe (daemon died between Release and now),
// the entry is dropped from the pool, its container scheduled for async
// DeleteContainer, and we look for the next candidate in the same
// bucket. Without this check Acquire would silently hand out dead
// containers and the caller would learn only when subsequent RPCs time
// out — the cold-path CheckLive wait that catches this is skipped on
// pool reuse.
func (p *ClientPool) Acquire(key string) *PoolEntry {
	if !p.Enabled() {
		return nil
	}
	for {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return nil
		}
		// Scan from the back (newest) so we return hot entries first.
		var found *list.Element
		for e := p.list.Back(); e != nil; e = e.Prev() {
			if e.Value.(*PoolEntry).Key == key {
				found = e
				break
			}
		}
		if found == nil {
			p.misses++
			p.mu.Unlock()
			return nil
		}
		entry := found.Value.(*PoolEntry)
		p.list.Remove(found)
		p.mu.Unlock()

		// Probe outside the lock: TCP dial may take up to poolProbeTimeout
		// in the failure case, and we don't want to block other
		// Acquire/Release calls behind it.
		if err := p.probe(entry.IP, entry.ResetPort); err == nil {
			p.mu.Lock()
			p.hits++
			p.mu.Unlock()
			return entry
		} else {
			slog.Warn("pool: stale entry rejected, deleting",
				"container", shortID(entry.ID), "ip", entry.IP, "err", err)
			p.mu.Lock()
			p.staleRejected++
			// Add(1) under the lock for the same reason as Release: keeps
			// Drain.Wait from racing past a not-yet-spawned goroutine.
			if !p.closed {
				p.deleting.Add(1)
			}
			closed := p.closed
			p.mu.Unlock()

			if !closed {
				go func(id string) {
					defer p.deleting.Done()
					if err := p.backend.DeleteContainer(id); err != nil {
						slog.Warn("pool: stale delete failed", "container", shortID(id), "err", err)
					}
				}(entry.ID)
			}
			// Loop: another entry in this bucket might still be alive.
		}
	}
}

// Release sends debug_setHead(0) to revert the daemon to genesis, then
// parks the entry under entry.Key. If the global cap is exceeded the
// oldest entry is evicted (DeleteContainer in a background goroutine).
// On reset failure returns false; caller is expected to delete the
// container.
func (p *ClientPool) Release(entry PoolEntry) bool {
	if !p.Enabled() || entry.ID == "" || entry.IP == "" || entry.Key == "" || entry.ResetPort == 0 {
		return false
	}
	if err := p.reset(entry.IP, entry.ResetPort); err != nil {
		slog.Warn("pool: reset failed, not retaining",
			"container", shortID(entry.ID), "ip", entry.IP, "err", err)
		p.mu.Lock()
		p.resetFailed++
		p.mu.Unlock()
		return false
	}

	var victimID string
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return false
	}
	if p.list.Len() >= p.maxIdle {
		oldest := p.list.Front()
		victimID = oldest.Value.(*PoolEntry).ID
		p.list.Remove(oldest)
		p.evicted++
	}
	p.list.PushBack(&entry)
	// Add to the WaitGroup *while still holding p.mu*: Drain takes the
	// same lock to flip closed=true and read the list, so any Add visible
	// to a future Release was either accounted for in this critical
	// section or short-circuited by the closed check above. Without the
	// lock here, an Add after Drain unlocks but before Drain.Wait could
	// spawn a goroutine the WaitGroup never observes.
	if victimID != "" {
		p.deleting.Add(1)
	}
	p.mu.Unlock()

	// DeleteContainer can take 50–200 ms; run it off the test-end hot
	// path. Drain waits on the WaitGroup at shutdown.
	if victimID != "" {
		go func() {
			defer p.deleting.Done()
			if err := p.backend.DeleteContainer(victimID); err != nil {
				slog.Warn("pool: evict delete failed", "container", shortID(victimID), "err", err)
			}
		}()
	}
	return true
}

// defaultReset POSTs debug_setHead(0x0) to ip:port. Erigon already
// exposes this on --http.api=...,debug — the test harness uses
// --http.api=admin,debug,trace,eth,net,txpool,web3,testing — so no
// client-side changes are needed for Erigon. Other clients (geth,
// reth, nethermind, besu) all support debug_setHead.
//
// The port is the same one the cold path uses for CheckLive
// (HIVE_CHECK_LIVE_PORT, default 8545); we don't assume 8545 unconditionally.
func (p *ClientPool) defaultReset(ip string, port uint16) error {
	ctx, cancel := context.WithTimeout(context.Background(), poolResetTimeout)
	defer cancel()

	url := fmt.Sprintf("http://%s/", net.JoinHostPort(ip, strconv.FormatUint(uint64(port), 10)))
	body := strings.NewReader(`{"jsonrpc":"2.0","method":"debug_setHead","params":["0x0"],"id":1}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("status %d: %s", resp.StatusCode, bytes.TrimSpace(buf))
	}
	// JSON-RPC returns 200 OK with an `error` field on application errors.
	var rpcResp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&rpcResp); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return nil
}

// defaultProbe does a TCP dial against the daemon's listen socket so
// Acquire can reject entries whose daemons died between Release and now.
// We don't issue an RPC — the dial succeeding is sufficient evidence the
// daemon is up; the next reset RPC at end-of-test will exercise the
// JSON-RPC path.
func defaultProbe(ip string, port uint16) error {
	addr := net.JoinHostPort(ip, strconv.FormatUint(uint64(port), 10))
	conn, err := net.DialTimeout("tcp", addr, poolProbeTimeout)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// Drain removes every parked container at hive shutdown.
func (p *ClientPool) Drain(ctx context.Context) {
	if !p.Enabled() {
		return
	}
	p.mu.Lock()
	p.closed = true
	hits, misses, evicted, resetFailed, staleRejected := p.hits, p.misses, p.evicted, p.resetFailed, p.staleRejected
	victims := make([]string, 0, p.list.Len())
	for e := p.list.Front(); e != nil; e = e.Next() {
		victims = append(victims, e.Value.(*PoolEntry).ID)
	}
	p.list = list.New()
	p.mu.Unlock()

	hitRate := 0.0
	if hits+misses > 0 {
		hitRate = 100 * float64(hits) / float64(hits+misses)
	}
	slog.Info("pool: summary",
		"hits", hits, "misses", misses,
		"hit_rate_pct", fmt.Sprintf("%.1f", hitRate),
		"evicted", evicted, "reset_failed", resetFailed,
		"stale_rejected", staleRejected)

	for _, id := range victims {
		if err := p.backend.DeleteContainer(id); err != nil {
			slog.Warn("pool drain: delete failed", "container", shortID(id), "err", err)
		}
	}
	p.deleting.Wait()
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
