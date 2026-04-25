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
	"strings"
	"sync"
	"time"
)

// LabelHivePoolKey marks pool-managed containers in their docker labels.
const LabelHivePoolKey = "hive.pool.key"

// poolResetPort is the JSON-RPC port hive talks to for the inter-test
// reset RPC. It matches the standard HIVE_CHECK_LIVE_PORT default.
const poolResetPort = 8545

// poolResetTimeout caps each debug_setHead call. The unwind itself is
// sub-millisecond inside Erigon; this is a generous safety net for HTTP.
const poolResetTimeout = 5 * time.Second

// PoolEntry is a single parked container.
type PoolEntry struct {
	ID  string
	IP  string
	key string // bucket key, set by Release
}

// ClientPool retains running client containers across tests. After a
// test ends, hive sends the client a JSON-RPC `debug_setHead(0)` to
// revert chain state to genesis and parks the entry under its
// (image, env, files) key. The next matching test reuses the
// already-running daemon — no docker create/start, no client init,
// no daemon boot.
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
	reset func(ip string) error

	mu     sync.Mutex
	list   *list.List // *PoolEntry, oldest at front
	closed bool

	// deleting tracks async DeleteContainer goroutines spawned by
	// eviction; Drain waits on it so shutdown sees all cleanup through.
	deleting sync.WaitGroup

	// Counters for the end-of-run summary log line.
	hits, misses, evicted, resetFailed uint64
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
	return p
}

// Enabled reports whether the pool is active.
func (p *ClientPool) Enabled() bool {
	return p != nil && p.maxIdle > 0
}

// ComputePoolKey hashes the inputs that determine "this container is
// interchangeable with another for the next test": the image name, the
// sanitized HIVE_* environment, and the contents of every file passed
// in (notably /genesis.json).
func ComputePoolKey(image string, env map[string]string, files map[string]*multipart.FileHeader) (string, error) {
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

// Acquire returns a parked entry whose key matches, or nil if none.
// On a hit the daemon is already running with chain state reset to
// genesis (done at Release), so the caller can use ID/IP directly.
func (p *ClientPool) Acquire(key string) *PoolEntry {
	if !p.Enabled() {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	// Scan from the back (newest) so we return hot entries first.
	for e := p.list.Back(); e != nil; e = e.Prev() {
		entry := e.Value.(*PoolEntry)
		if entry.key == key {
			p.list.Remove(e)
			p.hits++
			return entry
		}
	}
	p.misses++
	return nil
}

// Release sends debug_setHead(0) to revert the daemon to genesis, then
// parks the entry. If the global cap is exceeded the oldest entry is
// evicted (DeleteContainer in a background goroutine). On reset failure
// returns false; caller is expected to delete the container.
func (p *ClientPool) Release(entry PoolEntry, key string) bool {
	if !p.Enabled() || entry.ID == "" || entry.IP == "" || key == "" {
		return false
	}
	if err := p.reset(entry.IP); err != nil {
		slog.Warn("pool: reset failed, not retaining",
			"container", shortID(entry.ID), "ip", entry.IP, "err", err)
		p.mu.Lock()
		p.resetFailed++
		p.mu.Unlock()
		return false
	}

	entry.key = key
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
	p.mu.Unlock()

	// DeleteContainer can take 50–200 ms; run it off the test-end hot
	// path. Drain waits on the WaitGroup at shutdown.
	if victimID != "" {
		p.deleting.Add(1)
		go func() {
			defer p.deleting.Done()
			if err := p.backend.DeleteContainer(victimID); err != nil {
				slog.Warn("pool: evict delete failed", "container", shortID(victimID), "err", err)
			}
		}()
	}
	return true
}

// defaultReset POSTs debug_setHead(0x0) to ip:8545. Erigon already
// exposes this on --http.api=...,debug — the test harness uses
// --http.api=admin,debug,trace,eth,net,txpool,web3,testing — so no
// client-side changes are needed for Erigon. Other clients (geth,
// reth, nethermind, besu) all support debug_setHead.
func (p *ClientPool) defaultReset(ip string) error {
	ctx, cancel := context.WithTimeout(context.Background(), poolResetTimeout)
	defer cancel()

	url := fmt.Sprintf("http://%s/", net.JoinHostPort(ip, fmt.Sprintf("%d", poolResetPort)))
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

// Drain removes every parked container at hive shutdown.
func (p *ClientPool) Drain(ctx context.Context) {
	if !p.Enabled() {
		return
	}
	p.mu.Lock()
	p.closed = true
	hits, misses, evicted, resetFailed := p.hits, p.misses, p.evicted, p.resetFailed
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
		"evicted", evicted, "reset_failed", resetFailed)

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
