---
page_title: "caltra_hosted_agent Resource"
description: |-
  Manage a system-owned hosted agent in a Caltra workspace.
---

# caltra_hosted_agent (Resource)

Manage a system-owned hosted agent in an existing workspace. Changing its workspace or external ID requires replacement. Caltra rejects removal when the agent has retained sessions or is still referenced by an organization default.

```hcl
resource "caltra_hosted_agent" "example" {
  workspace_id = var.workspace_id
  external_id  = "catalog-assistant"
  name         = "Catalog assistant"
  instructions = "Answer questions about products using the catalog."
  capabilities = ["web_search"]

  tool_loading {
    backend_id = var.backend_id
    name       = "find_product"
    mode       = "summary"
  }
}
```

## Schema

### Required

- `workspace_id` (String) UUID of the existing workspace.
- `external_id` (String) Stable external identifier in the workspace's system-owned agent namespace.
- `name` (String) Agent display name.
- `instructions` (String) Instructions the agent uses. The content is stored in Terraform state.

### Optional

- `model_id` (String) UUID of the hosted model override. Omit to inherit workspace or organization model policy.
- `reasoning_level` (String) Hosted reasoning level override. Requires `model_id`.
- `capabilities` (Set of String) Built-in capabilities to enable. Omit for an empty set. Supported values are `attachments`, `web_search`, `background_execution`, `display_artifacts`, `interactive_approval` and `receive_external_events`.
- `tool_loading` (Block Set) Loading modes for explicitly managed backend and tool pairs. An existing grant is required. Removing a block resets its mode to `on_demand` and keeps the grant.

### Read-only

- `id` (String) Durable agent UUID.

### Nested set for `tool_loading`

- `backend_id` (String, Required) UUID of the tool's backend.
- `name` (String, Required) Tool name published by the backend.
- `mode` (String, Required) One of `on_demand`, `summary` or `full_definition`.

## Import

Import using the durable agent UUID. This form adopts the agent and leaves all existing tool loading settings under their current management.

```sh
terraform import caltra_hosted_agent.example AGENT_UUID
```

To adopt specific tool loading settings without changing them, append each declared backend and tool pair to the import ID and use its current mode in the resource configuration.

```sh
terraform import caltra_hosted_agent.example 'AGENT_UUID,BACKEND_UUID:find_product'
```
