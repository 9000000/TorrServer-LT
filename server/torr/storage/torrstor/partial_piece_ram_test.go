package torrstor

import (
	"bytes"
	"testing"
	"time"

	"server/settings"
)

func TestPartialPieceRAMAccounting(t *testing.T) {
	old := settings.BTsets()
	settings.StoreBTsets(&settings.BTSets{CacheSize: 96 << 20})
	t.Cleanup(func() { settings.StoreBTsets(old) })

	s := NewStorage()
	h := mkHash(0x95)
	const pieceLength = 8 << 20
	const numPieces = 20
	s.callbackOpen(95, h, numPieces, pieceLength)
	c := s.CacheByHash(h)
	if c == nil {
		t.Fatal("cache not registered")
	}
	t.Cleanup(func() { s.callbackClose(95) })

	// Populate 8 complete pieces (64 MiB), then touch 8 incomplete pieces with
	// a single 16 KiB block each (another 64 MiB of allocated MemPiece slices,
	// pushing true RAM usage to 128 MiB > 96 MiB CacheSize even though logical
	// Filled() is only ~64.1 MiB).
	full := bytes.Repeat([]byte{0x41}, pieceLength)
	for i := 0; i < 8; i++ {
		if _, err := c.writePiece(i, 0, full); err != nil {
			t.Fatal(err)
		}
		c.SignalPieceComplete(i)
	}
	block := bytes.Repeat([]byte{0x5a}, pieceBlockSize)
	for i := 8; i < 16; i++ {
		if _, err := c.writePiece(i, 0, block); err != nil {
			t.Fatal(err)
		}
	}

	// Run a deterministic eviction pass and verify allocated RAM stays within budget.
	c.evictPass()

	used := c.memUsed()
	capBytes := c.capacity()
	if used > capBytes {
		t.Fatalf("eviction left %d MiB allocated in RAM, exceeding %d MiB capacity",
			used>>20, capBytes>>20)
	}
}

func TestEvictPass_KeepsRecentIncompletePieceUnderBudget(t *testing.T) {
	old := settings.BTsets()
	settings.StoreBTsets(&settings.BTSets{CacheSize: 512 << 20})
	t.Cleanup(func() { settings.StoreBTsets(old) })

	s := NewStorage()
	h := mkHash(0x96)
	const pieceLength = 8 << 20
	s.callbackOpen(96, h, 20, pieceLength)
	c := s.CacheByHash(h)
	t.Cleanup(func() { s.callbackClose(96) })

	block := bytes.Repeat([]byte{0x5a}, pieceBlockSize)
	if _, err := c.writePiece(3, 0, block); err != nil {
		t.Fatal(err)
	}

	// Simulate 10s idle (> abandonEvictSec=4s, < hashGraceSec=30s) while well
	// under CacheSize (8 MiB << 512 MiB): incomplete piece must NOT be wiped,
	// because libtorrent still tracks its partial blocks in piece_picker.
	c.mu.RLock()
	p := c.pieces[3]
	c.mu.RUnlock()
	p.accessed.Store(time.Now().Unix() - (abandonEvictSec + 6))

	c.evictPass()

	c.mu.RLock()
	_, stillThere := c.pieces[3]
	c.mu.RUnlock()
	if !stillThere {
		t.Fatal("evictPass wiped a 10s-old incomplete piece while well under CacheSize")
	}

	// Once idle past hashGraceSec (30s), genuinely abandoned partial is reaped.
	p.accessed.Store(time.Now().Unix() - (hashGraceSec + 5))
	c.evictPass()

	c.mu.RLock()
	_, afterGrace := c.pieces[3]
	c.mu.RUnlock()
	if afterGrace {
		t.Fatal("evictPass should reap an incomplete piece idle past hashGraceSec")
	}
}
