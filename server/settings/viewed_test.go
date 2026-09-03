package settings

import (
	"encoding/json"
	"testing"
)

func TestReadIndexes(t *testing.T) {
	// Test empty buffer
	m := readIndexes(nil)
	if len(m) != 0 {
		t.Fatalf("expected empty map for nil buffer, got %v", m)
	}

	// Test legacy map[int]struct{} format
	legacyData := map[int]struct{}{
		1: {},
		3: {},
	}
	legacyBuf, err := json.Marshal(legacyData)
	if err != nil {
		t.Fatalf("failed to marshal legacy data: %v", err)
	}
	mLegacy := readIndexes(legacyBuf)
	if len(mLegacy) != 2 {
		t.Fatalf("expected 2 entries for legacy data, got %d", len(mLegacy))
	}
	if mLegacy[1] != 0 || mLegacy[3] != 0 {
		t.Fatalf("expected legacy timecode 0, got %v", mLegacy)
	}

	// Test new map[int]float64 format with timecode
	newData := map[int]float64{
		2: 1542.5,
		5: 3600.0,
	}
	newBuf, err := json.Marshal(newData)
	if err != nil {
		t.Fatalf("failed to marshal new data: %v", err)
	}
	mNew := readIndexes(newBuf)
	if len(mNew) != 2 {
		t.Fatalf("expected 2 entries for new data, got %d", len(mNew))
	}
	if mNew[2] != 1542.5 || mNew[5] != 3600.0 {
		t.Fatalf("expected timecode values preserved, got %v", mNew)
	}
}
