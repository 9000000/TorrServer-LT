package torrstor

import "io"

// MemPiece keeps a piece's bytes in a single heap-allocated slice. The
// slice is lazily allocated on first write (so empty pieces don't cost
// PieceLength bytes of RAM).
type MemPiece struct {
	piece *Piece
	buf   []byte
}

func newMemPiece(p *Piece) *MemPiece {
	return &MemPiece{piece: p}
}

func (m *MemPiece) WriteAt(b []byte, off int64) (int, error) {
	if off < 0 {
		return 0, io.ErrShortWrite
	}
	pl := m.piece.cache.PieceLength
	if pl <= 0 {
		return 0, io.ErrShortWrite
	}
	if m.buf == nil {
		m.buf = make([]byte, pl)
	}
	if off >= int64(len(m.buf)) {
		return 0, io.ErrShortWrite
	}
	n := copy(m.buf[off:], b)
	return n, nil
}

func (m *MemPiece) ReadAt(b []byte, off int64) (int, error) {
	if m.buf == nil || off < 0 || off >= int64(len(m.buf)) {
		return 0, io.EOF
	}
	n := copy(b, m.buf[off:])
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

// Allocated returns the RAM held by the backing buffer. The whole piece is
// allocated on the first write, so this can be much larger than the payload
// bytes received so far (upstream #955). Caller must hold piece.mu.
func (m *MemPiece) Allocated() int64 {
	return int64(cap(m.buf))
}

// Release drops the backing buffer (the next write re-allocates).
func (m *MemPiece) Release() {
	m.buf = nil
}
