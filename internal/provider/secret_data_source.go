// Copyright (c) Vestmark
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vestmark-infra/tf-provider-northflank/internal/client"
)

// Compile-time interface assertion.
var _ datasource.DataSourceWithConfigure = &SecretDataSource{}

// NewSecretDataSource returns a data source factory function for northflank_secret.
func NewSecretDataSource() datasource.DataSource {
	return &SecretDataSource{}
}

// SecretDataSource reads an existing Northflank project secret group.
type SecretDataSource struct {
	client *client.Client
}

// SecretDataSourceModel is the Terraform state model for the northflank_secret data source.
type SecretDataSourceModel struct {
	// Required lookup keys
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`

	// Read-only output attributes
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	SecretType  types.String `tfsdk:"secret_type"`
	Type        types.String `tfsdk:"type"`
	Priority    types.Int64  `tfsdk:"priority"`
	Tags        types.List   `tfsdk:"tags"`
	Variables   types.Map    `tfsdk:"variables"`
}

func (d *SecretDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

func (d *SecretDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Northflank project secret group. Useful for consuming secrets managed outside of this Terraform configuration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the secret group (the derived slug of its name).",
			},
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the project that the secret group belongs to.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the secret group.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the secret group.",
			},
			"secret_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Injection scope: `environment`, `arguments`, or `environment-arguments`.",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Hierarchy type: `secret` or `config`.",
			},
			"priority": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Merge priority (0–100).",
			},
			"tags": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Tags attached to the secret group.",
			},
			"variables": schema.MapAttribute{
				Computed:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
				MarkdownDescription: "Decrypted environment variable key/value pairs.",
			},
		},
	}
}

func (d *SecretDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	d.client = c
}

// Read fetches the secret group and populates state.
func (d *SecretDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SecretDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sg, err := d.client.GetProjectSecret(ctx, data.ProjectID.ValueString(), data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.Diagnostics.AddError(
				"Secret group not found",
				fmt.Sprintf("No secret group %q in project %q.", data.ID.ValueString(), data.ProjectID.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError("Error reading secret group", err.Error())
		return
	}

	data.ID = types.StringValue(sg.ID)
	data.ProjectID = types.StringValue(sg.ProjectID)
	data.Name = types.StringValue(sg.Name)
	data.Description = types.StringValue(sg.Description)
	data.SecretType = types.StringValue(sg.SecretType)
	data.Type = types.StringValue(sg.Type)
	data.Priority = types.Int64Value(int64(sg.Priority))

	tags, _ := types.ListValueFrom(ctx, types.StringType, sg.Tags)
	data.Tags = tags

	vars, _ := types.MapValueFrom(ctx, types.StringType, sg.Variables)
	data.Variables = vars

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
