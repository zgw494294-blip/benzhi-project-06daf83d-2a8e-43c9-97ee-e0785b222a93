package eventstore

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

const maxFrameBytes = 8 << 20

var ErrNotFound = errors.New("档案不存在")

type VersionConflict struct{ Expected, Actual int }

func (e *VersionConflict) Error() string {
	return fmt.Sprintf("版本冲突：期望 %d，实际 %d", e.Expected, e.Actual)
}

type IdempotencyConflict struct{}

func (*IdempotencyConflict) Error() string { return "idempotencyKey 已用于不同请求" }

type Store struct {
	mu                         sync.Mutex
	dir, logPath, snapshotPath string
	sequence                   uint64
	lastDigest                 string
	cases                      map[string]*rigging.ClearanceCase
	idempotency                map[string]IdempotencyRecord
	audits                     map[string][]rigging.AuditEvent
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, logPath: filepath.Join(dir, "events.log"), snapshotPath: filepath.Join(dir, "projection.json"), cases: map[string]*rigging.ClearanceCase{}, idempotency: map[string]IdempotencyRecord{}, audits: map[string][]rigging.AuditEvent{}}
	saved, err := readSnapshot(s.snapshotPath)
	if err != nil {
		return nil, err
	}
	if err := s.replay(); err != nil {
		return nil, err
	}
	if saved != nil {
		if saved.Sequence > s.sequence {
			return nil, fmt.Errorf("投影快照序号领先于事件日志")
		}
		if saved.Sequence == s.sequence {
			if saved.LastDigest != s.lastDigest {
				return nil, fmt.Errorf("投影快照与事件链摘要不一致")
			}
			s.cases, s.idempotency, s.audits = saved.Cases, saved.Idempotency, saved.Audits
		}
	}
	return s, nil
}

func (s *Store) replay() error {
	f, err := os.Open(s.logPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	var seq uint64 = 1
	previous := ""
	for {
		var length uint32
		err = binary.Read(r, binary.BigEndian, &length)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("事件日志长度前缀截断: %w", err)
		}
		if length == 0 || length > maxFrameBytes {
			return fmt.Errorf("事件帧长度无效: %d", length)
		}
		b := make([]byte, length)
		if _, err = io.ReadFull(r, b); err != nil {
			return fmt.Errorf("事件帧截断: %w", err)
		}
		var frame Frame
		if err = json.Unmarshal(b, &frame); err != nil {
			return fmt.Errorf("事件帧 JSON 无效: %w", err)
		}
		if err = verifyFrame(frame, seq, previous); err != nil {
			return err
		}
		if err = s.applyFrame(frame); err != nil {
			return err
		}
		previous = frame.Checksum
		seq++
	}
	s.sequence = seq - 1
	s.lastDigest = previous
	return nil
}

func (s *Store) applyFrame(frame Frame) error {
	for _, event := range frame.Events {
		c := s.cases[event.CaseID]
		if c == nil {
			c = &rigging.ClearanceCase{}
			s.cases[event.CaseID] = c
		}
		if err := rigging.Apply(c, event); err != nil {
			return fmt.Errorf("重放事件 %s: %w", event.Type, err)
		}
	}
	for _, audit := range frame.Audits {
		s.audits[audit.CaseID] = append(s.audits[audit.CaseID], audit)
	}
	if frame.Idempotency != nil {
		s.idempotency[frame.Idempotency.Key] = *frame.Idempotency
	}
	return nil
}

