terraform {
  required_version = ">= 1.5"

  required_providers {
    caltra = {
      source  = "caltra-dev/caltra"
      version = "~> 0.1.0"
    }
  }
}

variable "organization_id" {
  description = "UUID of the existing Caltra organization."
  type        = string
}

variable "organization_name" {
  description = "Name of the existing Caltra organization."
  type        = string
}

variable "workspace_id" {
  description = "UUID of the Caltra workspace for the agent."
  type        = string
}

import {
  to = caltra_organization.example
  id = var.organization_id
}

resource "caltra_organization" "example" {
  name = var.organization_name

  lifecycle {
    prevent_destroy = true
  }
}

resource "caltra_hosted_agent" "example" {
  workspace_id = var.workspace_id
  external_id  = "catalog-assistant"
  name         = "Catalog assistant"
  instructions = "Answer questions about the product catalog."
  capabilities = ["web_search"]
}

resource "caltra_organization_default_agent" "example" {
  organization_id   = caltra_organization.example.id
  agent_external_id = caltra_hosted_agent.example.external_id
}
