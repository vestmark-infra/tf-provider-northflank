// Copyright (c) Vestmark
// SPDX-License-Identifier: MPL-2.0

// Package client provides a thin, hand-written wrapper around the auto-generated
// nfapi Northflank REST client.  It handles authentication injection, maps API
// error codes to typed sentinel errors, and exposes a stable interface that
// insulates the provider from generated symbol churn.
package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/vestmark-infra/tf-provider-northflank/internal/nfapi"
)

const defaultBaseURL = "https://api.northflank.com"

// ErrNotFound is returned when the Northflank API responds with HTTP 404.
// The provider's Read function uses this to remove a resource from state.
var ErrNotFound = errors.New("northflank: resource not found (404)")

// Client wraps the generated nfapi.ClientWithResponses and injects Bearer auth.
type Client struct {
	api *nfapi.ClientWithResponses
}

// New creates a Client.  If baseURL is empty, the public API URL is used.
func New(baseURL, token string) (*Client, error) {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	bearer := func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
	api, err := nfapi.NewClientWithResponses(baseURL, nfapi.WithRequestEditorFn(bearer))
	if err != nil {
		return nil, fmt.Errorf("northflank client init: %w", err)
	}
	return &Client{api: api}, nil
}

// apiError represents a non-2xx, non-404 response from the Northflank API.
type apiError struct {
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("northflank API error %d: %s", e.StatusCode, e.Body)
}

// checkStatus returns nil for 2xx, ErrNotFound for 404, and an apiError otherwise.
func checkStatus(code int, rawBody []byte) error {
	if code >= 200 && code < 300 {
		return nil
	}
	if code == http.StatusNotFound {
		return ErrNotFound
	}
	return &apiError{StatusCode: code, Body: string(rawBody)}
}

// ptrOf returns a pointer to v.
func ptrOf[T any](v T) *T { return &v }
