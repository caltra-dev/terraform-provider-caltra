package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/caltra-dev/terraform-provider-caltra/internal/caltra"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = (*hostedAgentResource)(nil)
	_ resource.ResourceWithConfigure      = (*hostedAgentResource)(nil)
	_ resource.ResourceWithImportState    = (*hostedAgentResource)(nil)
	_ resource.ResourceWithValidateConfig = (*hostedAgentResource)(nil)
)

// hostedAgentResource configures a system-owned hosted agent in an existing
// workspace, with tool loading ownership limited to explicitly selected pairs.
type hostedAgentResource struct {
	resourceClient
}

type hostedAgentModel struct {
	ID             types.String `tfsdk:"id"`
	WorkspaceID    types.String `tfsdk:"workspace_id"`
	ExternalID     types.String `tfsdk:"external_id"`
	Name           types.String `tfsdk:"name"`
	Instructions   types.String `tfsdk:"instructions"`
	ModelID        types.String `tfsdk:"model_id"`
	ReasoningLevel types.String `tfsdk:"reasoning_level"`
	Capabilities   types.Set    `tfsdk:"capabilities"`
	ToolLoading    types.Set    `tfsdk:"tool_loading"`
}

type toolLoadingModel struct {
	BackendID types.String `tfsdk:"backend_id"`
	Name      types.String `tfsdk:"name"`
	Mode      types.String `tfsdk:"mode"`
}

var toolLoadingType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"backend_id": types.StringType,
		"name":       types.StringType,
		"mode":       types.StringType,
	},
}

// toolKey keeps equally named tools in separate backends independent.
type toolKey struct {
	backendID string
	name      string
}

func (r *hostedAgentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hosted_agent"
}

func (r *hostedAgentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a system-owned hosted agent in a Caltra workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Durable agent UUID.",
			},
			"workspace_id": schema.StringAttribute{
				Required:    true,
				Description: "UUID of the existing workspace that owns the hosted agent.",
				Validators:  []validator.String{uuidValidator{}},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"external_id": schema.StringAttribute{
				Required:    true,
				Description: "Stable external identifier in the workspace's system-owned agent namespace.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Agent display name.",
			},
			"instructions": schema.StringAttribute{
				Required:    true,
				Description: "Agent instructions.",
			},
			"model_id": schema.StringAttribute{
				Optional:    true,
				Description: "UUID of the hosted model override. Omit to inherit the workspace or organization policy.",
				Validators:  []validator.String{uuidValidator{}},
			},
			"reasoning_level": schema.StringAttribute{
				Optional:    true,
				Description: "Hosted reasoning level override. Requires model_id.",
			},
			"capabilities": schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Default:     setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
				Description: "Enabled built-in capabilities. Defaults to the empty set.",
			},
		},
		Blocks: map[string]schema.Block{
			"tool_loading": schema.SetNestedBlock{
				Description: "Loading mode for a tool with an existing grant. Removing the block resets the mode to on_demand.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"backend_id": schema.StringAttribute{
							Required:    true,
							Description: "UUID of the tool's backend.",
							Validators:  []validator.String{uuidValidator{}},
						},
						"name": schema.StringAttribute{
							Required:    true,
							Description: "Tool name published by the backend.",
						},
						"mode": schema.StringAttribute{
							Required:    true,
							Description: "One of on_demand, summary, or full_definition.",
						},
					},
				},
			},
		},
	}
}

func (r *hostedAgentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config hostedAgentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if config.ModelID.IsNull() && !config.ReasoningLevel.IsNull() && !config.ReasoningLevel.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("reasoning_level"), "Caltra model override required",
			"Set model_id to use a reasoning level override for this agent.")
	}
	if config.ToolLoading.IsUnknown() {
		return
	}

	var tools []toolLoadingModel
	resp.Diagnostics.Append(config.ToolLoading.ElementsAs(ctx, &tools, false)...)
	seen := make(map[toolKey]bool, len(tools))
	for _, tool := range tools {
		if tool.BackendID.IsUnknown() || tool.Name.IsUnknown() {
			continue
		}
		key := toolKey{backendID: tool.BackendID.ValueString(), name: tool.Name.ValueString()}
		if seen[key] {
			resp.Diagnostics.AddAttributeError(path.Root("tool_loading"), "Duplicate tool loading entry",
				"Declare each tool backend ID and name pair at most once.")
			return
		}
		seen[key] = true
	}
}

