package hiveproxy

import (
	"context"
	"errors"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

type proxyFunctions struct {
	mu     sync.Mutex
	cancel map[uint64]context.CancelFunc
}

func (pfn *proxyFunctions) CheckLive(ctx context.Context, id uint64, addr string) error {
	ctx, cancel := pfn.makeContext(ctx, id)
	defer cancel()

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if net.ParseIP(host) == nil {
		return errors.New("invalid IP")
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return errors.New("invalid port")
	}

	var (
		start   = time.Now()
		lastMsg time.Time
		timer   = time.NewTimer(checkLiveDelay)
		dialer  net.Dialer
	)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return errors.New("canceled")
		case <-timer.C:
			if time.Since(lastMsg) >= time.Second {
				log.Println("checking address:", addr)
				lastMsg = time.Now()
			}
			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err == nil {
				conn.Close()
				log.Printf("address %s live after %v", addr, time.Since(start).Round(time.Millisecond))
				return nil
			}
			timer.Reset(checkLiveInterval(time.Since(start)))
		}
	}
}

// checkLiveDelay is the wait before the first dial. No client opens its RPC port
// this quickly, so dialing earlier would only add refused connections.
const checkLiveDelay = 50 * time.Millisecond

// checkLiveInterval returns the delay before the next dial, given the time spent
// waiting so far. Most clients open their RPC port within a few hundred
// milliseconds, so dial often at first to avoid inflating their startup time,
// and back off in steps for slower clients. The interval never exceeds 100ms.
func checkLiveInterval(elapsed time.Duration) time.Duration {
	switch {
	case elapsed < 200*time.Millisecond:
		return 5 * time.Millisecond
	case elapsed < 1500*time.Millisecond:
		return 10 * time.Millisecond
	case elapsed < 4500*time.Millisecond:
		return 50 * time.Millisecond
	default:
		return 100 * time.Millisecond
	}
}

func (pfn *proxyFunctions) makeContext(baseCtx context.Context, id uint64) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(baseCtx)

	pfn.mu.Lock()
	defer pfn.mu.Unlock()
	if pfn.cancel == nil {
		pfn.cancel = make(map[uint64]context.CancelFunc)
	}
	if pfn.cancel[id] != nil {
		panic("duplicate id")
	}
	pfn.cancel[id] = cancel

	cf := func() {
		pfn.mu.Lock()
		defer pfn.mu.Unlock()
		cancel()
		delete(pfn.cancel, id)
	}
	return ctx, cf
}

func (pfn *proxyFunctions) Cancel(id uint64) {
	pfn.mu.Lock()
	defer pfn.mu.Unlock()

	cancel := pfn.cancel[id]
	if cancel != nil {
		cancel()
		delete(pfn.cancel, id)
	}
}
