package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected response: %d %v", recorder.Code, recorder.Header())
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["service"] != "arkhiv" || body["status"] != "ok" || body["stage"] != "bootstrap" {
		t.Fatalf("unexpected health: %v", body)
	}
}

func TestRouteBoundaries(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/api/missing", http.StatusNotFound},
		{"POST", "/api/health", http.StatusMethodNotAllowed},
	} {
		r := httptest.NewRecorder()
		Handler().ServeHTTP(r, httptest.NewRequest(tc.method, tc.path, nil))
		if r.Code != tc.status {
			t.Errorf("%s %s: got %d, want %d", tc.method, tc.path, r.Code, tc.status)
		}
	}
}

func TestTextBoundsCountCharacters(t *testing.T) {
	long := strings.Repeat("ж", 200)
	for _, tc := range []struct {
		name   string
		values url.Values
		within bool
	}{
		{"200-character query", url.Values{"q": {long}}, true},
		{"200-character subject", url.Values{"subject": {long}}, true},
		{"201-character query", url.Values{"q": {long + "ж"}}, false},
		{"201-character subject", url.Values{"subject": {long + "ж"}}, false},
	} {
		if textWithinBounds(tc.values) != tc.within {
			t.Errorf("%s: within bounds = %v, want %v", tc.name, !tc.within, tc.within)
		}
	}
}
