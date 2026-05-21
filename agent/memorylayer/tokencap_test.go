package memorylayer

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestTokenCap_PerTurnBlocks(t *testing.T) {
	tc := NewTokenCap(100, 10000)
	if !tc.Allow(60) {
		t.Fatal("expected Allow(60) to succeed")
	}
	if tc.Allow(60) {
		t.Fatal("expected Allow(60) to be blocked (would be 120 > 100)")
	}
	if !tc.Allow(40) {
		t.Fatal("expected Allow(40) to succeed (turn total would be exactly 100)")
	}
}

func TestTokenCap_PerSessionBlocksAcrossTurns(t *testing.T) {
	tc := NewTokenCap(100, 150)
	tc.ResetTurn()
	if !tc.Allow(80) {
		t.Fatal("expected turn 1 Allow(80) to succeed")
	}
	tc.ResetTurn()
	if tc.Allow(80) {
		t.Fatal("expected turn 2 Allow(80) to be blocked (session total would be 160 > 150)")
	}
	if !tc.Allow(70) {
		t.Fatal("expected turn 2 Allow(70) to succeed (session total would be exactly 150)")
	}
}

func TestTokenCap_ResetTurnDoesNotResetSession(t *testing.T) {
	tc := NewTokenCap(100, 10000)
	tc.ResetTurn()
	tc.Allow(30)
	tc.ResetTurn()
	tc.Allow(20)
	tc.ResetTurn()
	tc.Allow(10)
	if tc.SessionUsed() != 60 {
		t.Fatalf("expected session used 60, got %d", tc.SessionUsed())
	}
}

func TestTokenCap_ZeroCapsMeansUnlimited(t *testing.T) {
	tc := NewTokenCap(0, 0)
	if !tc.Allow(999999) {
		t.Fatal("expected unlimited cap to allow anything")
	}
	if !tc.Allow(999999) {
		t.Fatal("expected unlimited cap to allow anything repeatedly")
	}
}

func TestTokenCap_ConcurrentAllow(t *testing.T) {
	tc := NewTokenCap(0, 500)
	var wg sync.WaitGroup
	okCount := atomic.Int32{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tc.Allow(10) {
				okCount.Add(1)
			}
		}()
	}
	wg.Wait()
	if okCount.Load() != 50 {
		t.Fatalf("expected exactly 50 successes with perSession=500 and cost=10, got %d", okCount.Load())
	}
}
