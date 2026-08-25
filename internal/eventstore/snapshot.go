package eventstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

type snapshot struct {
	SchemaVersion int                               `json:"schemaVersion"`
	Sequence      uint64                            `json:"sequence"`
	LastDigest    string                            `json:"lastDigest"`
	Cases         map[string]*rigging.ClearanceCase `json:"cases"`
	Idempotency   map[string]IdempotencyRecord      `json:"idempotency"`
	Audits        map[string][]rigging.AuditEvent   `json:"audits"`
	Checksum      string                            `json:"checksum"`
}

func snapshotDigest(s snapshot) (string, error) {
	s.Checksum = ""
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func readSnapshot(path string) (*snapshot, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s snapshot
	if err = json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("解析投影快照: %w", err)
	}
	if s.SchemaVersion != schemaVersion {
		return nil, fmt.Errorf("不支持的快照 schemaVersion %d", s.SchemaVersion)
	}
	digest, err := snapshotDigest(s)
	if err != nil {
		return nil, err
	}
	if digest != s.Checksum {
		return nil, fmt.Errorf("投影快照校验和不匹配")
	}
	return &s, nil
}

func writeSnapshot(path string, s snapshot) error {
	digest, err := snapshotDigest(s)
	if err != nil {
		return err
	}
	s.Checksum = digest
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "projection-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		tmp.Close()
		if !ok {
			os.Remove(name)
		}
	}()
	if _, err = tmp.Write(b); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err == nil {
		err = dir.Sync()
		dir.Close()
	}
	if err != nil {
		return err
	}
	ok = true
	return nil
}
