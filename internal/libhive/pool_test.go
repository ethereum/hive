package libhive

import (
	"bytes"
	"context"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// recordingBackend is the minimum ContainerBackend surface area used by the
// pool. Methods that the pool doesn't touch are fine to leave as no-ops.
type recordingBackend struct {
	mu      sync.Mutex
	deletes []string
}

func (b *recordingBackend) DeleteContainer(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deletes = append(b.deletes, id)
	return nil
}

func (b *recordingBackend) Build(context.Context, Builder) error { return nil }
func (b *recordingBackend) SetHiveInstanceInfo(string, string)   {}
func (b *recordingBackend) GetDockerClient() interface{}         { return nil }
func (b *recordingBackend) ServeAPI(context.Context, http.Handler) (APIServer, error) {
	return nil, nil
}
func (b *recordingBackend) CreateContainer(context.Context, string, ContainerOptions) (string, error) {
	return "", nil
}
func (b *recordingBackend) StartContainer(context.Context, string, ContainerOptions) (*ContainerInfo, error) {
	return nil, nil
}
func (b *recordingBackend) PauseContainer(string) error   { return nil }
func (b *recordingBackend) UnpauseContainer(string) error { return nil }
func (b *recordingBackend) RunProgram(context.Context, string, []string) (*ExecInfo, error) {
	return nil, nil
}
func (b *recordingBackend) NetworkNameToID(string) (string, error)     { return "", nil }
func (b *recordingBackend) CreateNetwork(string) (string, error)       { return "", nil }
func (b *recordingBackend) RemoveNetwork(string) error                 { return nil }
func (b *recordingBackend) ContainerIP(string, string) (net.IP, error) { return nil, nil }
func (b *recordingBackend) ConnectContainer(string, string) error      { return nil }
func (b *recordingBackend) DisconnectContainer(string, string) error   { return nil }

// resetCall captures one invocation of the reset stub.
type resetCall struct {
	ip   string
	port uint16
}

// poolWithStubReset returns a pool whose reset function records calls
// instead of doing HTTP, so unit tests don't need a live RPC endpoint.
func poolWithStubReset(t *testing.T, maxIdle int, resetErr error) (*ClientPool, *recordingBackend, *[]resetCall) {
	t.Helper()
	be := &recordingBackend{}
	p := NewClientPool(be, maxIdle)
	var calls []resetCall
	p.reset = func(ip string, port uint16) error {
		calls = append(calls, resetCall{ip: ip, port: port})
		return resetErr
	}
	return p, be, &calls
}

// testEntry builds a PoolEntry with the boilerplate non-zero fields filled
// in so tests don't have to repeat ResetPort: 8545 everywhere.
func testEntry(id, ip string) PoolEntry {
	return PoolEntry{ID: id, IP: ip, ResetPort: 8545}
}

// waitForEvictions blocks until all async DeleteContainer goroutines
// spawned by Release have finished. Tests that observe be.deletes need
// to call this first.
func waitForEvictions(p *ClientPool) {
	p.deleting.Wait()
}

func TestPoolDisabled(t *testing.T) {
	be := &recordingBackend{}
	p := NewClientPool(be, 0)
	if p.Enabled() {
		t.Fatal("pool with size 0 should be disabled")
	}
	if got := p.Acquire("k"); got != nil {
		t.Fatalf("disabled pool should return nil on Acquire, got %+v", got)
	}
	if p.Release(testEntry("c", "1.2.3.4"), "k") {
		t.Fatal("disabled pool should not retain on Release")
	}
}

func TestPoolAcquireReleaseLIFO(t *testing.T) {
	// Pool fits all 3 entries; verify Acquire returns LIFO within bucket.
	p, _, calls := poolWithStubReset(t, 4, nil)

	if got := p.Acquire("k"); got != nil {
		t.Fatalf("empty bucket should return nil, got %+v", got)
	}

	e1 := testEntry("c1", "10.0.0.1")
	e2 := testEntry("c2", "10.0.0.2")
	e3 := testEntry("c3", "10.0.0.3")
	for _, e := range []PoolEntry{e1, e2, e3} {
		if !p.Release(e, "k") {
			t.Fatalf("Release of %s should succeed under cap", e.ID)
		}
	}

	if got := p.Acquire("k"); got == nil || got.ID != "c3" {
		t.Fatalf("Acquire should return most-recent (c3), got %+v", got)
	}
	if got := p.Acquire("k"); got == nil || got.ID != "c2" {
		t.Fatalf("Acquire should return c2 next, got %+v", got)
	}
	if got := p.Acquire("k"); got == nil || got.ID != "c1" {
		t.Fatalf("Acquire should return c1 last, got %+v", got)
	}
	if got := p.Acquire("k"); got != nil {
		t.Fatalf("Acquire on drained bucket should return nil, got %+v", got)
	}

	if len(*calls) != 3 {
		t.Fatalf("expected 3 reset calls, got %v", *calls)
	}
}

// TestPoolGlobalCapEvictsOldest verifies that when the global cap is hit,
// the oldest LRU entry (across all buckets) is evicted on the next Release,
// not the new entry being parked. This is the property that prevents the
// container-count blowup observed in CI run 24930111125.
func TestPoolGlobalCapEvictsOldest(t *testing.T) {
	p, be, _ := poolWithStubReset(t, 2, nil)

	// Park 2 entries to fill the global cap.
	if !p.Release(testEntry("c1", "10.0.0.1"), "k1") {
		t.Fatal("first release must succeed")
	}
	if !p.Release(testEntry("c2", "10.0.0.2"), "k2") {
		t.Fatal("second release must succeed under cap")
	}
	if got := len(be.deletes); got != 0 {
		t.Fatalf("no evictions yet, got %d deletes", got)
	}

	// Park a 3rd: evicts c1 (oldest in LRU) to make room.
	if !p.Release(testEntry("c3", "10.0.0.3"), "k3") {
		t.Fatal("Release should succeed by evicting oldest")
	}
	waitForEvictions(p)
	if got := be.deletes; len(got) != 1 || got[0] != "c1" {
		t.Fatalf("expected c1 to be evicted, got %v", got)
	}

	// c1's bucket k1 should now be empty.
	if got := p.Acquire("k1"); got != nil {
		t.Fatalf("k1 bucket should be empty after eviction, got %+v", got)
	}
	// c2 and c3 are still parked.
	if got := p.Acquire("k2"); got == nil || got.ID != "c2" {
		t.Fatalf("k2 should still hold c2, got %+v", got)
	}
	if got := p.Acquire("k3"); got == nil || got.ID != "c3" {
		t.Fatalf("k3 should still hold c3, got %+v", got)
	}
}

// TestPoolAcquireRefreshesLRU verifies that Acquire-ing an entry and
// re-Releasing it makes it the *newest* in the LRU, so future evictions
// favour entries that haven't been touched recently.
func TestPoolAcquireRefreshesLRU(t *testing.T) {
	p, be, _ := poolWithStubReset(t, 2, nil)

	p.Release(testEntry("c1", "10.0.0.1"), "k1")
	p.Release(testEntry("c2", "10.0.0.2"), "k2")

	// Acquire and re-Release c1 — that pushes it to the back of the LRU.
	got := p.Acquire("k1")
	if got == nil || got.ID != "c1" {
		t.Fatalf("Acquire k1 should return c1, got %+v", got)
	}
	if !p.Release(*got, "k1") {
		t.Fatal("Re-release of c1 should succeed")
	}

	// Park c3: evict the *new* oldest, which is now c2 (not c1).
	p.Release(testEntry("c3", "10.0.0.3"), "k3")
	waitForEvictions(p)
	if got := be.deletes; len(got) != 1 || got[0] != "c2" {
		t.Fatalf("expected c2 to be evicted (older after c1 was refreshed), got %v", got)
	}
}

func TestPoolReleaseFailsOnResetError(t *testing.T) {
	p, _, _ := poolWithStubReset(t, 4, errStubResetFailed)
	if p.Release(testEntry("c1", "10.0.0.1"), "k") {
		t.Fatal("Release should fail when reset returns an error")
	}
	if got := p.Acquire("k"); got != nil {
		t.Fatalf("nothing should have been parked; got %+v", got)
	}
}

// TestPoolReleaseRequiresResetPort guards the contract that callers must
// populate ResetPort. A zero port would otherwise produce a malformed
// reset URL and silently 0-port-fail at runtime.
func TestPoolReleaseRequiresResetPort(t *testing.T) {
	p, _, calls := poolWithStubReset(t, 4, nil)
	if p.Release(PoolEntry{ID: "c1", IP: "10.0.0.1" /* ResetPort omitted */}, "k") {
		t.Fatal("Release with ResetPort=0 should be rejected")
	}
	if len(*calls) != 0 {
		t.Fatalf("reset stub should not have been called, got %v", *calls)
	}
}

// TestPoolReleasePassesResetPort verifies the cold-path-supplied port
// reaches the reset RPC, so HIVE_CHECK_LIVE_PORT overrides take effect.
func TestPoolReleasePassesResetPort(t *testing.T) {
	p, _, calls := poolWithStubReset(t, 4, nil)
	entry := PoolEntry{ID: "c1", IP: "10.0.0.1", ResetPort: 9999}
	if !p.Release(entry, "k") {
		t.Fatal("Release should succeed")
	}
	if len(*calls) != 1 || (*calls)[0].port != 9999 || (*calls)[0].ip != "10.0.0.1" {
		t.Fatalf("reset called with wrong args: %+v", *calls)
	}
}

// TestPoolPreservesLogFile verifies that the LogFile populated at Release
// time round-trips through Acquire unchanged. api.startClient relies on
// this to direct subsequent tests' offsets at the same file the docker
// attach goroutine actually writes to.
func TestPoolPreservesLogFile(t *testing.T) {
	p, _, _ := poolWithStubReset(t, 4, nil)
	entry := PoolEntry{
		ID:        "c1",
		IP:        "10.0.0.1",
		LogFile:   "geth/client-deadbeef-full-id.log",
		ResetPort: 8545,
	}
	if !p.Release(entry, "k") {
		t.Fatal("Release should succeed")
	}
	got := p.Acquire("k")
	if got == nil {
		t.Fatal("Acquire should return the parked entry")
	}
	if got.LogFile != entry.LogFile {
		t.Fatalf("LogFile not preserved across pool: got %q, want %q", got.LogFile, entry.LogFile)
	}
	if got.ResetPort != entry.ResetPort {
		t.Fatalf("ResetPort not preserved: got %d, want %d", got.ResetPort, entry.ResetPort)
	}
}

func TestPoolDrain(t *testing.T) {
	p, be, _ := poolWithStubReset(t, 4, nil)

	p.Release(testEntry("c1", "10.0.0.1"), "k1")
	p.Release(testEntry("c2", "10.0.0.2"), "k2")
	p.Release(testEntry("c3", "10.0.0.3"), "k1")

	p.Drain(context.Background())

	if len(be.deletes) != 3 {
		t.Fatalf("Drain should delete all retained, got %d", len(be.deletes))
	}
	// After drain, Acquire is a no-op.
	if got := p.Acquire("k1"); got != nil {
		t.Fatalf("Acquire after Drain should return nil, got %+v", got)
	}
	if p.Release(testEntry("c4", "10.0.0.4"), "k1") {
		t.Fatal("Release after Drain should fail")
	}
}

var errStubResetFailed = errStubReset{}

type errStubReset struct{}

func (errStubReset) Error() string { return "stub reset failed" }

func TestComputePoolKeyDeterministic(t *testing.T) {
	env1 := map[string]string{"HIVE_CHAIN_ID": "1", "HIVE_FORK_HOMESTEAD": "0"}
	env2 := map[string]string{"HIVE_FORK_HOMESTEAD": "0", "HIVE_CHAIN_ID": "1"} // same set, diff iteration

	files1 := buildMultipartFiles(t, map[string]string{"/genesis.json": `{"alloc":{}}`})
	files2 := buildMultipartFiles(t, map[string]string{"/genesis.json": `{"alloc":{}}`})

	k1, err := ComputePoolKey("img:tag", env1, files1, nil)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := ComputePoolKey("img:tag", env2, files2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if k1 != k2 {
		t.Fatalf("key should not depend on env iteration order: %q vs %q", k1, k2)
	}

	// Different genesis -> different key.
	otherFiles := buildMultipartFiles(t, map[string]string{
		"/genesis.json": `{"alloc":{"0x00":{"balance":"0x1"}}}`,
	})
	k3, err := ComputePoolKey("img:tag", env1, otherFiles, nil)
	if err != nil {
		t.Fatal(err)
	}
	if k1 == k3 {
		t.Fatal("key should change with genesis bytes")
	}

	// Different image -> different key.
	k4, err := ComputePoolKey("img:other", env1, buildMultipartFiles(t, map[string]string{"/genesis.json": `{"alloc":{}}`}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if k1 == k4 {
		t.Fatal("key should change with image")
	}
}

// TestComputePoolKeyNetworks verifies that the network set participates in
// the key. Two tests with identical image/env/files but different requested
// networks are NOT interchangeable: the warm path skips ConnectContainer,
// so reusing across network sets would silently leave the container off
// the network the new test asked for.
func TestComputePoolKeyNetworks(t *testing.T) {
	env := map[string]string{"HIVE_CHAIN_ID": "1"}
	files := buildMultipartFiles(t, map[string]string{"/genesis.json": `{}`})

	files2 := buildMultipartFiles(t, map[string]string{"/genesis.json": `{}`})
	files3 := buildMultipartFiles(t, map[string]string{"/genesis.json": `{}`})
	files4 := buildMultipartFiles(t, map[string]string{"/genesis.json": `{}`})

	noNet, err := ComputePoolKey("img", env, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	withNetA, err := ComputePoolKey("img", env, files2, []string{"netA"})
	if err != nil {
		t.Fatal(err)
	}
	withNetB, err := ComputePoolKey("img", env, files3, []string{"netB"})
	if err != nil {
		t.Fatal(err)
	}
	if noNet == withNetA || withNetA == withNetB {
		t.Fatalf("network set must affect key: noNet=%s netA=%s netB=%s", noNet, withNetA, withNetB)
	}

	// Order-independence within the network set.
	ab1, err := ComputePoolKey("img", env, files4, []string{"netA", "netB"})
	if err != nil {
		t.Fatal(err)
	}
	ab2, err := ComputePoolKey("img", env, buildMultipartFiles(t, map[string]string{"/genesis.json": `{}`}), []string{"netB", "netA"})
	if err != nil {
		t.Fatal(err)
	}
	if ab1 != ab2 {
		t.Fatalf("network order should not affect key: %q vs %q", ab1, ab2)
	}
}

func TestComputePoolKeyEmpty(t *testing.T) {
	k, err := ComputePoolKey("img", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if k == "" {
		t.Fatal("key should be non-empty even with no env/files")
	}
}

// buildMultipartFiles constructs real *multipart.FileHeader values by writing
// to a multipart.Writer and parsing it back. This mirrors what
// http.Request.ParseMultipartForm produces in api.startClient.
func buildMultipartFiles(t *testing.T, contents map[string]string) map[string]*multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for name, content := range contents {
		fw, err := mw.CreateFormFile(name, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	mr := multipart.NewReader(strings.NewReader(body.String()), mw.Boundary())
	form, err := mr.ReadForm(int64(body.Len()) + 1024)
	if err != nil {
		t.Fatal(err)
	}

	out := make(map[string]*multipart.FileHeader, len(contents))
	for k, v := range form.File {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}
