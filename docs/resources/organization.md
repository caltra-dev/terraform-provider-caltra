---
page_title: "caltra_organization Resource"
description: |-
  Adopt an existing Caltra organization and manage its name and hosted runtime defaults.
---

# caltra_organization (Resource)

Import an existing Caltra organization to manage its name and hosted model configuration. Caltra handles organization creation and closure. Use `terraform state rm` to end Terraform management while keeping the organization.

```hcl
resource "caltra_organization" "example" {
  name = "Example organization"

  lifecycle {
    prevent_destroy = true
  }
}
```

## Schema

### Required

- `name` (String) Organization display name.

### Optional

- `model_id` (String) UUID of the organization default hosted model. Omit this field to preserve the Caltra setting.
- `reasoning_level` (String) Organization default hosted reasoning level. Omit this field to preserve the Caltra setting.

### Read-only

- `id` (String) Organization UUID.

## Import

```sh
terraform import caltra_organization.example ORGANIZATION_UUID
```
