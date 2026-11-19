package provider

import (
	"context"

	"github.com/caltra-dev/terraform-provider-caltra/internal/caltra"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*defaultAgentResource)(nil)
	_ resource.ResourceWithConfigure   = (*defaultAgentResource)(nil)
	_ resource.ResourceWithImportState = (*defaultAgentResource)(nil)
)

// defaultAgentResource manages the organization-wide logical key for its
// default agent. Caltra resolves the key inside each caller's workspace.
type defaultAgentResource struct {
	resourceClient
}

type defaultAgentModel struct {
	ID              types.String `tfsdk:"id"`
	OrganizationID  types.String `tfsdk:"organization_id"`
	AgentExternalID types.String `tfsdk:"agent_external_id"`
}

func (r *defaultAgentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_default_agent"
}

func (r *defaultAgentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Assign the external ID of the hosted agent used for new sessions that request an organization default.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Organization UUID, which identifies its singleton default assignment.",
			},
			"organization_id": schema.StringAttribute{
				Required:    true,
				Description: "UUID of the organization that owns the default assignment.",
				Validators:  []validator.String{uuidValidator{}},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"agent_external_id": schema.StringAttribute{
				Required:    true,
				Description: "External ID of the system-owned hosted agent, resolved in the requesting workspace.",
			},
		},
	}
}

func (r *defaultAgentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.write(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *defaultAgentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state defaultAgentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	assignment, err := r.client.DefaultAgent(ctx, state.ID.ValueString())
	if caltra.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Could not read the Caltra default agent assignment", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, defaultAgentState(assignment))...)
}

func (r *defaultAgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.write(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *defaultAgentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state defaultAgentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.ClearDefaultAgent(ctx, state.ID.ValueString())
	if err != nil && !caltra.IsNotFound(err) {
		resp.Diagnostics.AddError("Could not clear the Caltra default agent assignment", err.Error())
	}
}

func (r *defaultAgentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !validUUID(req.ID) {
		resp.Diagnostics.AddError("Invalid Caltra default agent import ID", "Provide the organization UUID.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func defaultAgentState(assignment caltra.DefaultAgent) defaultAgentModel {
	return defaultAgentModel{
		ID:              types.StringValue(assignment.OrganizationID),
		OrganizationID:  types.StringValue(assignment.OrganizationID),
		AgentExternalID: types.StringValue(assignment.AgentExternalID),
	}
}

func (r *defaultAgentResource) write(ctx context.Context, plan tfsdk.Plan, result *tfsdk.State, diagnostics *diag.Diagnostics) {
	var planned defaultAgentModel
	diagnostics.Append(plan.Get(ctx, &planned)...)
	if diagnostics.HasError() {
		return
	}
	assignment, err := r.client.SetDefaultAgent(ctx, planned.OrganizationID.ValueString(), caltra.SetDefaultAgent{
		AgentExternalID: planned.AgentExternalID.ValueString(),
	})
	if err != nil {
		diagnostics.AddError("Could not assign the Caltra default agent", err.Error())
		return
	}
	diagnostics.Append(result.Set(ctx, defaultAgentState(assignment))...)
}
