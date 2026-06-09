// Copyright (c) Vestmark
// SPDX-License-Identifier: MPL-2.0

package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vestmark-infra/tf-provider-northflank/internal/client"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// newTestServer starts an httptest.Server and returns it along with a
// *client.Client pointed at it.
func newTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *client.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := client.New(srv.URL, "test-token")
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}
	return srv, c
}

func jsonWrite(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// ─── auth injection ───────────────────────────────────────────────────────────

func TestClientSendsBearerToken(t *testing.T) {
	var gotAuth string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		jsonWrite(w, http.StatusOK, map[string]any{
			"data": map[string]any{
				"id": "my-secret", "name": "My Secret",
				"priority": 10, "secretType": "environment",
				"type": "secret", "tags": []string{},
				"secrets": map[string]any{},
				"projectId": "proj-1",
				"restrictions": map[string]any{},
				"createdAt": "2024-01-01T00:00:00Z",
				"updatedAt": "2024-01-01T00:00:00Z",
			},
		})
	})
	_, c := newTestServer(t, handler)
	_, _ = c.GetProjectSecret(context.Background(), "proj-1", "my-secret")
	if gotAuth != "Bearer test-token" {
		t.Errorf("expected Authorization 'Bearer test-token', got %q", gotAuth)
	}
}

// ─── CreateProjectSecret ──────────────────────────────────────────────────────

func TestCreateProjectSecret(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/projects/proj-1/secrets" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Parse and echo back the body
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		jsonWrite(w, http.StatusOK, map[string]any{
			"data": map[string]any{
				"id":         "my-secret-group",
				"name":       body["name"],
				"priority":   body["priority"],
				"secretType": body["secretType"],
				"type":       "secret",
				"tags":       []string{},
				"secrets":    map[string]any{"variables": map[string]string{"KEY": "VALUE"}},
			},
		})
	})
	_, c := newTestServer(t, handler)

	sg, err := c.CreateProjectSecret(context.Background(), client.CreateSecretInput{
		ProjectID:  "proj-1",
		Name:       "My Secret Group",
		SecretType: "environment",
		Priority:   10,
		Variables:  map[string]string{"KEY": "VALUE"},
	})
	if err != nil {
		t.Fatalf("CreateProjectSecret: %v", err)
	}
	if sg.ID != "my-secret-group" {
		t.Errorf("ID: want %q, got %q", "my-secret-group", sg.ID)
	}
	if sg.Variables["KEY"] != "VALUE" {
		t.Errorf("Variables[KEY]: want %q, got %q", "VALUE", sg.Variables["KEY"])
	}
}

// ─── GetProjectSecret ─────────────────────────────────────────────────────────

func TestGetProjectSecret_OK(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonWrite(w, http.StatusOK, map[string]any{
			"data": map[string]any{
				"id":         "my-secret",
				"name":       "My Secret",
				"priority":   5,
				"secretType": "environment",
				"type":       "secret",
				"tags":       []string{"tag-a"},
				"secrets":    map[string]any{"variables": map[string]string{"DB_HOST": "localhost"}},
				"projectId":  "proj-1",
				"restrictions": map[string]any{},
				"createdAt":  "2024-01-01T00:00:00Z",
				"updatedAt":  "2024-01-01T00:00:00Z",
			},
		})
	})
	_, c := newTestServer(t, handler)

	sg, err := c.GetProjectSecret(context.Background(), "proj-1", "my-secret")
	if err != nil {
		t.Fatalf("GetProjectSecret: %v", err)
	}
	if sg.Name != "My Secret" {
		t.Errorf("Name: want %q, got %q", "My Secret", sg.Name)
	}
	if sg.Variables["DB_HOST"] != "localhost" {
		t.Errorf("Variables[DB_HOST]: want %q, got %q", "localhost", sg.Variables["DB_HOST"])
	}
	if len(sg.Tags) != 1 || sg.Tags[0] != "tag-a" {
		t.Errorf("Tags: want [tag-a], got %v", sg.Tags)
	}
}

func TestGetProjectSecret_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonWrite(w, http.StatusNotFound, map[string]any{"error": "not found"})
	})
	_, c := newTestServer(t, handler)

	_, err := c.GetProjectSecret(context.Background(), "proj-1", "gone")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ─── UpdateProjectSecret ─────────────────────────────────────────────────────

func TestUpdateProjectSecret(t *testing.T) {
	var gotBody map[string]any
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		jsonWrite(w, http.StatusOK, map[string]any{
			"data": map[string]any{
				"id": "my-secret", "name": "My Secret",
				"priority": 20, "secretType": "environment",
				"type": "secret", "tags": []string{},
				"secrets": map[string]any{},
			},
		})
	})
	_, c := newTestServer(t, handler)

	newPriority := 20
	_, err := c.UpdateProjectSecret(context.Background(), client.UpdateSecretInput{
		ProjectID: "proj-1",
		SecretID:  "my-secret",
		Priority:  &newPriority,
	})
	if err != nil {
		t.Fatalf("UpdateProjectSecret: %v", err)
	}
	if v, ok := gotBody["priority"].(float64); !ok || int(v) != 20 {
		t.Errorf("expected priority=20 in PATCH body, got %v", gotBody["priority"])
	}
}

// ─── DeleteProjectSecret ─────────────────────────────────────────────────────

func TestDeleteProjectSecret_OK(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		jsonWrite(w, http.StatusOK, map[string]any{"data": map[string]any{}})
	})
	_, c := newTestServer(t, handler)

	if err := c.DeleteProjectSecret(context.Background(), "proj-1", "my-secret"); err != nil {
		t.Fatalf("DeleteProjectSecret: %v", err)
	}
}

func TestDeleteProjectSecret_AlreadyGone(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonWrite(w, http.StatusNotFound, map[string]any{"error": "not found"})
	})
	_, c := newTestServer(t, handler)

	// Deleting a resource that's already gone should not be an error.
	if err := c.DeleteProjectSecret(context.Background(), "proj-1", "gone"); err != nil {
		t.Fatalf("expected nil for 404-on-delete, got %v", err)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func isNotFound(err error) bool {
	return err != nil && (err == client.ErrNotFound ||
		// wrapped
		(func() bool {
			if uw, ok := err.(interface{ Unwrap() error }); ok {
				return isNotFound(uw.Unwrap())
			}
			return false
		}())	)
}
