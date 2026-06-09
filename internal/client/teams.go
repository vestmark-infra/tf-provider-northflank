// Copyright (c) Vestmark
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"fmt"

	"github.com/vestmark-infra/tf-provider-northflank/internal/nfapi"
)

// Team is a Northflank team (org sub-unit).
type Team struct {
	ID          string
	Name        string
	Description string
}

// Project is a Northflank project.
type Project struct {
	ID          string
	Name        string
	Description string
}

// ListTeams returns all teams visible to the token, fetching all pages.
func (c *Client) ListTeams(ctx context.Context) ([]Team, error) {
	var teams []Team
	var cursor *string

	for {
		params := &nfapi.GetTeamsParams{PerPage: ptrOf(100)}
		if cursor != nil {
			params.Cursor = cursor
		}
		resp, err := c.api.GetTeamsWithResponse(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("list teams: %w", err)
		}
		if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, fmt.Errorf("list teams: %w", err)
		}
		if resp.JSON200 == nil {
			break
		}
		for _, t := range resp.JSON200.Data.Teams {
			tm := Team{ID: t.Id, Name: t.Name}
			if t.Description != nil {
				tm.Description = *t.Description
			}
			teams = append(teams, tm)
		}
		pg := resp.JSON200.Pagination
		if !pg.HasNextPage || pg.Cursor == nil {
			break
		}
		cursor = pg.Cursor
	}
	return teams, nil
}

// ListProjects returns all projects for the token's implicit team, fetching all pages.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var projects []Project
	var cursor *string

	for {
		params := &nfapi.GetProjectsParams{PerPage: ptrOf(100)}
		if cursor != nil {
			params.Cursor = cursor
		}
		resp, err := c.api.GetProjectsWithResponse(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("list projects: %w", err)
		}
		if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, fmt.Errorf("list projects: %w", err)
		}
		if resp.JSON200 == nil {
			break
		}
		for _, p := range resp.JSON200.Data.Projects {
			proj := Project{ID: p.Id, Name: p.Name}
			if p.Description != nil {
				proj.Description = *p.Description
			}
			projects = append(projects, proj)
		}
		pg := resp.JSON200.Pagination
		if !pg.HasNextPage || pg.Cursor == nil {
			break
		}
		cursor = pg.Cursor
	}
	return projects, nil
}

// ListTeamProjects returns all projects belonging to a specific team, fetching all pages.
func (c *Client) ListTeamProjects(ctx context.Context, teamID string) ([]Project, error) {
	var projects []Project
	var cursor *string

	for {
		params := &nfapi.GetTeamProjectsParams{PerPage: ptrOf(100)}
		if cursor != nil {
			params.Cursor = cursor
		}
		resp, err := c.api.GetTeamProjectsWithResponse(ctx, teamID, params)
		if err != nil {
			return nil, fmt.Errorf("list team projects: %w", err)
		}
		if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, fmt.Errorf("list team %s projects: %w", teamID, err)
		}
		if resp.JSON200 == nil {
			break
		}
		for _, p := range resp.JSON200.Data.Projects {
			proj := Project{ID: p.Id, Name: p.Name}
			if p.Description != nil {
				proj.Description = *p.Description
			}
			projects = append(projects, proj)
		}
		pg := resp.JSON200.Pagination
		if !pg.HasNextPage || pg.Cursor == nil {
			break
		}
		cursor = pg.Cursor
	}
	return projects, nil
}
