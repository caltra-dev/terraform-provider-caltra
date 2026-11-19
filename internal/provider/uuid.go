package provider

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && value == parsed.String()
}

// uuidValidator requires the canonical representation returned by the API so
// a successful apply always has the same ID Terraform planned.
type uuidValidator struct{}

func (uuidValidator) Description(_ context.Context) string {
	return "Must be a UUID in lowercase canonical form."
}

func (v uuidValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v uuidValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() && !validUUID(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Caltra ID", v.Description(ctx))
	}
}
