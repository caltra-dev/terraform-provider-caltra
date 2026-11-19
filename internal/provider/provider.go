package provider

import (
	"context"
	"os"
	"strings"

	"github.com/caltra-dev/terraform-provider-caltra/internal/caltra"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const defaultAPIURL = "https://api.caltra.dev"

var _ provider.Provider = (*caltraProvider)(nil)

// caltraProvider configures the management client at the provider boundary.
// The credential is injected via the environment and never enters HCL or state.
type caltraProvider struct {
	version string
}

type providerConfig struct {
	APIURL types.String `tfsdk:"api_url"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &caltraProvider{version: version}
	}
}

func (p *caltraProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "caltra"
	resp.Version = p.version
}

func (p *caltraProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Caltra organizations, hosted agents, and default agent assignments.",
		Attributes: map[string]schema.Attribute{
			"api_url": schema.StringAttribute{
				Optional:    true,
				Description: "Caltra API base URL. Defaults to CALTRA_API_URL or https://api.caltra.dev.",
			},
		},
	}
}

func (p *caltraProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerConfig
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if config.APIURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("api_url"), "Unknown Caltra API URL",
			"Set api_url to a value that can be resolved before planning.")
		return
	}

	apiURL := defaultAPIURL
	if fromEnv := os.Getenv("CALTRA_API_URL"); fromEnv != "" {
		apiURL = fromEnv
	}
	if !config.APIURL.IsNull() {
		apiURL = config.APIURL.ValueString()
	}
	token := os.Getenv("CALTRA_API_TOKEN")
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		resp.Diagnostics.AddError("Missing or invalid Caltra API token",
			"Set CALTRA_API_TOKEN to a scoped Caltra management API token.")
		return
	}
	client, err := caltra.NewClient(apiURL, token)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("api_url"), "Invalid Caltra API URL", err.Error())
		return
	}
	resp.ResourceData = client
}

func (p *caltraProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource { return &organizationResource{} },
		func() resource.Resource { return &hostedAgentResource{} },
		func() resource.Resource { return &defaultAgentResource{} },
	}
}

func (p *caltraProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
