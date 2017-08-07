package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// Wire-shape pinning test: no pre-existing field on /internal/status,
// /internal/status/endpoints or /internal/status/models may be renamed,
// retyped or removed by the additive dashboard fields.
//
// The expected maps below were captured from main's handlers (the branch had
// not touched these files when this test was written). Each entry pins a JSON
// key to the type encoding/json produces when decoding into interface{}:
// string, float64 (numbers), bool, map[string]interface{} or
// []interface{}. Additive fields added later are allowed: this test only
// asserts the pre-existing contract is preserved, so it does not fail when
// new keys appear alongside the pinned ones.

func jsonShape(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &m), "response was not a JSON object")
	return m
}

func doStatus(t *testing.T, app *Application, target string) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	switch target {
	case "/internal/status":
		app.statusHandler(w, req)
	case "/internal/status/endpoints":
		app.endpointsStatusHandler(w, req)
	case "/internal/status/models":
		app.modelsStatusHandler(w, req)
	default:
		t.Fatalf("unknown target %s", target)
	}
	require.Equal(t, http.StatusOK, w.Code, "handler for %s returned non-200", target)
	return w.Body.Bytes()
}

func assertKeysTyped(t *testing.T, obj map[string]interface{}, expected map[string]string) {
	t.Helper()
	for key, wantType := range expected {
		got, ok := obj[key]
		require.True(t, ok, "missing pinned key %q (additive-field wire-shape guarantee regression)", key)
		switch wantType {
		case "string":
			_, ok := got.(string)
			assert.True(t, ok, "key %q: want string, got %T (%v)", key, got, got)
		case "number":
			_, ok := got.(float64)
			assert.True(t, ok, "key %q: want number, got %T (%v)", key, got, got)
		case "bool":
			_, ok := got.(bool)
			assert.True(t, ok, "key %q: want bool, got %T (%v)", key, got, got)
		case "object":
			_, ok := got.(map[string]interface{})
			assert.True(t, ok, "key %q: want object, got %T (%v)", key, got, got)
		case "array":
			_, ok := got.([]interface{})
			assert.True(t, ok, "key %q: want array, got %T (%v)", key, got, got)
		default:
			t.Fatalf("test bug: unknown wantType %q for key %q", wantType, key)
		}
	}
}

func TestWireShape_StatusResponse(t *testing.T) {
	app := seedWireShapeApp(t)
	shape := jsonShape(t, doStatus(t, app, "/internal/status"))

	assertKeysTyped(t, shape, map[string]string{
		"timestamp": "string",
		"endpoints": "array",
		"proxy":     "object",
		"security":  "object",
		"system":    "object",
	})

	sys, _ := shape["system"].(map[string]interface{})
	require.NotNil(t, sys, "system was not an object")
	assertKeysTyped(t, sys, map[string]string{
		"status":              "string",
		"endpoints_up":        "string",
		"success_rate":        "string",
		"avg_latency":         "string",
		"total_traffic":       "string",
		"uptime":              "string",
		"version":             "string",
		"commit":              "string",
		"active_connections":  "number",
		"security_violations": "number",
		"total_requests":      "number",
		"total_failures":      "number",
	})

	proxy, _ := shape["proxy"].(map[string]interface{})
	require.NotNil(t, proxy)
	assertKeysTyped(t, proxy, map[string]string{
		"engine":   "string",
		"profile":  "string",
		"balancer": "string",
	})

	sec, _ := shape["security"].(map[string]interface{})
	require.NotNil(t, sec)
	assertKeysTyped(t, sec, map[string]string{
		"status":      "string",
		"blocked_ips": "number",
		"violations":  "object",
	})

	endpoints, _ := shape["endpoints"].([]interface{})
	require.NotEmpty(t, endpoints, "seed should produce at least one endpoint")
	first, _ := endpoints[0].(map[string]interface{})
	require.NotNil(t, first)
	assertKeysTyped(t, first, map[string]string{
		"name":         "string",
		"status":       "string",
		"success_rate": "string",
		"avg_latency":  "string",
		"traffic":      "string",
		"last_check":   "string",
		"next_check":   "string",
		"issues":       "string",
		"models":       "object",
		"priority":     "number",
		"connections":  "number",
		"requests":     "number",
		"url":          "string",
	})
}

func TestWireShape_EndpointsStatusResponse(t *testing.T) {
	app := seedWireShapeApp(t)
	shape := jsonShape(t, doStatus(t, app, "/internal/status/endpoints"))

	assertKeysTyped(t, shape, map[string]string{
		"timestamp":      "string",
		"endpoints":      "array",
		"total_count":    "number",
		"healthy_count":  "number",
		"routable_count": "number",
	})

	endpoints, _ := shape["endpoints"].([]interface{})
	require.NotEmpty(t, endpoints)
	// Find the endpoint with traffic so request_count / health_check are
