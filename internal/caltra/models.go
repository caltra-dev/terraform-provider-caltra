package caltra

type Organization struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	ModelID        *string `json:"model_id"`
	ReasoningLevel *string `json:"reasoning_level"`
}

// UpdateOrganization omits unconfigured defaults so that the API preserves them.
type UpdateOrganization struct {
	Name           string  `json:"name"`
	ModelID        *string `json:"model_id,omitempty"`
	ReasoningLevel *string `json:"reasoning_level,omitempty"`
}

type HostedAgent struct {
	ID             string        `json:"id"`
	WorkspaceID    string        `json:"workspace_id"`
	ExternalID     string        `json:"external_id"`
	Name           string        `json:"name"`
	Instructions   string        `json:"instructions"`
	ModelID        *string       `json:"model_id"`
	ReasoningLevel *string       `json:"reasoning_level"`
	Capabilities   []string      `json:"capabilities"`
	ToolLoading    []ToolLoading `json:"tool_loading"`
}

// HostedAgentSettings sends null overrides to select inherited settings and
// limits tool updates to the pairs under this resource's ownership.
type HostedAgentSettings struct {
	Name           string        `json:"name"`
	Instructions   string        `json:"instructions"`
	ModelID        *string       `json:"model_id"`
	ReasoningLevel *string       `json:"reasoning_level"`
	Capabilities   []string      `json:"capabilities"`
	ToolLoading    []ToolLoading `json:"tool_loading,omitempty"`
}

type CreateHostedAgent struct {
	WorkspaceID string `json:"workspace_id"`
	ExternalID  string `json:"external_id"`
	HostedAgentSettings
}

type ToolLoading struct {
	BackendID string  `json:"backend_id"`
	Name      string  `json:"name"`
	Mode      *string `json:"mode"`
}

type DefaultAgent struct {
	OrganizationID  string `json:"organization_id"`
	AgentExternalID string `json:"agent_external_id"`
}

type SetDefaultAgent struct {
	AgentExternalID string `json:"agent_external_id"`
}
