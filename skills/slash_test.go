package skills

import (
	"context"
	"strings"
	"testing"
)

func TestSlashDispatch(t *testing.T) {
	reg := NewSlashRegistry()
	reg.Register(&SlashCommand{
		Name:        "echo",
		Description: "echo args",
		Handler: func(ctx context.Context, args []string) (string, error) {
			return strings.Join(args, " "), nil
		},
	})
	reply, handled, err := reg.Dispatch(context.Background(), "/echo hello world")
	if err != nil || !handled {
		t.Fatalf("err=%v handled=%v", err, handled)
	}
	if reply != "hello world" {
		t.Errorf("reply = %q", reply)
	}
}

func TestSlashNonCommand(t *testing.T) {
	reg := NewSlashRegistry()
	_, handled, err := reg.Dispatch(context.Background(), "plain message")
	if handled {
		t.Error("expected not handled")
	}
	if err != nil {
		t.Error(err)
	}
}

func TestSlashUnknown(t *testing.T) {
	reg := NewSlashRegistry()
	_, handled, err := reg.Dispatch(context.Background(), "/whatever")
	if !handled {
		t.Error("expected handled even on unknown")
	}
	if err == nil {
		t.Error("expected error on unknown")
	}
}

func TestSlashAllSorted(t *testing.T) {
	reg := NewSlashRegistry()
	reg.Register(&SlashCommand{Name: "b"})
	reg.Register(&SlashCommand{Name: "a"})
	reg.Register(&SlashCommand{Name: "c"})
	got := reg.All()
	if len(got) != 3 || got[0].Name != "a" || got[2].Name != "c" {
		t.Errorf("sort wrong: %+v", got)
	}
}
