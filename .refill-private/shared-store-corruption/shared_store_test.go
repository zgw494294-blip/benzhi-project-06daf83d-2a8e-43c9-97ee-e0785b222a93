package shared_store_corruption_test

import (
	"sync"
	"testing"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

func TestConcurrentStoresCannotCorruptSharedEventLog(t *testing.T) {
	dir := t.TempDir()
	type opened struct {
		store *eventstore.Store
		err   error
	}
	startOpen := make(chan struct{})
	openedCh := make(chan opened, 2)
	for range 2 {
		go func() {
			<-startOpen
			s, err := eventstore.Open(dir)
			openedCh <- opened{s, err}
		}()
	}
	close(startOpen)
	first, second := <-openedCh, <-openedCh
	if first.err != nil || second.err != nil {
		return
	}
	stores := []*eventstore.Store{first.store, second.store}
	startCommit := make(chan struct{})
	errs := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for i, store := range stores {
		go func(i int, store *eventstore.Store) {
			ready.Done()
			<-startCommit
			id := "case-a"
			if i == 1 {
				id = "case-b"
			}
			event, err := rigging.NewEvent(rigging.EventCaseCreated, id, "actor", time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC), rigging.CreatedData{Title: id, Venue: "V", ManagerName: "M", PerformanceAt: time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)})
			if err == nil {
				_, _, err = store.Commit(0, []rigging.Event{event}, "actor", "key-"+id, map[string]string{"id": id})
			}
			errs <- err
		}(i, store)
	}
	ready.Wait()
	close(startCommit)
	if err := <-errs; err != nil {
		return
	}
	if err := <-errs; err != nil {
		return
	}
	if _, err := eventstore.Open(dir); err != nil {
		t.Fatalf("两个 Store 实例均接受写入后事件日志无法重放：%v", err)
	}
}
