package eventstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

const schemaVersion = 1

type IdempotencyRecord struct {
	Key           string `json:"key"`
	RequestDigest string `json:"requestDigest"`
	CaseID        string `json:"caseId"`
	Version       int    `json:"version"`
}

type Frame struct {
	SchemaVersion  int                  `json:"schemaVersion"`
	Sequence       uint64               `json:"sequence"`
	PreviousDigest string               `json:"previousDigest"`
	Events         []rigging.Event      `json:"events"`
	Audits         []rigging.AuditEvent `json:"audits"`
	Idempotency    *IdempotencyRecord   `json:"idempotency,omitempty"`
	Checksum       string               `json:"checksum"`
}

func frameDigest(frame Frame) (string, error) {
	frame.Checksum = ""
	b, err := json.Marshal(frame)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func sealFrame(frame *Frame) error {
	digest, err := frameDigest(*frame)
	if err != nil {
		return err
	}
	frame.Checksum = digest
	return nil
}

func verifyFrame(frame Frame, sequence uint64, previous string) error {
	if frame.SchemaVersion != schemaVersion {
		return fmt.Errorf("不支持的事件帧 schemaVersion %d", frame.SchemaVersion)
	}
	if frame.Sequence != sequence {
		return fmt.Errorf("事件帧序号不连续：得到 %d，期望 %d", frame.Sequence, sequence)
	}
	if frame.PreviousDigest != previous {
		return fmt.Errorf("事件帧前序摘要不匹配")
	}
	digest, err := frameDigest(frame)
	if err != nil {
		return err
	}
	if digest != frame.Checksum {
		return fmt.Errorf("事件帧校验和不匹配")
	}
	return nil
}
