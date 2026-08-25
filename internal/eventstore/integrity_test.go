package eventstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

func TestOpenRejectsTamperedLog(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	event, err := rigging.NewEvent(rigging.EventCaseCreated, "case", "actor", time.Now(), rigging.CreatedData{Title: "T", Venue: "V", ManagerName: "M", PerformanceAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.Commit(0, []rigging.Event{event}, "actor", "key", map[string]string{"a": "b"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "events.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-2] ^= 1
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(dir); err == nil {
		t.Fatal("篡改日志应拒绝启动")
	}
}

func TestOpenRejectsTruncatedFrame(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "events.log"), []byte{0, 0, 0, 20, '{'}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("截断帧应拒绝启动")
	}
}
