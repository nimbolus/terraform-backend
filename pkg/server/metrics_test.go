package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// gatherRequestCount returns the values of the request_count metric, keyed by method, path and code label.
func gatherRequestCount(t *testing.T) map[[3]string]float64 {
	t.Helper()

	reg := prometheus.NewPedanticRegistry()
	require.NoError(t, reg.Register(requestCount))

	families, err := reg.Gather()
	require.NoError(t, err)

	values := map[[3]string]float64{}

	for _, f := range families {
		for _, m := range f.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}

			values[[3]string{labels["method"], labels["path"], labels["code"]}] = m.GetCounter().GetValue()
		}
	}

	return values
}

func TestRequestCount_PathLabelIsRouteTemplate(t *testing.T) {
	r, _ := newUnitTestRouter(t, 1024)

	const template = "/state/{project}/{name}"

	paths := []string{"/state/project1/example", "/state/project2/other", "/state/random/path"}
	key := [3]string{"LOCK", template, "403"}

	before := gatherRequestCount(t)[key]

	for _, path := range paths {
		req := httptest.NewRequest("LOCK", path, bytes.NewReader(nil))

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusForbidden, rec.Code)
	}

	after := gatherRequestCount(t)
	require.Equal(t, before+3, after[key])

	for labels := range after {
		require.NotContains(t, paths, labels[1], "raw request path must not be used as label")
	}
}

func TestRouteTemplate_NoRoute(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/state/project1/example", nil)

	require.Equal(t, "unknown", routeTemplate(req))
}
