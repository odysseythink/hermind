package meta

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/tool"
)

func TestTodoAddListDone(t *testing.T) {
	list := NewTodoList()
	reg := tool.NewRegistry()
	RegisterTodo(reg, list)

	addArgs, _ := json.Marshal(map[string]string{"text": "ship 7b"})
	addRes, _ := reg.Dispatch(context.Background(), "todo_add", addArgs)
	if !strings.Contains(addRes, `"id":1`) {
		t.Errorf("add: %s", addRes)
	}
	listRes, _ := reg.Dispatch(context.Background(), "todo_list", json.RawMessage(`{}`))
	if !strings.Contains(listRes, "ship 7b") {
		t.Errorf("list: %s", listRes)
	}
	doneArgs, _ := json.Marshal(map[string]int{"id": 1})
	doneRes, _ := reg.Dispatch(context.Background(), "todo_done", doneArgs)
	if !strings.Contains(doneRes, `"ok":true`) {
		t.Errorf("done: %s", doneRes)
	}
	if !list.All()[0].Done {
		t.Error("expected done")
	}
}

func TestClarifyReturnsPending(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterClarify(reg)
	args, _ := json.Marshal(map[string]string{"question": "which db?"})
	res, _ := reg.Dispatch(context.Background(), "clarify", args)
	if !strings.Contains(res, `"clarify_pending":true`) {
		t.Errorf("expected clarify_pending: %s", res)
	}
}

func TestCheckpointSaveAndRestore(t *testing.T) {
	t.Setenv("HERMIND_HOME", t.TempDir())
	reg := tool.NewRegistry()
	RegisterCheckpoint(reg)
	saveArgs, _ := json.Marshal(map[string]any{
		"name":  "snap1",
		"state": map[string]any{"k": "v"},
	})
	if _, err := reg.Dispatch(context.Background(), "checkpoint_save", saveArgs); err != nil {
		t.Fatal(err)
	}
	restoreArgs, _ := json.Marshal(map[string]string{"name": "snap1"})
	res, err := reg.Dispatch(context.Background(), "checkpoint_restore", restoreArgs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, `"k":"v"`) {
		t.Errorf("expected restored state: %s", res)
	}
}

func TestApprovalPending(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterApproval(reg)
	args, _ := json.Marshal(map[string]string{"action": "rm -rf /", "reason": "cleanup"})
	res, _ := reg.Dispatch(context.Background(), "approval_request", args)
	if !strings.Contains(res, `"approval_pending":true`) {
		t.Errorf("expected pending: %s", res)
	}
}