func digestRequest(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Store) Commit(expected int, events []rigging.Event, actor, key string, request any) (*rigging.ClearanceCase, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(events) == 0 {
		return nil, false, errors.New("没有待提交事件")
	}
	requestDigest, err := digestRequest(request)
	if err != nil {
		return nil, false, err
	}
	if prior, ok := s.idempotency[key]; ok {
		if prior.RequestDigest != requestDigest {
			return nil, false, &IdempotencyConflict{}
		}
		c := s.cases[prior.CaseID]
		clone, e := rigging.Clone(c)
		return clone, true, e
	}
	caseID := events[0].CaseID
	c := s.cases[caseID]
	actual := 0
	if c != nil {
		actual = c.Version
	}
	if expected != actual {
		return nil, false, &VersionConflict{Expected: expected, Actual: actual}
	}
	for _, event := range events {
		if event.CaseID != caseID {
			return nil, false, errors.New("一次提交不能跨档案")
		}
	}
	frame := Frame{SchemaVersion: schemaVersion, Sequence: s.sequence + 1, PreviousDigest: s.lastDigest, Events: events}
	for _, event := range events {
		payloadSum := sha256.Sum256(event.Data)
		frame.Audits = append(frame.Audits, rigging.AuditEvent{Sequence: frame.Sequence, CaseID: caseID, EventType: event.Type, Actor: actor, OccurredAt: event.OccurredAt, IdempotencyKey: key, PayloadDigest: hex.EncodeToString(payloadSum[:]), PreviousDigest: s.lastDigest})
	}
	frame.Idempotency = &IdempotencyRecord{Key: key, RequestDigest: requestDigest, CaseID: caseID, Version: expected + len(events)}
	if err = sealFrame(&frame); err != nil {
		return nil, false, err
	}
	// Apply events to a working copy so a mid-batch failure or a persistence
	// failure leaves the live in-memory state untouched. The live state is only
	// updated after the frame has been durably appended to the event log.
	var working *rigging.ClearanceCase
	if c != nil {
		working, err = rigging.Clone(c)
		if err != nil {
			return nil, false, err
		}
	} else {
		working = &rigging.ClearanceCase{}
	}
	for _, event := range frame.Events {
		if err = rigging.Apply(working, event); err != nil {
			return nil, false, fmt.Errorf("重放事件 %s: %w", event.Type, err)
		}
	}
	if err = s.appendFrame(frame); err != nil {
		return nil, false, err
	}
	s.cases[caseID] = working
	s.audits[caseID] = append(s.audits[caseID], frame.Audits...)
	s.idempotency[key] = *frame.Idempotency
	s.sequence = frame.Sequence
	s.lastDigest = frame.Checksum
	if err = s.saveSnapshot(); err != nil {
		return nil, false, err
	}
	clone, err := rigging.Clone(s.cases[caseID])
	return clone, false, err
}

// Replay returns the result of an already committed request before domain
// validation is repeated against the newer aggregate state.
func (s *Store) Replay(key string, request any) (*rigging.ClearanceCase, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prior, ok := s.idempotency[key]
	if !ok {
		return nil, false, nil
	}
	digest, err := digestRequest(request)
	if err != nil {
		return nil, false, err
	}
	if prior.RequestDigest != digest {
		return nil, false, &IdempotencyConflict{}
	}
	c := s.cases[prior.CaseID]
	clone, err := rigging.Clone(c)
	return clone, true, err
}

func (s *Store) appendFrame(frame Frame) error {
	b, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = binary.Write(f, binary.BigEndian, uint32(len(b))); err != nil {
		return err
	}
	if _, err = f.Write(b); err != nil {
		return err
	}
	return f.Sync()
}

func (s *Store) saveSnapshot() error {
	return writeSnapshot(s.snapshotPath, snapshot{SchemaVersion: schemaVersion, Sequence: s.sequence, LastDigest: s.lastDigest, Cases: s.cases, Idempotency: s.idempotency, Audits: s.audits})
}

func (s *Store) Get(id string) (*rigging.ClearanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.cases[id]
	if c == nil {
		return nil, ErrNotFound
	}
	return rigging.Clone(c)
}
func (s *Store) List() ([]*rigging.ClearanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*rigging.ClearanceCase, 0, len(s.cases))
	for _, c := range s.cases {
		clone, err := rigging.Clone(c)
		if err != nil {
			return nil, err
		}
		out = append(out, clone)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Timeline(id string) ([]rigging.AuditEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cases[id] == nil {
		return nil, ErrNotFound
	}
	return append([]rigging.AuditEvent(nil), s.audits[id]...), nil
}
func (s *Store) FindCredential(number string) (*rigging.ClearanceCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.cases {
		if c.Credential != nil && c.Credential.Number == number {
			return rigging.Clone(c)
		}
	}
	return nil, ErrNotFound
}
