---
page_title: "Caltra Provider"
description: |-
  Manage Caltra organization settings, system-owned hosted agents, and the organization default agent.
---

# Caltra Provider

The Caltra provider manages existing organization settings, system-owned hosted agents in existing workspaces and the organization default agent assignment.

## Configuration

Set `CALTRA_API_TOKEN` in the Terraform process environment to a scoped Caltra management token. Give plan jobs a read-only token and apply jobs a token with the write permissions needed for the managed resources.

```hcl
terraform {
  required_version = ">= 1.5"

  required_providers {
    caltra = {
      source  = "caltra-dev/caltra"
      version = "~> 0.1.0"
    }
  }
}

provider "caltra" {}
```

## Schema

### Optional

- `api_url` (String) Caltra API base URL. The `CALTRA_API_URL` environment variable is used when omitted. The default is `https://api.caltra.dev`.
