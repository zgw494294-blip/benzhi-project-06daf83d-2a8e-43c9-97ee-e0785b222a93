package eventstore

import (
	"bufio"
	"bytes"
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
	lockPath                   string
	sequence                   uint64
	lastDigest                 string
	cases                      map[string]*rigging.ClearanceCase
	idempotency                map[string]IdempotencyRecord
	audits                     map[string][]rigging.AuditEvent
}

// withLock acquires an exclusive flock on the sidecar lock file, runs fn, and
// releases the lock. It serializes commits across any number of service
// instances that share the same data directory: while one instance holds the
// lock, another's Commit blocks (in its own withLock call) rather than
// interleaving writes. The in-process mutex above still serializes goroutines
// within a single process. Open also uses this for its initial replay so the
// startup view is consistent with any in-flight committers.
func (s *Store) withLock(fn func() error) error {
	lf, err := os.OpenFile(s.lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lf.Close()
	if err = acquireLock(int(lf.Fd())); err != nil {
		return err
	}
	defer releaseLock(int(lf.Fd()))
	return fn()
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, logPath: filepath.Join(dir, "events.log"), snapshotPath: filepath.Join(dir, "projection.json"), lockPath: filepath.Join(dir, ".lock"), cases: map[string]*rigging.ClearanceCase{}, idempotency: map[string]IdempotencyRecord{}, audits: map[string][]rigging.AuditEvent{}}
	var saved *snapshot
	if err := s.withLock(func() error {
		if err := s.replay(); err != nil {
			return err
		}
		snap, err := readSnapshot(s.snapshotPath)
		if err != nil {
			return err
		}
		saved = snap
		return nil
	}); err != nil {
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

// Close is retained for API completeness; the cross-process lock is acquired
// and released per commit, so there is no persistent resource to release.
func (s *Store) Close() error { return nil }

// replay re-reads the entire event log from disk and rebuilds the in-memory
// projection, sequence counter and chain digest. It is safe to call again at
// any time: it resets the projection before re-applying frames so that repeated
// calls do not accumulate duplicate state.
func (s *Store) replay() error {
	s.sequence = 0
	s.lastDigest = ""
	s.cases = map[string]*rigging.ClearanceCase{}
	s.idempotency = map[string]IdempotencyRecord{}
	s.audits = map[string][]rigging.AuditEvent{}
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
	// Hold the cross-process lock for the whole reconcile→append→snapshot
	// sequence so two instances sharing this data directory never write from
	// the same sequence tip or interleave frame bytes in the log. withLock
	// blocks until any other committer (in this or another process) releases.
	var result *rigging.ClearanceCase
	var replayed bool
	if err = s.withLock(func() error {
		// Another instance sharing this data directory may have appended and
		// snapshotted frames while we waited for the exclusive lock. Reconcile
		// the in-memory sequence, chain digest and projection with the log on
		// disk before validating expectedVersion or building a new frame, so we
		// never write a duplicate sequence or a PreviousDigest that chains onto
		// a stale tip. replay() also rebuilds idempotency, which makes the
		// prior-key check reflect requests already committed by the other
		// instance.
		if err := s.replay(); err != nil {
			return err
		}
		if prior, ok := s.idempotency[key]; ok {
			if prior.RequestDigest != requestDigest {
				return &IdempotencyConflict{}
			}
			c := s.cases[prior.CaseID]
			clone, e := rigging.Clone(c)
			result, replayed = clone, true
			return e
		}
		caseID := events[0].CaseID
		c := s.cases[caseID]
		actual := 0
		if c != nil {
			actual = c.Version
		}
		if expected != actual {
			return &VersionConflict{Expected: expected, Actual: actual}
		}
		working := &rigging.ClearanceCase{}
		if c != nil {
			working, err = rigging.Clone(c)
			if err != nil {
				return err
			}
		}
		for _, event := range events {
			if event.CaseID != caseID {
				return errors.New("一次提交不能跨档案")
			}
			if err = rigging.Apply(working, event); err != nil {
				return err
			}
		}
		frame := Frame{SchemaVersion: schemaVersion, Sequence: s.sequence + 1, PreviousDigest: s.lastDigest, Events: events}
		for _, event := range events {
			payloadSum := sha256.Sum256(event.Data)
			frame.Audits = append(frame.Audits, rigging.AuditEvent{Sequence: frame.Sequence, CaseID: caseID, EventType: event.Type, Actor: actor, OccurredAt: event.OccurredAt, IdempotencyKey: key, PayloadDigest: hex.EncodeToString(payloadSum[:]), PreviousDigest: s.lastDigest})
		}
		frame.Idempotency = &IdempotencyRecord{Key: key, RequestDigest: requestDigest, CaseID: caseID, Version: working.Version}
		if err = sealFrame(&frame); err != nil {
			return err
		}
		if err = s.appendFrame(frame); err != nil {
			return err
		}
		if err = s.applyFrame(frame); err != nil {
			return err
		}
		s.sequence = frame.Sequence
		s.lastDigest = frame.Checksum
		if err = s.saveSnapshot(); err != nil {
			return err
		}
		clone, err := rigging.Clone(working)
		result = clone
		return err
	}); err != nil {
		return result, replayed, err
	}
	return result, replayed, nil
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
	// Build the complete on-disk frame — length prefix and JSON body — in a
	// single buffer so it is written with one Write call. Combined with the
	// exclusive flock held by Commit, this guarantees that frames from
	// concurrent committers never interleave byte ranges within the log.
	var buf bytes.Buffer
	if err = binary.Write(&buf, binary.BigEndian, uint32(len(b))); err != nil {
		return err
	}
	buf.Write(b)
	f, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(buf.Bytes()); err != nil {
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
