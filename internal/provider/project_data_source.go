// Copyright (c) Vestmark
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vestmark-infra/tf-provider-northflank/internal/client"
)

var _ datasource.DataSourceWithConfigure = &ProjectDataSource{}

func NewProjectDataSource() datasource.DataSource { return &ProjectDataSource{} }

// ProjectDataSource looks up a Northflank project by name, optionally scoped to
// a specific team via team_id.  When team_id is set the team-scoped
// /v1/teams/{teamId}/projects endpoint is used; otherwise the token-implicit
// /v1/projects endpoint is used.
type ProjectDataSource struct {
	client *client.Client
}

type ProjectDataSourceModel struct {
	Name        types.String `tfsdk:"name"`
	TeamID      types.String `tfsdk:"team_id"`
	ID          types.String `tfsdk:"id"`
	Description types.String `tfsdk:"description"`
}

func (d *ProjectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *ProjectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Northflank project by name. When `team_id` is provided the search is scoped to that team's projects (required for multi-team configurations). Use the resulting `id` as `project_id` in `northflank_secret` resources.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the project to look up.",
			},
			"team_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "ID of the team to scope the search to. Obtain this from a `northflank_team` data source. When omitted, searches projects accessible to the token's implicit team.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project identifier.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project description.",
			},
		},
	}
}

func (d *ProjectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *ProjectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var projects []client.Project
	var err error

	if !data.TeamID.IsNull() && !data.TeamID.IsUnknown() && data.TeamID.ValueString() != "" {
		projects, err = d.client.ListTeamProjects(ctx, data.TeamID.ValueString())
	} else {
		projects, err = d.client.ListProjects(ctx)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error listing projects", err.Error())
		return
	}

	name := data.Name.ValueString()
	var matches []client.Project
	for _, p := range projects {
		if p.Name == name {
			matches = append(matches, p)
		}
	}

	switch len(matches) {
	case 0:
		resp.Diagnostics.AddError(
			"Project not found",
			fmt.Sprintf("No project with name %q was found.", name),
		)
		return
	case 1:
		// exact match
	default:
		resp.Diagnostics.AddError(
			"Ambiguous project name",
			fmt.Sprintf("%d projects match name %q. Use `id` directly or narrow with `team_id`.", len(matches), name),
		)
		return
	}

	p := matches[0]
	data.ID = types.StringValue(p.ID)
	data.Description = types.StringValue(p.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
