package caltra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const maxResponseSize = 8 * 1024 * 1024

var problemCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// Client implements the scoped Caltra management API. A single client is
// shared by concurrently evaluated resources in a provider configuration.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewClient requires HTTPS outside loopback and refuses redirects so that a
// misconfigured API endpoint cannot forward the deployment credential.
func NewClient(baseURL, token string) (*Client, error) {
	endpoint, err := url.Parse(baseURL)
	if err != nil || endpoint.Hostname() == "" || endpoint.User != nil ||
		endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" {
		return nil, errors.New("api_url must be an absolute API URL with no user information, query or fragment")
	}
	host := endpoint.Hostname()
	loopback := host == "localhost" || net.ParseIP(host).IsLoopback()
	if endpoint.Scheme != "https" && (endpoint.Scheme != "http" || !loopback) {
		return nil, errors.New("api_url requires HTTPS outside local loopback")
	}
	return &Client{
		baseURL: strings.TrimRight(endpoint.String(), "/") + "/management/v1",
		token:   token,
		http: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// HTTPError exposes the API problem code without exposing request or response
// bodies, which can contain agent instructions and credentials.
type HTTPError struct {
	Status int
	Code   string
}

func (e *HTTPError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("Caltra management API returned HTTP %d (%s).", e.Status, e.Code)
	}
	return fmt.Sprintf("Caltra management API returned HTTP %d.", e.Status)
}

// IsNotFound distinguishes a missing managed object from a proxy or an API
// endpoint that has not been deployed. Only the former can remove state.
func IsNotFound(err error) bool {
	var response *HTTPError
	return errors.As(err, &response) && response.Status == http.StatusNotFound && response.Code == "resource_not_found"
}

func (c *Client) Organization(ctx context.Context, id string) (Organization, error) {
	var result Organization
	err := c.request(ctx, http.MethodGet, organizationPath(id), nil, &result)
	return result, err
}

func (c *Client) UpdateOrganization(ctx context.Context, id string, update UpdateOrganization) (Organization, error) {
	var result Organization
	err := c.request(ctx, http.MethodPatch, organizationPath(id), update, &result)
	return result, err
}

func (c *Client) HostedAgent(ctx context.Context, id string) (HostedAgent, error) {
	var result HostedAgent
	err := c.request(ctx, http.MethodGet, agentPath(id), nil, &result)
	return result, err
}

func (c *Client) CreateHostedAgent(ctx context.Context, create CreateHostedAgent) (HostedAgent, error) {
	var result HostedAgent
	err := c.request(ctx, http.MethodPost, "/hosted-agents", create, &result)
	return result, err
}

func (c *Client) UpdateHostedAgent(ctx context.Context, id string, settings HostedAgentSettings) (HostedAgent, error) {
	var result HostedAgent
	err := c.request(ctx, http.MethodPatch, agentPath(id), settings, &result)
	return result, err
}

func (c *Client) DeleteHostedAgent(ctx context.Context, id string) error {
	return c.request(ctx, http.MethodDelete, agentPath(id), nil, nil)
}

func (c *Client) DefaultAgent(ctx context.Context, id string) (DefaultAgent, error) {
	var result DefaultAgent
	err := c.request(ctx, http.MethodGet, organizationPath(id)+"/default-agent", nil, &result)
	return result, err
}

func (c *Client) SetDefaultAgent(ctx context.Context, id string, update SetDefaultAgent) (DefaultAgent, error) {
	var result DefaultAgent
	err := c.request(ctx, http.MethodPut, organizationPath(id)+"/default-agent", update, &result)
	return result, err
}

func (c *Client) ClearDefaultAgent(ctx context.Context, id string) error {
	return c.request(ctx, http.MethodDelete, organizationPath(id)+"/default-agent", nil, nil)
}

func organizationPath(id string) string {
	return "/organizations/" + url.PathEscape(id)
}

func agentPath(id string) string {
	return "/hosted-agents/" + url.PathEscape(id)
}

func (c *Client) request(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		payload, err := json.Marshal(input)
		if err != nil {
			return errors.New("could not encode the Caltra management API request")
		}
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return errors.New("could not construct the Caltra management API request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach the Caltra management API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		problem := struct {
			Code string `json:"code"`
		}{}
		apiError := &HTTPError{Status: resp.StatusCode}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&problem); err == nil &&
			problemCodePattern.MatchString(problem.Code) {
			apiError.Code = problem.Code
		}
		return apiError
	}
	if output == nil {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return errors.New("could not read the Caltra management API response")
	}
	if len(data) > maxResponseSize || json.Unmarshal(data, output) != nil {
		return errors.New("received an invalid response from the Caltra management API")
	}
	return nil
}
