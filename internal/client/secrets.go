// Copyright (c) Vestmark
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vestmark-infra/tf-provider-northflank/internal/nfapi"
)

// SecretGroup is the canonical representation of a Northflank project secret
// group, decoupled from verbose generated nfapi type names.
type SecretGroup struct {
	// Identifiers
	ID        string
	ProjectID string

	// Fields surfaced in the Terraform schema (milestone 1: variables only)
	Name        string
	Description string
	SecretType  string // "environment" | "arguments" | "environment-arguments"
	Type        string // "secret" | "config"
	Priority    int
	Tags        []string
	Variables   map[string]string // secrets.variables
}

// CreateSecretInput carries the fields for a create request.
type CreateSecretInput struct {
	ProjectID   string
	Name        string
	Description string
	SecretType  string
	Type        string
	Priority    int
	Tags        []string
	Variables   map[string]string
}

// UpdateSecretInput carries the fields that may be patched.  Pointer fields are
// omitted from the PATCH body when nil, enabling partial updates.
type UpdateSecretInput struct {
	ProjectID   string
	SecretID    string
	Description *string
	SecretType  *string
	Type        *string
	Priority    *int
	Tags        *[]string
	Variables   *map[string]string
}

// CreateProjectSecret creates a new secret group in the given project.
func (c *Client) CreateProjectSecret(ctx context.Context, in CreateSecretInput) (*SecretGroup, error) {
	body := nfapi.PostProjectsSecretsJSONRequestBody{
		Name:       in.Name,
		SecretType: nfapi.PostProjectsSecretsJSONBodySecretType(in.SecretType),
		Priority:   in.Priority,
	}
	if in.Description != "" {
		body.Description = ptrOf(in.Description)
	}
	if in.Type != "" {
		t := nfapi.PostProjectsSecretsJSONBodyType(in.Type)
		body.Type = &t
	}
	if len(in.Tags) > 0 {
		body.Tags = &in.Tags
	}
	if len(in.Variables) > 0 {
		body.Secrets = &struct {
			DockerSecretMounts *map[string]interface{} `json:"dockerSecretMounts,omitempty"`
			Files              *map[string]interface{} `json:"files,omitempty"`
			Variables          *map[string]string      `json:"variables,omitempty"`
		}{
			Variables: &in.Variables,
		}
	}

	resp, err := c.api.PostProjectsSecretsWithResponse(ctx, in.ProjectID, body)
	if err != nil {
		return nil, fmt.Errorf("create project secret: %w", err)
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, fmt.Errorf("create project secret: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("create project secret: empty response body")
	}
	d := resp.JSON200.Data
	sg := &SecretGroup{
		ID:         d.Id,
		ProjectID:  in.ProjectID,
		Name:       d.Name,
		Priority:   d.Priority,
		SecretType: string(d.SecretType),
	}
	if d.Description != nil {
		sg.Description = *d.Description
	}
	if d.Type != nil {
		sg.Type = string(*d.Type)
	}
	if d.Tags != nil {
		sg.Tags = *d.Tags
	}
	if d.Secrets != nil && d.Secrets.Variables != nil {
		sg.Variables = *d.Secrets.Variables
	}
	return sg, nil
}

// GetProjectSecret fetches a single secret group.  Returns ErrNotFound on 404.
func (c *Client) GetProjectSecret(ctx context.Context, projectID, secretID string) (*SecretGroup, error) {
	params := &nfapi.GetProjectsSecretsSecretidParams{
		// "this" → only this group's own secrets, not inherited from addons
		Show: "this",
	}
	resp, err := c.api.GetProjectsSecretsSecretidWithResponse(ctx, projectID, secretID, params)
	if err != nil {
		return nil, fmt.Errorf("get project secret: %w", err)
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, fmt.Errorf("get project secret %s/%s: %w", projectID, secretID, err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("get project secret: empty response body")
	}
	d := resp.JSON200.Data
	sg := &SecretGroup{
		ID:         d.Id,
		ProjectID:  d.ProjectId,
		Name:       d.Name,
		Priority:   d.Priority,
		SecretType: string(d.SecretType),
		Type:       string(d.Type),
		Tags:       d.Tags,
		Variables:  extractVariables(d.Secrets),
	}
	if d.Description != nil {
		sg.Description = *d.Description
	}
	return sg, nil
}

// UpdateProjectSecret partially updates a secret group via PATCH.
func (c *Client) UpdateProjectSecret(ctx context.Context, in UpdateSecretInput) (*SecretGroup, error) {
	body := nfapi.PatchProjectsSecretsSecretidJSONRequestBody{}

	if in.Description != nil {
		body.Description = in.Description
	}
	if in.SecretType != nil {
		st := nfapi.PatchProjectsSecretsSecretidJSONBodySecretType(*in.SecretType)
		body.SecretType = &st
	}
	if in.Type != nil {
		t := nfapi.PatchProjectsSecretsSecretidJSONBodyType(*in.Type)
		body.Type = &t
	}
	if in.Priority != nil {
		body.Priority = in.Priority
	}
	if in.Tags != nil {
		body.Tags = in.Tags
	}
	if in.Variables != nil {
		body.Secrets = &struct {
			DockerSecretMounts *map[string]interface{} `json:"dockerSecretMounts,omitempty"`
			Files              *map[string]interface{} `json:"files,omitempty"`
			Variables          *map[string]string      `json:"variables,omitempty"`
		}{
			Variables: in.Variables,
		}
	}

	resp, err := c.api.PatchProjectsSecretsSecretidWithResponse(ctx, in.ProjectID, in.SecretID, body)
	if err != nil {
		return nil, fmt.Errorf("update project secret: %w", err)
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, fmt.Errorf("update project secret %s/%s: %w", in.ProjectID, in.SecretID, err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("update project secret: empty response body")
	}
	d := resp.JSON200.Data
	sg := &SecretGroup{
		ID:         d.Id,
		ProjectID:  in.ProjectID,
		Name:       d.Name,
		Priority:   d.Priority,
		SecretType: string(d.SecretType),
	}
	if d.Description != nil {
		sg.Description = *d.Description
	}
	if d.Type != nil {
		sg.Type = string(*d.Type)
	}
	if d.Tags != nil {
		sg.Tags = *d.Tags
	}
	if d.Secrets != nil && d.Secrets.Variables != nil {
		sg.Variables = *d.Secrets.Variables
	}
	return sg, nil
}

// DeleteProjectSecret deletes a secret group.  A 404 is treated as success.
func (c *Client) DeleteProjectSecret(ctx context.Context, projectID, secretID string) error {
	resp, err := c.api.DeleteProjectsSecretsSecretidWithResponse(ctx, projectID, secretID)
	if err != nil {
		return fmt.Errorf("delete project secret: %w", err)
	}
	if resp.StatusCode() == 404 {
		return nil // already gone — idempotent
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return fmt.Errorf("delete project secret %s/%s: %w", projectID, secretID, err)
	}
	return nil
}

// extractVariables reads the "variables" key from the GET response Secrets
// map[string]interface{} and returns it as map[string]string.
func extractVariables(secrets map[string]interface{}) map[string]string {
	raw, ok := secrets["variables"]
	if !ok || raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var vars map[string]string
	if err := json.Unmarshal(b, &vars); err != nil {
		return nil
	}
	return vars
}
