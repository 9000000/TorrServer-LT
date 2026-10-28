package torrstor

import (
	"bytes"
	"testing"
)

// TestCache_WriteAfterCloseDoesNotPanic guards the straggler-block path: a
// libtorrent disk write can land after the torrent (and its storage) was
// removed. It must be acknowledged and dropped, not panic on a nil piece map.
func TestCache_WriteAfterCloseDoesNotPanic(t *testing.T) {
	s := NewStorage()
	h := mkHash(0xE1)
	s.callbackOpen(1, h, 4, pieceLen)
	c := s.CacheByHash(h)

	c.close()

	payload := bytes.Repeat([]byte{0x42}, 1024)
	n, err := c.writePiece(0, 0, payload)
	if err != nil {
		t.Fatalf("write after close: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("write after close: n=%d, want %d", n, len(payload))
	}

	c.mu.RLock()
	resident := len(c.pieces)
	c.mu.RUnlock()
	if resident != 0 {
		t.Fatalf("closed cache materialised %d pieces", resident)
	}

	if _, err := c.readPiece(0, 0, make([]byte, 16)); err == nil {
		t.Fatal("read after close should fail, not serve stale bytes")
	}
}

// TestCache_SignalHashFailedUsesNumPiecesBound guards the bound check: the
// pieces map only holds the materialised streaming window, so bounding by
// len(c.pieces) skipped the wipe for every piece outside that window and left
// libtorrent-rejected bytes in the cache.
func TestCache_SignalHashFailedUsesNumPiecesBound(t *testing.T) {
	s := NewStorage()
	h := mkHash(0xE2)
	s.callbackOpen(1, h, 8, pieceLen)
	c := s.CacheByHash(h)

	// Only piece 3 is materialised, so len(c.pieces) == 1 while NumPieces == 8.
	payload := bytes.Repeat([]byte{0x5A}, int(pieceLen))
	if _, err := c.writePiece(3, 0, payload); err != nil {
		t.Fatalf("write piece 3: %v", err)
	}
	c.SignalPieceComplete(3)
	if !c.Have(3) {
		t.Fatal("precondition: piece 3 should be complete")
	}

	c.SignalHashFailed(3)

	if c.Have(3) {
		t.Fatal("SignalHashFailed did not wipe piece 3 (bound compared against len(pieces))")
	}
}
