package terraform

import (
	"encoding/json"
	"fmt"
	"time"
)

// FormatV1 is the current format of stored states.
const FormatV1 = "v1"

// StoredState is the format states are stored in. Metadata is not encrypted, so the storage can be inspected without
// the KMS key. Data holds the encrypted state.
type StoredState struct {
	Metadata Metadata `json:"metadata"`
	Data     []byte   `json:"data"`
}

type Metadata struct {
	Format                  string    `json:"format"`
	TerraformBackendVersion string    `json:"terraform_backend_version"`
	Project                 string    `json:"project"`
	State                   string    `json:"state"`
	LastUpdated             time.Time `json:"last_updated"`
}

// MarshalStoredState wraps the encrypted data of the state with its metadata.
func MarshalStoredState(s *State, version string, now time.Time) ([]byte, error) {
	return json.Marshal(StoredState{
		Metadata: Metadata{
			Format:                  FormatV1,
			TerraformBackendVersion: version,
			Project:                 s.Project,
			State:                   s.Name,
			LastUpdated:             now.UTC().Truncate(time.Second),
		},
		Data: s.Data,
	})
}

// UnmarshalStoredState unwraps a stored state and checks that it belongs to the state s.
func UnmarshalStoredState(b []byte, s *State) (*StoredState, error) {
	var stored StoredState
	if err := json.Unmarshal(b, &stored); err != nil {
		return nil, fmt.Errorf("decoding stored state: %w", err)
	}

	if stored.Metadata.Format != FormatV1 {
		return nil, fmt.Errorf("unsupported format %q of stored state", stored.Metadata.Format)
	}

	if stored.Metadata.Project != s.Project || stored.Metadata.State != s.Name {
		return nil, fmt.Errorf("stored state belongs to %s/%s", stored.Metadata.Project, stored.Metadata.State)
	}

	return &stored, nil
}
