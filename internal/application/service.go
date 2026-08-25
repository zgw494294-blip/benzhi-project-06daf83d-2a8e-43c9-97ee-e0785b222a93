package application

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

type Service struct {
	store          *eventstore.Store
	commitMu       sync.Mutex
	dashboardMu    sync.Mutex
	dashboardCases []*rigging.ClearanceCase
	now            func() time.Time
}

func New(store *eventstore.Store) *Service         { return &Service{store: store, now: time.Now} }
func (s *Service) SetClock(clock func() time.Time) { s.now = clock }

func newID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b)
}
func clean(v string) string { return strings.TrimSpace(v) }
func required(value, name string) error {
	if clean(value) == "" {
		return &Error{Code: "FIELD_REQUIRED", Message: name + " 不能为空", Status: 400}
	}
	return nil
}

func (s *Service) Create(cmd CreateCase) (*rigging.ClearanceCase, error) {
	if err := cmd.CommandMeta.validate(); err != nil {
		return nil, err
	}
	if cmd.ExpectedVersion != 0 {
		return nil, &Error{Code: "INVALID_VERSION", Message: "创建档案的 expectedVersion 必须为 0", Status: 400}
	}
	if err := required(cmd.Title, "title"); err != nil {
		return nil, err
	}
	if err := required(cmd.Venue, "venue"); err != nil {
		return nil, err
	}
	if err := required(cmd.ManagerName, "managerName"); err != nil {
		return nil, err
	}
	if cmd.PerformanceAt.IsZero() {
		return nil, &Error{Code: "FIELD_REQUIRED", Message: "performanceAt 不能为空", Status: 400}
	}
	if cmd.ID == "" {
		cmd.ID = newID("case")
	}
	event, err := rigging.NewEvent(rigging.EventCaseCreated, cmd.ID, cmd.Actor, s.now(), rigging.CreatedData{Title: clean(cmd.Title), Venue: clean(cmd.Venue), ManagerName: clean(cmd.ManagerName), PerformanceAt: cmd.PerformanceAt.UTC()})
	if err != nil {
		return nil, normalize(err)
	}
	return s.commit(cmd.CommandMeta, []rigging.Event{event}, cmd)
}

func (s *Service) commit(meta CommandMeta, events []rigging.Event, request any) (*rigging.ClearanceCase, error) {
	s.commitMu.Lock()
	defer s.commitMu.Unlock()
	c, _, err := s.store.Commit(meta.ExpectedVersion, events, meta.Actor, meta.IdempotencyKey, request)
	return c, normalize(err)
}
func (s *Service) load(id string) (*rigging.ClearanceCase, error) {
	c, err := s.store.Get(id)
	return c, normalize(err)
}

func (s *Service) replay(key string, request any) (*rigging.ClearanceCase, bool, error) {
	c, found, err := s.store.Replay(key, request)
	return c, found, normalize(err)
}
