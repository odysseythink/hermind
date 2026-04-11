package gateway

import (
	"context"
	"testing"
	"time"
)

func TestHooksRunPreDrops(t *testing.T) {
	h := NewHooks()
	h.AddPre(func(ctx context.Context, in *IncomingMessage) (*IncomingMessage, error) {
		return nil, nil // drop
	})
	out, err := h.RunPre(context.Background(), IncomingMessage{Text: "hi"})
	if err != nil || out != nil {
		t.Errorf("expected drop, got out=%v err=%v", out, err)
	}
}

func TestHooksRunPostMutates(t *testing.T) {
	h := NewHooks()
	h.AddPost(func(ctx context.Context, in IncomingMessage, out *OutgoingMessage) (*OutgoingMessage, error) {
		out.Text = "[mod] " + out.Text
		return out, nil
	})
	in := IncomingMessage{Text: "hi"}
	out := OutgoingMessage{Text: "hello"}
	res, err := h.RunPost(context.Background(), in, out)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.Text != "[mod] hello" {
		t.Errorf("unexpected: %+v", res)
	}
}

func TestRateLimitHookDrops(t *testing.T) {
	hook := RateLimitHook(100 * time.Millisecond)
	in := &IncomingMessage{Platform: "fake", UserID: "u1", Text: "hi"}
	if out, err := hook(context.Background(), in); err != nil || out == nil {
		t.Errorf("first call should pass: out=%v err=%v", out, err)
	}
	if out, err := hook(context.Background(), in); err != nil || out != nil {
		t.Errorf("second call should drop: out=%v err=%v", out, err)
	}
}

func TestUserBanHook(t *testing.T) {
	hook := UserBanHook([]string{"bad_user"})
	_, err := hook(context.Background(), &IncomingMessage{UserID: "bad_user"})
	if err == nil {
		t.Error("expected ban error")
	}
}

func TestCommandAllowlistHook(t *testing.T) {
	hook := CommandAllowlistHook([]string{"/start", "/help"})
	if out, _ := hook(context.Background(), &IncomingMessage{Text: "/start"}); out == nil {
		t.Error("/start should pass")
	}
	if out, _ := hook(context.Background(), &IncomingMessage{Text: "hi"}); out != nil {
		t.Error("non-allowlisted should drop")
	}
}

func TestProfanityRedactHook(t *testing.T) {
	hook := ProfanityRedactHook([]string{"bad"})
	out := &OutgoingMessage{Text: "this is bad"}
	res, _ := hook(context.Background(), IncomingMessage{}, out)
	if res.Text != "this is ***" {
		t.Errorf("redact = %q", res.Text)
	}
}

func TestChannelDirectory(t *testing.T) {
	d := NewChannelDirectory()
	d.Upsert(Channel{Platform: "slack", ID: "C1", Name: "general", Active: true})
	d.Upsert(Channel{Platform: "slack", ID: "C2", Name: "random"})
	all := d.All()
	if len(all) != 2 {
		t.Errorf("len = %d", len(all))
	}
	d.Remove("slack", "C1")
	if len(d.All()) != 1 {
		t.Error("remove failed")
	}
}

func TestPairingCreateRedeem(t *testing.T) {
	p := NewPairing(0)
	tok := p.Create("alice")
	prof, err := p.Redeem(tok, "tg", "123")
	if err != nil {
		t.Fatal(err)
	}
	if prof != "alice" {
		t.Errorf("profile = %q", prof)
	}
	if p.Lookup("tg", "123") != "alice" {
		t.Error("lookup failed")
	}
	// Token is single-use.
	if _, err := p.Redeem(tok, "tg", "456"); err == nil {
		t.Error("token should be consumed")
	}
}

func TestPairingExpiry(t *testing.T) {
	p := NewPairing(1 * time.Millisecond)
	tok := p.Create("bob")
	time.Sleep(5 * time.Millisecond)
	if _, err := p.Redeem(tok, "tg", "1"); err == nil {
		t.Error("expected expiry error")
	}
}

func TestStickerCachePutAndGet(t *testing.T) {
	sc := NewStickerCache()
	h := sc.Put([]byte("hello"), "text/plain")
	if sc.Len() != 1 {
		t.Error("expected len 1")
	}
	// Second put of the same payload is a dedup.
	sc.Put([]byte("hello"), "text/plain")
	if sc.Len() != 1 {
		t.Error("expected dedup")
	}
	if !sc.Has(h) {
		t.Error("has failed")
	}
	data, meta, ok := sc.Get(h)
	if !ok || string(data) != "hello" || meta != "text/plain" {
		t.Errorf("get: %v %v %v", ok, data, meta)
	}
}
