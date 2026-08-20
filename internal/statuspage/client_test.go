package statuspage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

const baseURL = "https://www.githubstatus.com/api/v2"

// loadFixture reads a testdata file into a byte slice.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

// newTestServer returns an httptest server that serves the given route->fixture map.
func newTestServer(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for route, fixture := range routes {
		route, fixture := route, fixture
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(loadFixture(t, fixture))
		})
	}
	return httptest.NewServer(mux)
}

func TestClient_GetStatus(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/status.json": "summary.json",
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	status, err := c.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus returned error: %v", err)
	}
	if status.Indicator != StatusNone {
		t.Errorf("expected indicator %q, got %q", StatusNone, status.Indicator)
	}
	if status.Description == "" {
		t.Error("expected non-empty description")
	}
}

func TestClient_GetComponents(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/components.json": "components.json",
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	comps, err := c.GetComponents()
	if err != nil {
		t.Fatalf("GetComponents returned error: %v", err)
	}
	if len(comps) == 0 {
		t.Fatal("expected at least one component")
	}
	// githubstatus.com exposes 11 components (Git Operations, Webhooks, API, Issues, PRs, Actions, Packages, Pages, Copilot, Codespaces, Copilot AI Model Providers)
	if len(comps) < 5 {
		t.Errorf("expected >=5 components, got %d", len(comps))
	}
	for _, comp := range comps {
		if comp.Name == "" {
			t.Error("component with empty name")
		}
		if comp.Status == "" {
			t.Errorf("component %q with empty status", comp.Name)
		}
	}
}

func TestClient_GetIncidents(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/incidents.json": "incidents_page1.json",
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	incs, err := c.GetIncidents(1)
	if err != nil {
		t.Fatalf("GetIncidents returned error: %v", err)
	}
	if len(incs) == 0 {
		t.Fatal("expected at least one incident")
	}
	for _, inc := range incs {
		if inc.ID == "" {
			t.Error("incident with empty ID")
		}
		if inc.Name == "" {
			t.Error("incident with empty name")
		}
		if inc.CreatedAt.IsZero() {
			t.Errorf("incident %q has zero CreatedAt", inc.ID)
		}
	}
}

func TestClient_GetIncidents_Pagination(t *testing.T) {
	// Page 2 should also return incidents (the API paginates at 50 per page).
	srv := newTestServer(t, map[string]string{
		"/incidents.json": "incidents_page2.json",
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	incs, err := c.GetIncidents(2)
	if err != nil {
		t.Fatalf("GetIncidents(2) returned error: %v", err)
	}
	if len(incs) == 0 {
		t.Fatal("expected incidents on page 2")
	}
}

func TestClient_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if _, err := c.GetStatus(); err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
	if _, err := c.GetComponents(); err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
	if _, err := c.GetIncidents(1); err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
}

func TestClient_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"not":"json"`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if _, err := c.GetStatus(); err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}

// TestIncidentJSONShape guards the raw JSON shape used by the statuspage API
// (e.g. that Incident.CreatedAt parses from RFC3339 strings).
func TestIncidentJSONShape(t *testing.T) {
	raw := `[{"id":"abc123","name":"Incident with Actions","status":"resolved","impact":"major","created_at":"2026-08-06T15:22:49.049Z","resolved_at":"2026-08-07T02:04:44.460Z","shortlink":"https://stspg.io/abc123"}]`
	var incs []Incident
	if err := json.Unmarshal([]byte(raw), &incs); err != nil {
		t.Fatalf("failed to unmarshal incident: %v", err)
	}
	if len(incs) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incs))
	}
	inc := incs[0]
	if inc.CreatedAt.Format("2006-01-02") != "2026-08-06" {
		t.Errorf("unexpected CreatedAt: %v", inc.CreatedAt)
	}
	if inc.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}
}
