package api

import (
	"context"
	"sync"
	"testing"
)

func TestRegistry_RegisterAndCancel(t *testing.T) {
	r := NewSessionRegistry()
	called := false
	ok := r.Register("s1", func() { called = true })
	if !ok {
		t.Fatal("Register should return true on first insert")
	}
	if !r.IsBusy("s1") {
		t.Error("IsBusy should be true after Register")
	}
	cancelled := r.Cancel("s1")
	if !cancelled {
		t.Error("Cancel should return true for known id")
	}
	if !called {
		t.Error("Cancel should invoke the stored func")
	}
	if r.IsBusy("s1") {
		t.Error("IsBusy should be false after Cancel")
	}
	if r.Cancel("s1") {
		t.Error("second Cancel should return false")
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewSessionRegistry()
	r.Register("s1", func() {})
	if r.Register("s1", func() {}) {
		t.Error("second Register for same id should return false")
	}
}

func TestRegistry_Clear(t *testing.T) {
	r := NewSessionRegistry()
	r.Register("s1", func() {})
	r.Clear("s1")
	if r.IsBusy("s1") {
		t.Error("IsBusy should be false after Clear")
	}
}

func TestRegistry_Concurrent(t *testing.T) {
	r := NewSessionRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('a' + (i % 26))) + "-" + string(rune('0'+(i%10)))
			r.Register(id, func() {})
			r.Clear(id)
		}(i)
	}
	wg.Wait()
	_ = context.Background()
}
