package batch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDataset_BasicShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.jsonl")
	lines := "" +
		`{"id":"q1","prompt":"what is 2+2?"}` + "\n" +
		`{"prompt":"no id here"}` + "\n" +
		`{"id":"q3","prompt":"third"}` + "\n"
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := ReadDataset(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("len = %d", len(items))
	}
	if items[0].ID != "q1" || items[0].Prompt != "what is 2+2?" {
		t.Errorf("items[0] = %#v", items[0])
	}
	if items[1].ID != "line-2" {
		t.Errorf("items[1].ID = %q", items[1].ID)
	}
	if len(items[2].Raw) == 0 {
		t.Errorf("expected raw bytes preserved")
	}
}

func TestReadDataset_MaxItems(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.jsonl")
	lines := `{"id":"a","prompt":"a"}` + "\n" +
		`{"id":"b","prompt":"b"}` + "\n" +
		`{"id":"c","prompt":"c"}` + "\n"
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := ReadDataset(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("len = %d", len(items))
	}
}

func TestReadDataset_SkipsBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.jsonl")
	lines := `{"id":"a","prompt":"a"}` + "\n\n" +
		`   ` + "\n" +
		`{"id":"b","prompt":"b"}` + "\n"
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := ReadDataset(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("len = %d", len(items))
	}
}

func TestReadDataset_RejectsMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.jsonl")
	if err := os.WriteFile(path, []byte(`not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDataset(path, 0); err == nil {
		t.Error("expected parse error")
	}
}

func TestReadDataset_MissingFile(t *testing.T) {
	if _, err := ReadDataset(filepath.Join(t.TempDir(), "none.jsonl"), 0); err == nil {
		t.Error("expected error for missing file")
	}
}
