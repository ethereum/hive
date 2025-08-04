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
		lastMsg time.Time
		ticker  = time.NewTicker(100 * time.Millisecond)
		dialer  net.Dialer
	)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return errors.New("canceled")
		case <-ticker.C:
			if time.Since(lastMsg) >= time.Second {
				log.Println("checking address:", addr)
				lastMsg = time.Now()
			}
			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err == nil {
				conn.Close()
				return nil
			}
		}
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
