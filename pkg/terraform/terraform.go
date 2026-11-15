package terraform

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
)

// StateIDPrefix starts every state ID of the current format. Legacy IDs are plain SHA-256 hex strings, so the two
// formats can never share an ID.
const StateIDPrefix = "v1-"

type State struct {
	ID string
	// LegacyID is the ID the state had before v0.3.0 (see GetLegacyStateID).
	// TODO: remove in 1.0.0, together with the migration of legacy states.
	LegacyID string
	Data     []byte
	Lock     LockInfo
	Project  string
	Name     string
}

// GetStateID returns the ID of a state. Project and name are length-prefixed, so different states can't share an ID.
func GetStateID(project, name string) string {
	return HashID("state", project, name)
}

// GetLegacyStateID returns the ID used before v0.3.0, which joined project and name with "-". It is ambiguous if
// project or name contain "-": "a-b/c" and "a/b-c" share the same legacy ID.
// TODO: remove in 1.0.0.
func GetLegacyStateID(project, name string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%s", project, name)))
	return fmt.Sprintf("%x", hash[:])
}

// IsLegacyStateID reports whether the ID has the format used before v0.3.0.
// TODO: remove in 1.0.0.
func IsLegacyStateID(id string) bool {
	return !strings.HasPrefix(id, StateIDPrefix)
}

// HashID returns a state ID derived from the fields. Every field is length-prefixed, and the purpose separates IDs
// derived for different uses (e.g. an authenticator hashing the ID with a secret) from each other.
func HashID(purpose string, fields ...string) string {
	h := sha256.New()

	for _, f := range append([]string{"terraform-backend " + purpose}, fields...) {
		_ = binary.Write(h, binary.BigEndian, uint64(len(f)))
		h.Write([]byte(f))
	}

	return fmt.Sprintf("%s%x", StateIDPrefix, h.Sum(nil))
}

type LockInfo struct {
	ID        string `json:"ID"`
	Path      string `json:"Path"`
	Operation string `json:"Operation"`
	Who       string `json:"Who"`
	Version   string `json:"Version"`
	Created   string `json:"Created"`
	Info      string `json:"Info"`
}

func (l LockInfo) Equal(r LockInfo) bool {
	return l.ID == r.ID &&
		l.Path == r.Path &&
		l.Operation == r.Operation &&
		l.Who == r.Who &&
		l.Version == r.Version &&
		l.Created == r.Created &&
		l.Info == r.Info
}
