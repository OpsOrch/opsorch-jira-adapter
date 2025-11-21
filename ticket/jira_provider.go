package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/opsorch/opsorch-core/schema"
	coreticket "github.com/opsorch/opsorch-core/ticket"
)

// ProviderName is the registry key under which this adapter registers.
const ProviderName = "jira"

// AdapterVersion and RequiresCore express compatibility.
const (
	AdapterVersion = "0.1.0"
	RequiresCore   = ">=0.1.0"
)

var errNotFound = errors.New("ticket not found")

// Config captures decrypted configuration from OpsOrch Core.
type Config struct {
	Source           string
	APIToken         string
	APIURL           string
	ProjectKey       string
	DefaultIssueType string
}

// JiraProvider integrates with Jira REST API v3.
type JiraProvider struct {
	cfg    Config
	client *http.Client
}

// New constructs the provider from decrypted config.
func New(cfg map[string]any) (coreticket.Provider, error) {
	parsed := parseConfig(cfg)
	if parsed.APIToken == "" {
		return nil, errors.New("jira apiToken is required")
	}
	if parsed.ProjectKey == "" {
		return nil, errors.New("jira projectKey is required")
	}
	if parsed.APIURL == "" {
		return nil, errors.New("jira apiURL is required")
	}
	return &JiraProvider{
		cfg:    parsed,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func parseConfig(cfg map[string]any) Config {
	out := Config{
		Source:           "jira",
		APIURL:           "https://your-domain.atlassian.net",
		DefaultIssueType: "Task",
	}
	if v, ok := cfg["source"].(string); ok && v != "" {
		out.Source = v
	}
	if v, ok := cfg["apiToken"].(string); ok {
		out.APIToken = strings.TrimSpace(v)
	}
	if v, ok := cfg["apiURL"].(string); ok && v != "" {
		out.APIURL = strings.TrimSpace(v)
	}
	if v, ok := cfg["projectKey"].(string); ok {
		out.ProjectKey = strings.TrimSpace(v)
	}
	if v, ok := cfg["defaultIssueType"].(string); ok && v != "" {
		out.DefaultIssueType = v
	}
	return out
}

func init() {
	_ = coreticket.RegisterProvider(ProviderName, New)
}

// Create creates a new Jira issue.
func (p *JiraProvider) Create(ctx context.Context, in schema.CreateTicketInput) (schema.Ticket, error) {
	payload := map[string]any{
		"fields": map[string]any{
			"project": map[string]string{
				"key": p.cfg.ProjectKey,
			},
			"summary": in.Title,
			"issuetype": map[string]string{
				"name": p.cfg.DefaultIssueType,
			},
		},
	}

	if in.Description != "" {
		payload["fields"].(map[string]any)["description"] = map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []map[string]any{
				{
					"type": "paragraph",
					"content": []map[string]any{
						{
							"type": "text",
							"text": in.Description,
						},
					},
				},
			},
		}
	}

	// Add custom fields if provided
	if in.Fields != nil {
		for k, v := range in.Fields {
			payload["fields"].(map[string]any)[k] = v
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return schema.Ticket{}, fmt.Errorf("marshal create payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.cfg.APIURL+"/rest/api/3/issue", bytes.NewReader(body))
	if err != nil {
		return schema.Ticket{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return schema.Ticket{}, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return schema.Ticket{}, fmt.Errorf("jira api error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Self string `json:"self"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return schema.Ticket{}, fmt.Errorf("decode response: %w", err)
	}

	// Fetch the created issue to get full details
	return p.Get(ctx, result.Key)
}

// Get retrieves a single Jira issue by ID or key.
func (p *JiraProvider) Get(ctx context.Context, id string) (schema.Ticket, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.cfg.APIURL+"/rest/api/3/issue/"+id, nil)
	if err != nil {
		return schema.Ticket{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.cfg.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return schema.Ticket{}, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return schema.Ticket{}, errNotFound
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return schema.Ticket{}, fmt.Errorf("jira api error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var issue jiraIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return schema.Ticket{}, fmt.Errorf("decode response: %w", err)
	}

	return convertJiraIssue(issue, p.cfg.Source), nil
}

// Query searches for Jira issues using JQL.
func (p *JiraProvider) Query(ctx context.Context, q schema.TicketQuery) ([]schema.Ticket, error) {
	jql := buildJQL(q, p.cfg.ProjectKey)

	params := url.Values{}
	params.Set("jql", jql)
	if q.Limit > 0 {
		params.Set("maxResults", fmt.Sprintf("%d", q.Limit))
	} else {
		params.Set("maxResults", "50")
	}

	reqURL := p.cfg.APIURL + "/rest/api/3/search?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.cfg.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira api error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Issues []jiraIssue `json:"issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	tickets := make([]schema.Ticket, len(result.Issues))
	for i, issue := range result.Issues {
		tickets[i] = convertJiraIssue(issue, p.cfg.Source)
	}

	return tickets, nil
}

func buildJQL(q schema.TicketQuery, projectKey string) string {
	var clauses []string

	// Always filter by project
	clauses = append(clauses, fmt.Sprintf("project = %s", projectKey))

	// Free-text search
	if q.Query != "" {
		clauses = append(clauses, fmt.Sprintf("text ~ \"%s\"", escapeJQL(q.Query)))
	}

	// Status filter
	if len(q.Statuses) > 0 {
		statuses := make([]string, len(q.Statuses))
		for i, s := range q.Statuses {
			statuses[i] = fmt.Sprintf("\"%s\"", escapeJQL(s))
		}
		clauses = append(clauses, fmt.Sprintf("status IN (%s)", strings.Join(statuses, ",")))
	}

	// Assignee filter
	if len(q.Assignees) > 0 {
		assignees := make([]string, len(q.Assignees))
		for i, a := range q.Assignees {
			assignees[i] = fmt.Sprintf("\"%s\"", escapeJQL(a))
		}
		clauses = append(clauses, fmt.Sprintf("assignee IN (%s)", strings.Join(assignees, ",")))
	}

	// Reporter filter
	if q.Reporter != "" {
		clauses = append(clauses, fmt.Sprintf("reporter = \"%s\"", escapeJQL(q.Reporter)))
	}

	// Order by key descending (newest first)
	jql := strings.Join(clauses, " AND ")
	jql += " ORDER BY key DESC"

	return jql
}

func escapeJQL(s string) string {
	// Escape quotes in JQL strings
	return strings.ReplaceAll(s, "\"", "\\\"")
}

// Update modifies a Jira issue.
func (p *JiraProvider) Update(ctx context.Context, id string, in schema.UpdateTicketInput) (schema.Ticket, error) {
	payload := map[string]any{
		"fields": map[string]any{},
	}

	if in.Title != nil {
		payload["fields"].(map[string]any)["summary"] = *in.Title
	}

	if in.Description != nil {
		payload["fields"].(map[string]any)["description"] = map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []map[string]any{
				{
					"type": "paragraph",
					"content": []map[string]any{
						{
							"type": "text",
							"text": *in.Description,
						},
					},
				},
			},
		}
	}

	if in.Assignees != nil && len(*in.Assignees) > 0 {
		// Jira only supports single assignee, use first one
		payload["fields"].(map[string]any)["assignee"] = map[string]string{
			"accountId": (*in.Assignees)[0],
		}
	}

	// Add custom fields if provided
	if in.Fields != nil {
		for k, v := range in.Fields {
			payload["fields"].(map[string]any)[k] = v
		}
	}

	// Handle status transitions separately if provided
	if in.Status != nil {
		if err := p.transitionIssue(ctx, id, *in.Status); err != nil {
			return schema.Ticket{}, fmt.Errorf("transition issue: %w", err)
		}
	}

	// Only send update if there are fields to update
	if len(payload["fields"].(map[string]any)) > 0 {
		body, err := json.Marshal(payload)
		if err != nil {
			return schema.Ticket{}, fmt.Errorf("marshal update payload: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "PUT", p.cfg.APIURL+"/rest/api/3/issue/"+id, bytes.NewReader(body))
		if err != nil {
			return schema.Ticket{}, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+p.cfg.APIToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return schema.Ticket{}, fmt.Errorf("execute request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return schema.Ticket{}, errNotFound
		}

		if resp.StatusCode != http.StatusNoContent {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return schema.Ticket{}, fmt.Errorf("jira api error: %d %s", resp.StatusCode, string(bodyBytes))
		}
	}

	// Fetch updated issue
	return p.Get(ctx, id)
}

func (p *JiraProvider) transitionIssue(ctx context.Context, id string, targetStatus string) error {
	// Get available transitions
	req, err := http.NewRequestWithContext(ctx, "GET", p.cfg.APIURL+"/rest/api/3/issue/"+id+"/transitions", nil)
	if err != nil {
		return fmt.Errorf("create transitions request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.cfg.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute transitions request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("get transitions error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var transitionsResp struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name string `json:"name"`
			} `json:"to"`
		} `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&transitionsResp); err != nil {
		return fmt.Errorf("decode transitions: %w", err)
	}

	// Find transition that leads to target status
	var transitionID string
	for _, t := range transitionsResp.Transitions {
		if strings.EqualFold(t.To.Name, targetStatus) {
			transitionID = t.ID
			break
		}
	}

	if transitionID == "" {
		return fmt.Errorf("no transition found to status: %s", targetStatus)
	}

	// Execute transition
	transitionPayload := map[string]any{
		"transition": map[string]string{
			"id": transitionID,
		},
	}

	body, err := json.Marshal(transitionPayload)
	if err != nil {
		return fmt.Errorf("marshal transition payload: %w", err)
	}

	req, err = http.NewRequestWithContext(ctx, "POST", p.cfg.APIURL+"/rest/api/3/issue/"+id+"/transitions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create transition request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err = p.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute transition request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("transition error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// jiraIssue represents a Jira issue from the API.
type jiraIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary     string `json:"summary"`
		Description struct {
			Content []struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"content"`
		} `json:"description"`
		Status struct {
			Name string `json:"name"`
		} `json:"status"`
		Assignee *struct {
			AccountID   string `json:"accountId"`
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
		Reporter *struct {
			AccountID   string `json:"accountId"`
			DisplayName string `json:"displayName"`
		} `json:"reporter"`
		Created string `json:"created"`
		Updated string `json:"updated"`
	} `json:"fields"`
}

func convertJiraIssue(issue jiraIssue, source string) schema.Ticket {
	ticket := schema.Ticket{
		ID:       issue.ID,
		Key:      issue.Key,
		Title:    issue.Fields.Summary,
		Status:   issue.Fields.Status.Name,
		Metadata: map[string]any{"source": source},
	}

	// Extract description text
	var descParts []string
	for _, content := range issue.Fields.Description.Content {
		for _, textContent := range content.Content {
			if textContent.Text != "" {
				descParts = append(descParts, textContent.Text)
			}
		}
	}
	ticket.Description = strings.Join(descParts, " ")

	// Extract assignees
	if issue.Fields.Assignee != nil {
		ticket.Assignees = []string{issue.Fields.Assignee.AccountID}
	}

	// Extract reporter
	if issue.Fields.Reporter != nil {
		ticket.Reporter = issue.Fields.Reporter.AccountID
	}

	// Parse timestamps
	if createdAt, err := time.Parse(time.RFC3339, issue.Fields.Created); err == nil {
		ticket.CreatedAt = createdAt
	}
	if updatedAt, err := time.Parse(time.RFC3339, issue.Fields.Updated); err == nil {
		ticket.UpdatedAt = updatedAt
	}

	return ticket
}
