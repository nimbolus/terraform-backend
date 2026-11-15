package terraform

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetStateID_Unambiguous(t *testing.T) {
	require.Equal(t, GetLegacyStateID("a-b", "c"), GetLegacyStateID("a", "b-c"), "the legacy ID is ambiguous")
	require.NotEqual(t, GetStateID("a-b", "c"), GetStateID("a", "b-c"))
	require.NotEqual(t, GetStateID("ab", "c"), GetStateID("a", "bc"))
	require.NotEqual(t, GetStateID("a", ""), GetStateID("", "a"))
}

func TestGetStateID_Format(t *testing.T) {
	id := GetStateID("project1", "example")

	require.Regexp(t, `^v1-[0-9a-f]{64}$`, id)
	require.False(t, IsLegacyStateID(id))
	require.Equal(t, id, GetStateID("project1", "example"), "the ID must be stable")
	require.NotEqual(t, id, HashID("other", "project1", "example"), "the purpose must change the ID")
}

func TestGetLegacyStateID(t *testing.T) {
	// sha256("project1-example"), the ID previous versions used
	require.Equal(t, "beeaa72f907d525e0e34826f3201d96d8fa9a86870804a09da3814b085dd7b36", GetLegacyStateID("project1", "example"))
	require.True(t, IsLegacyStateID(GetLegacyStateID("project1", "example")))
}
