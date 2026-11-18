package torrstor

import (
	"bytes"
	"testing"

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
