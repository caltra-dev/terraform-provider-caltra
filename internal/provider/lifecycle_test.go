package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/caltra-dev/terraform-provider-caltra/internal/caltra"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	organizationUUID = "b79e6aa4-aa35-4914-8de8-036a7f35482b"
	workspaceUUID    = "5326001b-1206-40f8-9e1f-3862e1f89f06"
	agentUUID        = "8176080e-8b19-4ad6-aba2-2764d53fedf3"
	managedBackend   = "1f4c2340-e964-404a-a0e4-fa73d30a733e"
	unmanagedBackend = "25a67fcb-e496-4c0b-a6f4-bec0e3637991"
	toolName         = "find_item"
	fixtureModel     = "d508556e-46a7-4596-9811-621328620ad2"
	driftModel       = "23d763a6-ef9e-4d8c-ba59-8e30018aa24b"
)

func TestTerraformHostedAgentLifecycle(t *testing.T) {
	api := &lifecycleAPI{t: t}
	server := httptest.NewServer(api)
	defer server.Close()
	t.Setenv("CALTRA_API_URL", server.URL)
	t.Setenv("CALTRA_API_TOKEN", "test-write")

	managedConfig := lifecycleConfig(fmt.Sprintf(`
	tool_loading {
		backend_id = %q
		name = %q
		mode = "summary"
	}
`, managedBackend, toolName))
	updatedConfig := lifecycleConfig(fmt.Sprintf("model_id = %q", fixtureModel))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"caltra": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: managedConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("caltra_hosted_agent.example", "id", agentUUID),
					resource.TestCheckNoResourceAttr("caltra_hosted_agent.example", "model_id"),
					resource.TestCheckResourceAttr("caltra_hosted_agent.example", "tool_loading.#", "1"),
					resource.TestCheckResourceAttr("caltra_organization_default_agent.example", "agent_external_id", "catalog"),
				),
			},
			{
				ResourceName:      "caltra_hosted_agent.example",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s,%s:%s", agentUUID, managedBackend, toolName), nil
				},
			},
			{
				ResourceName:      "caltra_organization_default_agent.example",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("caltra_hosted_agent.example", "id", agentUUID),
					resource.TestCheckResourceAttr("caltra_hosted_agent.example", "model_id", fixtureModel),
					resource.TestCheckResourceAttr("caltra_hosted_agent.example", "tool_loading.#", "0"),
					api.checkToolModes,
				),
			},
			{
				Config: updatedConfig,
				PreConfig: func() {
					api.mu.Lock()
					defer api.mu.Unlock()
					drifted := driftModel
					api.agent.ModelID = &drifted
					t.Setenv("CALTRA_API_TOKEN", "test-read")
				},
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config:    updatedConfig,
				PreConfig: func() { t.Setenv("CALTRA_API_TOKEN", "test-write") },
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("caltra_hosted_agent.example", "model_id", fixtureModel),
					api.checkToolModes,
				),
			},
		},
		CheckDestroy: func(_ *terraform.State) error {
			api.mu.Lock()
			defer api.mu.Unlock()
			if api.agent != nil || api.assignment != nil {
				return fmt.Errorf("agent or default assignment was retained after Terraform destroy")
			}
			return nil
		},
	})
}

func lifecycleConfig(agentSettings string) string {
	return fmt.Sprintf(`
resource "caltra_hosted_agent" "example" {
	workspace_id = %q
	external_id = "catalog"
	name = "Catalog"
	instructions = "Answer catalog questions."
	%s
}

resource "caltra_organization_default_agent" "example" {
	organization_id = %q
	agent_external_id = caltra_hosted_agent.example.external_id
}
`, workspaceUUID, agentSettings, organizationUUID)
}

// lifecycleAPI stands in for the remote API while Terraform and the provider
// run their normal create, import, refresh, update, and destroy operations.
type lifecycleAPI struct {
	t          *testing.T
	mu         sync.Mutex
	agent      *caltra.HostedAgent
	assignment *caltra.DefaultAgent
}

