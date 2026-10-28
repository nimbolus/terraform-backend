package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewHTTPServer_Timeouts(t *testing.T) {
	handler := http.NewServeMux()
	srv := newHTTPServer(":8080", handler)

	require.Equal(t, ":8080", srv.Addr)
	require.Equal(t, handler, srv.Handler)

	// zero means no timeout for all of these
	require.Positive(t, srv.ReadHeaderTimeout)
	require.Positive(t, srv.ReadTimeout)
	require.Positive(t, srv.WriteTimeout)
	require.Positive(t, srv.IdleTimeout)
}
