package torr

import (
	"io"
	"sync"
	"testing"
)

// A settings save disconnects the engine and connects it again. Everything
// that reads the session pointer must do so under bt.mu, or the read races
// with that swap — and a nil session reached through a stale check panics
// inside libtorrent instead of being reported.
func TestSessionReadsAreSynchronised(t *testing.T) {
	bt := NewBTS()
	old := bts
	InitApiHelper(bt)
	t.Cleanup(func() { bts = old })

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Stand in for Connect/Disconnect: they replace bt.session while holding
	// bt.mu, which only orders against readers that take the lock too. The
	// value stays nil so no libtorrent call is ever made from here.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			bt.mu.Lock()
			bt.session = nil
			bt.mu.Unlock()
			InitApiHelper(bt)
		}
		close(stop)
	}()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				WriteStatus(io.Discard)
				if _, err := NewTorrent(&TorrentSpec{}, bt); err == nil {
					t.Error("NewTorrent without a session must fail, not succeed")
					return
				}
				if s := bt.Session(); s != nil {
					t.Error("Session() returned a session that was never created")
					return
				}
			}
		}()
	}
	wg.Wait()
}
