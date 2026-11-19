package provider

import (
	"context"

	"github.com/caltra-dev/terraform-provider-caltra/internal/caltra"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*organizationResource)(nil)
	_ resource.ResourceWithConfigure   = (*organizationResource)(nil)
	_ resource.ResourceWithImportState = (*organizationResource)(nil)
)

// organizationResource adopts an existing organization and updates its
// Terraform-managed settings while leaving account and billing ownership intact.
type organizationResource struct {
	resourceClient
}

type organizationModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	ModelID        types.String `tfsdk:"model_id"`
	ReasoningLevel types.String `tfsdk:"reasoning_level"`
}

func (r *organizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (r *organizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Adopt an existing Caltra organization to manage its name and hosted runtime defaults.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Organization UUID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Organization display name.",
			},
			"model_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "UUID of the organization default hosted model. Omit to preserve the API value.",
				Validators:  []validator.String{uuidValidator{}},
			},
			"reasoning_level": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Organization default hosted reasoning level. Omit to preserve the API value.",
			},
		},
	}
}

func (r *organizationResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError("Caltra organizations must be imported",
		"Create the organization in Caltra, then import it using its UUID.")
}

func (r *organizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state organizationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	org, err := r.client.Organization(ctx, state.ID.ValueString())
	if caltra.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Could not read the Caltra organization", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, organizationState(org))...)
}

func (r *organizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state, config organizationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	org, err := r.client.UpdateOrganization(ctx, state.ID.ValueString(), caltra.UpdateOrganization{
		Name:           config.Name.ValueString(),
		ModelID:        config.ModelID.ValueStringPointer(),
		ReasoningLevel: config.ReasoningLevel.ValueStringPointer(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Could not update the Caltra organization", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, organizationState(org))...)
}

func (r *organizationResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddError("Caltra organizations cannot be destroyed by Terraform",
		"To stop managing this organization while keeping it in Caltra, remove the resource from Terraform state.")
}

func (r *organizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !validUUID(req.ID) {
		resp.Diagnostics.AddError("Invalid Caltra organization import ID", "Provide the organization UUID.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func organizationState(org caltra.Organization) organizationModel {
	return organizationModel{
		ID:             types.StringValue(org.ID),
		Name:           types.StringValue(org.Name),
		ModelID:        types.StringPointerValue(org.ModelID),
		ReasoningLevel: types.StringPointerValue(org.ReasoningLevel),
	}
}
