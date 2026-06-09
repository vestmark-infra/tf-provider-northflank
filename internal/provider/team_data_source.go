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

var _ datasource.DataSourceWithConfigure = &TeamDataSource{}

func NewTeamDataSource() datasource.DataSource { return &TeamDataSource{} }

// TeamDataSource looks up a Northflank team by name and exposes its ID.
type TeamDataSource struct {
	client *client.Client
}

type TeamDataSourceModel struct {
	Name        types.String `tfsdk:"name"`
	ID          types.String `tfsdk:"id"`
	Description types.String `tfsdk:"description"`
}

func (d *TeamDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (d *TeamDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Northflank team by name. Use the resulting `id` to scope project lookups in multi-team configurations.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name of the team to look up.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Team identifier.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Team description.",
			},
		},
	}
}

func (d *TeamDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TeamDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TeamDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teams, err := d.client.ListTeams(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing teams", err.Error())
		return
	}

	name := data.Name.ValueString()
	var matches []client.Team
	for _, t := range teams {
		if t.Name == name {
			matches = append(matches, t)
		}
	}

	switch len(matches) {
	case 0:
		resp.Diagnostics.AddError(
			"Team not found",
			fmt.Sprintf("No team with name %q was found. Available teams: %s", name, teamNames(teams)),
		)
		return
	case 1:
		// exact match
	default:
		resp.Diagnostics.AddError(
			"Ambiguous team name",
			fmt.Sprintf("%d teams match name %q. Use the team ID directly.", len(matches), name),
		)
		return
	}

	t := matches[0]
	data.ID = types.StringValue(t.ID)
	data.Description = types.StringValue(t.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func teamNames(teams []client.Team) string {
	names := make([]string, 0, len(teams))
	for _, t := range teams {
		names = append(names, fmt.Sprintf("%q", t.Name))
	}
	if len(names) == 0 {
		return "(none)"
	}
	result := names[0]
	for _, n := range names[1:] {
		result += ", " + n
	}
	return result
}
