---
page_title: "caltra_organization_default_agent Resource"
description: |-
  Manage the logical default hosted agent for a Caltra organization.
---

# caltra_organization_default_agent (Resource)

Assign the logical agent used when a new session requests the organization's default. Caltra resolves the external ID among active system-owned hosted agents in the requesting workspace and checks the caller's access. Session requests that select an agent explicitly use their explicit selection.

Destroying this resource clears the assignment. Agents and their sessions are retained.

```hcl
resource "caltra_organization_default_agent" "example" {
  organization_id   = caltra_organization.example.id
  agent_external_id = caltra_hosted_agent.example.external_id
}
```

## Schema

### Required

- `organization_id` (String) UUID of the organization that owns the assignment.
- `agent_external_id` (String) Stable external identifier of the default agent.

### Read-only

- `id` (String) Organization UUID, which identifies its singleton assignment.

## Import

```sh
terraform import caltra_organization_default_agent.example ORGANIZATION_UUID
```
