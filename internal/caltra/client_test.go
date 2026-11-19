package caltra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateOrganizationPreservesOmittedDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch || req.URL.Path != "/management/v1/organizations/example" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		if req.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("request is missing the configured authorization")
		}

		var body map[string]json.RawMessage
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Errorf("invalid request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(body) != 1 || string(body["name"]) != `"Renamed"` {
			t.Error("unconfigured organization defaults must be omitted")
		}
		_, _ = fmt.Fprint(w, `{"id":"example","name":"Renamed","model_id":"existing","reasoning_level":null}`)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	org, err := client.UpdateOrganization(context.Background(), "example", UpdateOrganization{Name: "Renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if org.ModelID == nil || *org.ModelID != "existing" || org.ReasoningLevel != nil {
		t.Fatal("organization response did not preserve the existing defaults")
	}
}

func TestRequestsDoNotFollowRedirectsOrExposeErrorBodies(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("management client followed a redirect")
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasSuffix(req.URL.Path, "/redirect") {
			http.Redirect(w, req, target.URL, http.StatusTemporaryRedirect)
			return
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = fmt.Fprint(w, `{"code":"tool_grant_required","detail":"sensitive agent instructions"}`)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.HostedAgent(context.Background(), "redirect")
	var problem *HTTPError
	if !errors.As(err, &problem) || problem.Status != http.StatusTemporaryRedirect {
		t.Fatalf("expected redirect error, got %v", err)
	}
	_, err = client.HostedAgent(context.Background(), "conflict")
	if !errors.As(err, &problem) || problem.Status != http.StatusConflict ||
		problem.Code != "tool_grant_required" || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("expected sanitized conflict error, got %v", err)
	}
}

func TestOnlyManagementObjectNotFoundCanRemoveState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if strings.HasSuffix(req.URL.Path, "/missing-object") {
			_, _ = fmt.Fprint(w, `{"code":"resource_not_found"}`)
		}
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.HostedAgent(context.Background(), "missing-object")
	if !IsNotFound(err) {
		t.Fatalf("expected a missing Caltra object response, got %v", err)
	}
	_, err = client.HostedAgent(context.Background(), "missing-route")
	if IsNotFound(err) {
		t.Fatalf("a missing API endpoint must leave Terraform state in place")
	}
}