func (r *hostedAgentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan hostedAgentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	desired := readToolLoading(ctx, plan.ToolLoading, &resp.Diagnostics)
	settings := plan.settings(ctx, desired, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	agent, err := r.client.CreateHostedAgent(ctx, caltra.CreateHostedAgent{
		WorkspaceID:         plan.WorkspaceID.ValueString(),
		ExternalID:          plan.ExternalID.ValueString(),
		HostedAgentSettings: settings,
	})
	var problem *caltra.HTTPError
	if errors.As(err, &problem) && problem.Code == "agent_already_exists" {
		resp.Diagnostics.AddError("Caltra hosted agent already exists",
			"Import the existing agent by its durable UUID before managing it with Terraform.")
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Could not create the Caltra hosted agent", err.Error())
		return
	}
	writeHostedAgentState(ctx, &resp.State, &resp.Diagnostics, agent, desired, true)
}

func (r *hostedAgentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hostedAgentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	managed := readToolLoading(ctx, state.ToolLoading, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	agent, err := r.client.HostedAgent(ctx, state.ID.ValueString())
	if caltra.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Could not read the Caltra hosted agent", err.Error())
		return
	}
	writeHostedAgentState(ctx, &resp.State, &resp.Diagnostics, agent, managed, false)
}

func (r *hostedAgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state hostedAgentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	desired := readToolLoading(ctx, plan.ToolLoading, &resp.Diagnostics)
	previous := readToolLoading(ctx, state.ToolLoading, &resp.Diagnostics)
	settings := plan.settings(ctx, toolLoadingUpdates(desired, previous), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	agent, err := r.client.UpdateHostedAgent(ctx, state.ID.ValueString(), settings)
	if err != nil {
		resp.Diagnostics.AddError("Could not update the Caltra hosted agent", err.Error())
		return
	}
	writeHostedAgentState(ctx, &resp.State, &resp.Diagnostics, agent, desired, true)
}

func (r *hostedAgentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hostedAgentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteHostedAgent(ctx, state.ID.ValueString())
	if err != nil && !caltra.IsNotFound(err) {
		resp.Diagnostics.AddError("Could not remove the Caltra hosted agent", err.Error())
	}
}

func (r *hostedAgentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fields := strings.Split(req.ID, ",")
	agentID := fields[0]
	if !validUUID(agentID) {
		resp.Diagnostics.AddError("Invalid Caltra hosted agent import ID", "Provide the durable agent UUID.")
		return
	}

	// Import receives no HCL configuration. Explicit pairs let Terraform adopt
	// selected tool settings without claiming any other grants on the agent.
	tools := make([]toolLoadingModel, 0, len(fields)-1)
	seen := make(map[toolKey]bool, len(fields)-1)
	for _, item := range fields[1:] {
		backendID, name, ok := strings.Cut(item, ":")
		key := toolKey{backendID: backendID, name: name}
		if !ok || !validUUID(backendID) || name == "" || seen[key] {
			resp.Diagnostics.AddError("Invalid Caltra tool loading import selection",
				"After the agent UUID, append each selected pair once as ,BACKEND_UUID:TOOL_NAME.")
			return
		}
		seen[key] = true
		tools = append(tools, toolLoadingModel{
			BackendID: types.StringValue(backendID),
			Name:      types.StringValue(name),
			Mode:      types.StringValue("on_demand"),
		})
	}
	toolSet, diagnostics := types.SetValueFrom(ctx, toolLoadingType, tools)
	resp.Diagnostics.Append(diagnostics...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(agentID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tool_loading"), toolSet)...)
}

func (plan hostedAgentModel) settings(ctx context.Context, tools []caltra.ToolLoading, diagnostics *diag.Diagnostics) caltra.HostedAgentSettings {
	capabilities := make([]string, 0, len(plan.Capabilities.Elements()))
	diagnostics.Append(plan.Capabilities.ElementsAs(ctx, &capabilities, false)...)
	return caltra.HostedAgentSettings{
		Name:           plan.Name.ValueString(),
		Instructions:   plan.Instructions.ValueString(),
		ModelID:        plan.ModelID.ValueStringPointer(),
		ReasoningLevel: plan.ReasoningLevel.ValueStringPointer(),
		Capabilities:   capabilities,
		ToolLoading:    tools,
	}
}

func readToolLoading(ctx context.Context, set types.Set, diagnostics *diag.Diagnostics) []caltra.ToolLoading {
	var values []toolLoadingModel
	diagnostics.Append(set.ElementsAs(ctx, &values, false)...)
	result := make([]caltra.ToolLoading, 0, len(values))
	for _, tool := range values {
		result = append(result, caltra.ToolLoading{
			BackendID: tool.BackendID.ValueString(),
			Name:      tool.Name.ValueString(),
			Mode:      tool.Mode.ValueStringPointer(),
		})
	}
	return result
}

// toolLoadingUpdates sends the desired modes and resets previously owned
// pairs removed from configuration. Unrelated pairs remain under API control.
func toolLoadingUpdates(desired, previous []caltra.ToolLoading) []caltra.ToolLoading {
	active := make(map[toolKey]bool, len(desired))
	updates := make([]caltra.ToolLoading, 0, len(desired)+len(previous))
	for _, tool := range desired {
		active[toolKey{backendID: tool.BackendID, name: tool.Name}] = true
		updates = append(updates, tool)
	}
	for _, tool := range previous {
		if !active[toolKey{backendID: tool.BackendID, name: tool.Name}] {
			tool.Mode = nil
			updates = append(updates, tool)
		}
	}
	return updates
}

func writeHostedAgentState(ctx context.Context, state *tfsdk.State, diagnostics *diag.Diagnostics, agent caltra.HostedAgent, managed []caltra.ToolLoading, applied bool) {
	capabilities := agent.Capabilities
	if capabilities == nil {
		capabilities = []string{}
	}
	capabilitySet, result := types.SetValueFrom(ctx, types.StringType, capabilities)
	diagnostics.Append(result...)

	modes := make(map[toolKey]*string, len(agent.ToolLoading))
	for _, tool := range agent.ToolLoading {
		modes[toolKey{backendID: tool.BackendID, name: tool.Name}] = tool.Mode
	}
	tools := make([]toolLoadingModel, 0, len(managed))
	for _, tool := range managed {
		mode := modes[toolKey{backendID: tool.BackendID, name: tool.Name}]
		if applied && (mode == nil || tool.Mode == nil || *mode != *tool.Mode) {
			diagnostics.AddAttributeError(path.Root("tool_loading"), "Caltra tool loading change was not reflected by the API",
				"Ensure the tool is active, enabled for this agent and granted to the workspace system principal. If adopting an existing agent, import its settings first.")
			return
		}
		// Keep ownership when a grant disappears. A null mode shows drift and
		// still lets a later removal reset the previously managed setting.
		tools = append(tools, toolLoadingModel{
			BackendID: types.StringValue(tool.BackendID),
			Name:      types.StringValue(tool.Name),
			Mode:      types.StringPointerValue(mode),
		})
	}
	toolSet, result := types.SetValueFrom(ctx, toolLoadingType, tools)
	diagnostics.Append(result...)
	if diagnostics.HasError() {
		return
	}
	diagnostics.Append(state.Set(ctx, hostedAgentModel{
		ID:             types.StringValue(agent.ID),
		WorkspaceID:    types.StringValue(agent.WorkspaceID),
		ExternalID:     types.StringValue(agent.ExternalID),
		Name:           types.StringValue(agent.Name),
		Instructions:   types.StringValue(agent.Instructions),
		ModelID:        types.StringPointerValue(agent.ModelID),
		ReasoningLevel: types.StringPointerValue(agent.ReasoningLevel),
		Capabilities:   capabilitySet,
		ToolLoading:    toolSet,
	})...)
}
