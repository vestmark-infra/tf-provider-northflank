// Copyright (c) Vestmark
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vestmark-infra/tf-provider-northflank/internal/client"
)

// Compile-time interface assertions.
var (
	_ resource.Resource                = &SecretResource{}
	_ resource.ResourceWithConfigure   = &SecretResource{}
	_ resource.ResourceWithImportState = &SecretResource{}
)

// NewSecretResource returns a resource factory function for northflank_secret.
func NewSecretResource() resource.Resource {
	return &SecretResource{}
}

// SecretResource manages a Northflank project secret group.
type SecretResource struct {
	client *client.Client
}

// SecretResourceModel is the Terraform state model for northflank_secret.
type SecretResourceModel struct {
	// Computed identifiers
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`

	// Required config
	Name       types.String `tfsdk:"name"`
	SecretType types.String `tfsdk:"secret_type"`
	Priority   types.Int64  `tfsdk:"priority"`

	// Optional config
	Description types.String `tfsdk:"description"`
	Type        types.String `tfsdk:"type"`
	Tags        types.List   `tfsdk:"tags"`

	// The secret variables (sensitive — masked in plan/apply output).
	Variables types.Map `tfsdk:"variables"`
}

func (r *SecretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

func (r *SecretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Northflank project secret group. Secret groups hold environment variables, files, and docker secret mounts that are injected into project services and jobs.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Server-generated identifier (derived slug of `name`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the project that this secret group belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the secret group (3–100 chars; alphanumeric with hyphens or spaces between words).",
			},
			"secret_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Injection scope. One of `environment`, `arguments`, or `environment-arguments`.",
			},
			"priority": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Merge priority (0–100). Higher value wins when multiple groups define the same key.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Human-readable description of the secret group.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Hierarchy type. One of `secret` (default) or `config` (plaintext config values).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tags": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Tags attached to the secret group.",
			},
			"variables": schema.MapAttribute{
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
				MarkdownDescription: "Environment variable key/value pairs. Values are encrypted at rest. Keys may only contain letters, numbers, hyphens, forward slashes, and dots.",
			},
		},
	}
}

func (r *SecretResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // happens during early schema validation
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	r.client = c
}

// Create provisions a new secret group and saves state.
func (r *SecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SecretResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := client.CreateSecretInput{
		ProjectID:  plan.ProjectID.ValueString(),
		Name:       plan.Name.ValueString(),
		SecretType: plan.SecretType.ValueString(),
		Priority:   int(plan.Priority.ValueInt64()),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		in.Description = plan.Description.ValueString()
	}
	if !plan.Type.IsNull() && !plan.Type.IsUnknown() {
		in.Type = plan.Type.ValueString()
	}
	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &in.Tags, false)...)
	}
	if !plan.Variables.IsNull() && !plan.Variables.IsUnknown() {
		vars := make(map[string]string)
		resp.Diagnostics.Append(plan.Variables.ElementsAs(ctx, &vars, false)...)
		in.Variables = vars
	}
	if resp.Diagnostics.HasError() {
		return
	}

	sg, err := r.client.CreateProjectSecret(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("Error creating secret group", err.Error())
		return
	}

	secretGroupToState(ctx, sg, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes state from the API.
func (r *SecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SecretResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sg, err := r.client.GetProjectSecret(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx) // drift: resource deleted outside Terraform
			return
		}
		resp.Diagnostics.AddError("Error reading secret group", err.Error())
		return
	}

	secretGroupToState(ctx, sg, &state, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update applies changes via PATCH.
func (r *SecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state SecretResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := client.UpdateSecretInput{
		ProjectID: state.ProjectID.ValueString(),
		SecretID:  state.ID.ValueString(),
	}

	// Only send fields that changed.
	if !plan.Description.Equal(state.Description) {
		v := plan.Description.ValueString()
		in.Description = &v
	}
	if !plan.SecretType.Equal(state.SecretType) {
		v := plan.SecretType.ValueString()
		in.SecretType = &v
	}
	if !plan.Type.Equal(state.Type) {
		v := plan.Type.ValueString()
		in.Type = &v
	}
	if !plan.Priority.Equal(state.Priority) {
		v := int(plan.Priority.ValueInt64())
		in.Priority = &v
	}
	if !plan.Tags.Equal(state.Tags) {
		var tags []string
		resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
		in.Tags = &tags
	}
	if !plan.Variables.Equal(state.Variables) {
		vars := make(map[string]string)
		resp.Diagnostics.Append(plan.Variables.ElementsAs(ctx, &vars, false)...)
		in.Variables = &vars
	}
	if resp.Diagnostics.HasError() {
		return
	}

	sg, err := r.client.UpdateProjectSecret(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("Error updating secret group", err.Error())
		return
	}

	secretGroupToState(ctx, sg, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the secret group.
func (r *SecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SecretResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteProjectSecret(ctx, state.ProjectID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting secret group", err.Error())
	}
}

// ImportState handles `terraform import northflank_secret.x project-id/secret-id`.
func (r *SecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Import ID must be in the format <project_id>/<secret_id>. Got: %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// secretGroupToState populates a SecretResourceModel from a client.SecretGroup.
func secretGroupToState(ctx context.Context, sg *client.SecretGroup, m *SecretResourceModel, diagnostics *diag.Diagnostics) {
	m.ID = types.StringValue(sg.ID)
	m.ProjectID = types.StringValue(sg.ProjectID)
	m.Name = types.StringValue(sg.Name)
	m.SecretType = types.StringValue(sg.SecretType)
	m.Priority = types.Int64Value(int64(sg.Priority))
	m.Description = types.StringValue(sg.Description)
	m.Type = types.StringValue(sg.Type)

	tags, tagsDiag := types.ListValueFrom(ctx, types.StringType, sg.Tags)
	diagnostics.Append(tagsDiag...)
	m.Tags = tags

	vars, varsDiag := types.MapValueFrom(ctx, types.StringType, sg.Variables)
	diagnostics.Append(varsDiag...)
	m.Variables = vars
}
