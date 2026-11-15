package terraform

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStoredState(t *testing.T) {
	s := &State{Project: "project1", Name: "example", Data: []byte("encrypted")}
	now := time.Date(2026, 11, 6, 10, 0, 0, 123, time.FixedZone("CET", 3600))

	b, err := MarshalStoredState(s, "v0.3.0", now)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"metadata": {
			"format": "v1",
			"terraform_backend_version": "v0.3.0",
			"project": "project1",
			"state": "example",
			"last_updated": "2026-11-06T09:00:00Z"
		},
		"data": "ZW5jcnlwdGVk"
	}`, string(b))

	stored, err := UnmarshalStoredState(b, s)
	require.NoError(t, err)
	require.Equal(t, s.Data, stored.Data)
}

func TestUnmarshalStoredState_Invalid(t *testing.T) {
	s := &State{Project: "a", Name: "b-c"}

	other, err := MarshalStoredState(&State{Project: "a-b", Name: "c"}, "v0.3.0", time.Now())
	require.NoError(t, err)

	unknownFormat, err := json.Marshal(StoredState{Metadata: Metadata{Format: "v2", Project: "a", State: "b-c"}})
	require.NoError(t, err)

	for name, b := range map[string][]byte{
		"other state":    other,
		"unknown format": unknownFormat,
		"legacy data":    []byte("not json"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := UnmarshalStoredState(b, s)
			require.Error(t, err)
		})
	}
}