func (a *lifecycleAPI) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch req.Header.Get("Authorization") {
	case "Bearer test-read":
		if req.Method != http.MethodGet {
			a.t.Errorf("read-only Terraform plan attempted %s", req.Method)
			w.WriteHeader(http.StatusForbidden)
			return
		}
	case "Bearer test-write":
	default:
		a.t.Error("Terraform request did not carry the configured credential")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch req.URL.Path {
	case "/management/v1/hosted-agents":
		if req.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var create caltra.CreateHostedAgent
		if !a.decode(w, req, &create) {
			return
		}
		initialMode := "full_definition"
		a.agent = &caltra.HostedAgent{
			ID:          agentUUID,
			WorkspaceID: create.WorkspaceID,
			ExternalID:  create.ExternalID,
			ToolLoading: []caltra.ToolLoading{{BackendID: unmanagedBackend, Name: toolName, Mode: &initialMode}},
		}
		a.applySettings(create.HostedAgentSettings)
		a.encode(w, http.StatusCreated, a.agent)
	case "/management/v1/hosted-agents/" + agentUUID:
		a.serveAgent(w, req)
	case "/management/v1/organizations/" + organizationUUID + "/default-agent":
		a.serveDefault(w, req)
	default:
		a.t.Errorf("unexpected Terraform API path: %s", req.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (a *lifecycleAPI) serveAgent(w http.ResponseWriter, req *http.Request) {
	if a.agent == nil {
		a.notFound(w)
		return
	}
	switch req.Method {
	case http.MethodGet:
		a.encode(w, http.StatusOK, a.agent)
	case http.MethodPatch:
		var settings caltra.HostedAgentSettings
		if a.decode(w, req, &settings) {
			a.applySettings(settings)
			a.encode(w, http.StatusOK, a.agent)
		}
	case http.MethodDelete:
		if a.assignment != nil {
			w.WriteHeader(http.StatusConflict)
			return
		}
		a.agent = nil
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *lifecycleAPI) serveDefault(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		if a.assignment == nil {
			a.notFound(w)
			return
		}
		a.encode(w, http.StatusOK, a.assignment)
	case http.MethodPut:
		var update caltra.SetDefaultAgent
		if a.decode(w, req, &update) {
			a.assignment = &caltra.DefaultAgent{OrganizationID: organizationUUID, AgentExternalID: update.AgentExternalID}
			a.encode(w, http.StatusOK, a.assignment)
		}
	case http.MethodDelete:
		a.assignment = nil
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *lifecycleAPI) applySettings(settings caltra.HostedAgentSettings) {
	a.agent.Name = settings.Name
	a.agent.Instructions = settings.Instructions
	a.agent.ModelID = settings.ModelID
	a.agent.ReasoningLevel = settings.ReasoningLevel
	a.agent.Capabilities = settings.Capabilities
	for _, tool := range settings.ToolLoading {
		if tool.BackendID != managedBackend || tool.Name != toolName {
			a.t.Errorf("Terraform tried to manage %s:%s outside the declared pair", tool.BackendID, tool.Name)
			continue
		}
		if tool.Mode == nil {
			onDemand := "on_demand"
			tool.Mode = &onDemand
		}
		if len(a.agent.ToolLoading) == 1 {
			a.agent.ToolLoading = append(a.agent.ToolLoading, tool)
		} else {
			a.agent.ToolLoading[1] = tool
		}
	}
}

func (a *lifecycleAPI) decode(w http.ResponseWriter, req *http.Request, input any) bool {
	if err := json.NewDecoder(req.Body).Decode(input); err != nil {
		a.t.Errorf("could not decode Terraform API request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return false
	}
	return true
}

func (a *lifecycleAPI) encode(w http.ResponseWriter, status int, output any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(output); err != nil {
		a.t.Errorf("could not encode fixture response: %v", err)
	}
}

func (a *lifecycleAPI) checkToolModes(_ *terraform.State) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.agent == nil || len(a.agent.ToolLoading) != 2 {
		return fmt.Errorf("expected both tool grants to remain")
	}
	unmanaged, managed := a.agent.ToolLoading[0], a.agent.ToolLoading[1]
	if unmanaged.Mode == nil || *unmanaged.Mode != "full_definition" {
		return fmt.Errorf("Terraform changed the unmanaged loading mode")
	}
	if managed.Mode == nil || *managed.Mode != "on_demand" {
		return fmt.Errorf("Terraform did not reset the removed loading mode")
	}
	return nil
}

func (a *lifecycleAPI) notFound(w http.ResponseWriter) {
	a.encode(w, http.StatusNotFound, map[string]string{"code": "resource_not_found"})
}
