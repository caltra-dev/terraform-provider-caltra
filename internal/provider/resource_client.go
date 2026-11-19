package provider

import (
	"context"
	"fmt"

	"github.com/caltra-dev/terraform-provider-caltra/internal/caltra"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// resourceClient binds the shared provider client to each managed resource.
// Schema validation can run before the provider has built a client.
type resourceClient struct {
	client *caltra.Client
}

func (r *resourceClient) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*caltra.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Caltra provider data",
			fmt.Sprintf("Expected a Caltra API client, got %T.", req.ProviderData))
		return
	}
	r.client = client
}
