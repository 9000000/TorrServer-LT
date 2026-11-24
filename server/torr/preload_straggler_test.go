package torr

import (
	"reflect"
	"testing"

	storageState "server/torr/storage/state"
)

func TestPreloadStragglerPieces(t *testing.T) {
	head := []int{0, 1, 2, 3}
	probeHead := []int{0, 1}
	tail := []int{1231}

	t.Run("no escalation before any piece finishes", func(t *testing.T) {
		snap := map[int]storageState.ItemState{
			0: {Id: 0, Size: 300 << 10, Completed: false},
		}
		if got := preloadStragglerPieces(head, probeHead, tail, true, false, snap); got != nil {
			t.Fatalf("expected nil at done==0, got %v", got)
		}
		if got := preloadStragglerPieces(head, probeHead, tail, false, false, snap); got != nil {
			t.Fatalf("expected nil at done==0 (single-phase), got %v", got)
		}
	})

	t.Run("two-phase scopes to probe pieces and keeps tail first", func(t *testing.T) {
		snap := map[int]storageState.ItemState{
			0: {Id: 0, Completed: true},
		}
		got := preloadStragglerPieces(head, probeHead, tail, true, false, snap)
		want := []int{1231, 1}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("twoPhase before probe: got %v, want %v", got, want)
		}
	})

	t.Run("after probe escalates remaining phase-2 pieces in order", func(t *testing.T) {
		snap := map[int]storageState.ItemState{
			0:    {Id: 0, Completed: true},
			1:    {Id: 1, Completed: true},
			1231: {Id: 1231, Completed: true},
		}
		got := preloadStragglerPieces(head, probeHead, tail, true, true, snap)
		want := []int{2, 3}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("twoPhase after probe: got %v, want %v", got, want)
		}
	})

	t.Run("does not escalate when more than stragglerEscalatePieces remain", func(t *testing.T) {
		bigHead := make([]int, 16)
		for i := range bigHead {
			bigHead[i] = i
		}
		snap := map[int]storageState.ItemState{
			0: {Id: 0, Completed: true},
		}
		if got := preloadStragglerPieces(bigHead, bigHead[:2], tail, false, false, snap); got != nil {
			t.Fatalf("expected nil when >%d pieces remain, got %v", stragglerEscalatePieces, got)
		}
	})
}
