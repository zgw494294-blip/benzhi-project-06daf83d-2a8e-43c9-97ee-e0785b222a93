package eventstore

import "fmt"

type Integrity struct {
	Sequence   uint64 `json:"sequence"`
	LastDigest string `json:"lastDigest"`
	CaseCount  int    `json:"caseCount"`
}

func (s *Store) Integrity() Integrity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Integrity{Sequence: s.sequence, LastDigest: s.lastDigest, CaseCount: len(s.cases)}
}

func (s *Store) String() string {
	i := s.Integrity()
	return fmt.Sprintf("sequence=%d cases=%d digest=%s", i.Sequence, i.CaseCount, i.LastDigest)
}
