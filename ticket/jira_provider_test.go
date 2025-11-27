package ticket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opsorch/opsorch-core/schema"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]any
		expect Config
	}{
		{
			name: "valid config with all fields",
			input: map[string]any{
				"source":           "custom-jira",
				"apiToken":         "test-token",
				"apiURL":           "https://example.atlassian.net",
				"projectKey":       "PROJ",
				"defaultIssueType": "Bug",
			},
			expect: Config{
				Source:           "custom-jira",
				APIToken:         "test-token",
				APIURL:           "https://example.atlassian.net",
				ProjectKey:       "PROJ",
				DefaultIssueType: "Bug",
			},
		},
		{
			name: "config with defaults",
			input: map[string]any{
				"apiToken":   "test-token",
				"projectKey": "PROJ",
			},
			expect: Config{
				Source:           "jira",
				APIToken:         "test-token",
				APIURL:           "https://your-domain.atlassian.net",
				ProjectKey:       "PROJ",
				DefaultIssueType: "Task",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseConfig(tt.input)
			if got.Source != tt.expect.Source {
				t.Errorf("Source = %v, want %v", got.Source, tt.expect.Source)
			}
			if got.APIToken != tt.expect.APIToken {
				t.Errorf("APIToken = %v, want %v", got.APIToken, tt.expect.APIToken)
			}
			if got.APIURL != tt.expect.APIURL {
				t.Errorf("APIURL = %v, want %v", got.APIURL, tt.expect.APIURL)
			}
			if got.ProjectKey != tt.expect.ProjectKey {
				t.Errorf("ProjectKey = %v, want %v", got.ProjectKey, tt.expect.ProjectKey)
			}
			if got.DefaultIssueType != tt.expect.DefaultIssueType {
				t.Errorf("DefaultIssueType = %v, want %v", got.DefaultIssueType, tt.expect.DefaultIssueType)
			}
		})
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]any
		expectErr bool
	}{
		{
			name: "valid config",
			config: map[string]any{
				"apiToken":   "test-token",
				"email":      "test@example.com",
				"projectKey": "PROJ",
			},
			expectErr: false,
		},
		{
			name: "missing apiToken",
			config: map[string]any{
				"projectKey": "PROJ",
			},
			expectErr: true,
		},
		{
			name: "missing projectKey",
			config: map[string]any{
				"apiToken": "test-token",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.config)
			if (err != nil) != tt.expectErr {
				t.Errorf("New() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue" && r.Method == "POST" {
			// Verify auth header (Basic Auth)
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// Parse request body
			var payload map[string]any
			json.NewDecoder(r.Body).Decode(&payload)

			// Return created issue
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"id":   "10001",
				"key":  "PROJ-1",
				"self": "/rest/api/3/issue/10001",
			})
			return
		}

		if strings.HasPrefix(r.URL.Path, "/rest/api/3/issue/") && r.Method == "GET" {
			// Return issue details
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"id":  "10001",
				"key": "PROJ-1",
				"fields": map[string]any{
					"summary": "Test ticket",
					"description": map[string]any{
						"content": []map[string]any{
							{
								"content": []map[string]any{
									{"text": "Test description"},
								},
							},
						},
					},
					"status": map[string]any{
						"name": "To Do",
					},
					"created": "2025-11-21T10:00:00Z",
					"updated": "2025-11-21T10:00:00Z",
				},
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &JiraProvider{
		cfg: Config{
			Source:           "jira",
			Email:            "test@example.com",
			APIToken:         "test-token",
			APIURL:           server.URL,
			ProjectKey:       "PROJ",
			DefaultIssueType: "Task",
		},
		client: &http.Client{},
	}
	ctx := context.Background()

	t.Run("create with minimal input", func(t *testing.T) {
		in := schema.CreateTicketInput{
			Title: "Test ticket",
		}
		ticket, err := p.Create(ctx, in)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if ticket.ID != "10001" {
			t.Errorf("ID = %v, want 10001", ticket.ID)
		}
		if ticket.Key != "PROJ-1" {
			t.Errorf("Key = %v, want PROJ-1", ticket.Key)
		}
		if ticket.Title != "Test ticket" {
			t.Errorf("Title = %v, want Test ticket", ticket.Title)
		}
		if ticket.Status != "To Do" {
			t.Errorf("Status = %v, want To Do", ticket.Status)
		}
		if ticket.Metadata["source"] != "jira" {
			t.Errorf("Metadata[source] = %v, want jira", ticket.Metadata["source"])
		}
	})

	t.Run("create with description", func(t *testing.T) {
		in := schema.CreateTicketInput{
			Title:       "Test ticket",
			Description: "Test description",
		}
		ticket, err := p.Create(ctx, in)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if ticket.Description != "Test description" {
			t.Errorf("Description = %v, want Test description", ticket.Description)
		}
	})
}

func TestGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/PROJ-1" && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"id":  "10001",
				"key": "PROJ-1",
				"fields": map[string]any{
					"summary": "Test ticket",
					"description": map[string]any{
						"content": []map[string]any{
							{
								"content": []map[string]any{
									{"text": "Test description"},
								},
							},
						},
					},
					"status": map[string]any{
						"name": "To Do",
					},
					"priority": map[string]any{
						"id":   "3",
						"name": "High",
					},
					"issuetype": map[string]any{
						"id":   "10001",
						"name": "Bug",
					},
					"labels":     []string{"backend", "urgent"},
					"components": []map[string]any{{"id": "10000", "name": "API"}},
					"assignee": map[string]any{
						"accountId":   "user123",
						"displayName": "Alice",
					},
					"reporter": map[string]any{
						"accountId":   "user456",
						"displayName": "Bob",
					},
					"created": "2025-11-21T10:00:00Z",
					"updated": "2025-11-21T10:00:00Z",
				},
			})
			return
		}

		if r.URL.Path == "/rest/api/3/issue/NOTFOUND" && r.Method == "GET" {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{
				"errorMessages": []string{"Issue does not exist"},
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &JiraProvider{
		cfg: Config{
			Source:     "jira",
			Email:      "test@example.com",
			APIToken:   "test-token",
			APIURL:     server.URL,
			ProjectKey: "PROJ",
		},
		client: &http.Client{},
	}
	ctx := context.Background()

	t.Run("get existing ticket", func(t *testing.T) {
		ticket, err := p.Get(ctx, "PROJ-1")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if ticket.ID != "10001" {
			t.Errorf("ID = %v, want 10001", ticket.ID)
		}
		if ticket.Key != "PROJ-1" {
			t.Errorf("Key = %v, want PROJ-1", ticket.Key)
		}
		if ticket.Title != "Test ticket" {
			t.Errorf("Title = %v, want Test ticket", ticket.Title)
		}
		if ticket.Description != "Test description" {
			t.Errorf("Description = %v, want Test description", ticket.Description)
		}
		if len(ticket.Assignees) != 1 || ticket.Assignees[0] != "user123" {
			t.Errorf("Assignees = %v, want [user123]", ticket.Assignees)
		}
		if ticket.Reporter != "user456" {
			t.Errorf("Reporter = %v, want user456", ticket.Reporter)
		}
		if ticket.Metadata["priority"] != "High" {
			t.Errorf("Metadata[priority] = %v, want High", ticket.Metadata["priority"])
		}
		if ticket.Metadata["issue_type"] != "Bug" {
			t.Errorf("Metadata[issue_type] = %v, want Bug", ticket.Metadata["issue_type"])
		}
		if labels, ok := ticket.Metadata["labels"].([]string); !ok || len(labels) != 2 {
			t.Errorf("Metadata[labels] = %v, want []string with 2 items", ticket.Metadata["labels"])
		}
		if components, ok := ticket.Metadata["components"].([]string); !ok || len(components) != 1 {
			t.Errorf("Metadata[components] = %v, want []string with 1 item", ticket.Metadata["components"])
		}
	})

	t.Run("get non-existent ticket", func(t *testing.T) {
		_, err := p.Get(ctx, "NOTFOUND")
		if err != errNotFound {
			t.Errorf("Get() error = %v, want errNotFound", err)
		}
	})
}

func TestQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" && r.Method == "POST" {
			var payload map[string]any
			json.NewDecoder(r.Body).Decode(&payload)
			jql, _ := payload["jql"].(string)

			var issues []map[string]any

			// Return different results based on JQL
			if strings.Contains(jql, "text ~ \"login\"") {
				issues = []map[string]any{
					{
						"id":  "10001",
						"key": "PROJ-1",
						"fields": map[string]any{
							"summary":     "Fix login bug",
							"description": map[string]any{"content": []map[string]any{}},
							"status":      map[string]any{"name": "To Do"},
							"created":     "2025-11-21T10:00:00Z",
							"updated":     "2025-11-21T10:00:00Z",
						},
					},
				}
			} else if strings.Contains(jql, "status IN (\"Done\")") {
				issues = []map[string]any{
					{
						"id":  "10002",
						"key": "PROJ-2",
						"fields": map[string]any{
							"summary":     "Deploy feature",
							"description": map[string]any{"content": []map[string]any{}},
							"status":      map[string]any{"name": "Done"},
							"created":     "2025-11-21T10:00:00Z",
							"updated":     "2025-11-21T10:00:00Z",
						},
					},
				}
			} else if strings.Contains(jql, "assignee IN (\"alice\")") {
				issues = []map[string]any{
					{
						"id":  "10001",
						"key": "PROJ-1",
						"fields": map[string]any{
							"summary":     "Task 1",
							"description": map[string]any{"content": []map[string]any{}},
							"status":      map[string]any{"name": "To Do"},
							"assignee":    map[string]any{"accountId": "alice"},
							"created":     "2025-11-21T10:00:00Z",
							"updated":     "2025-11-21T10:00:00Z",
						},
					},
				}
			} else if strings.Contains(jql, "reporter = \"bob\"") {
				issues = []map[string]any{
					{
						"id":  "10003",
						"key": "PROJ-3",
						"fields": map[string]any{
							"summary":     "Task 3",
							"description": map[string]any{"content": []map[string]any{}},
							"status":      map[string]any{"name": "To Do"},
							"reporter":    map[string]any{"accountId": "bob"},
							"created":     "2025-11-21T10:00:00Z",
							"updated":     "2025-11-21T10:00:00Z",
						},
					},
				}
			} else {
				// Default: return all
				issues = []map[string]any{
					{
						"id":  "10001",
						"key": "PROJ-1",
						"fields": map[string]any{
							"summary":     "Task 1",
							"description": map[string]any{"content": []map[string]any{}},
							"status":      map[string]any{"name": "To Do"},
							"created":     "2025-11-21T10:00:00Z",
							"updated":     "2025-11-21T10:00:00Z",
						},
					},
					{
						"id":  "10002",
						"key": "PROJ-2",
						"fields": map[string]any{
							"summary":     "Task 2",
							"description": map[string]any{"content": []map[string]any{}},
							"status":      map[string]any{"name": "In Progress"},
							"created":     "2025-11-21T10:00:00Z",
							"updated":     "2025-11-21T10:00:00Z",
						},
					},
				}
			}

			// Apply limit from payload
			if maxResults, ok := payload["maxResults"].(float64); ok && int(maxResults) == 1 && len(issues) > 1 {
				issues = issues[:1]
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"issues": issues,
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &JiraProvider{
		cfg: Config{
			Source:     "jira",
			Email:      "test@example.com",
			APIToken:   "test-token",
			APIURL:     server.URL,
			ProjectKey: "PROJ",
		},
		client: &http.Client{},
	}
	ctx := context.Background()

	t.Run("query all tickets", func(t *testing.T) {
		tickets, err := p.Query(ctx, schema.TicketQuery{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(tickets) != 2 {
			t.Errorf("len(tickets) = %v, want 2", len(tickets))
		}
	})

	t.Run("query with free-text search", func(t *testing.T) {
		tickets, err := p.Query(ctx, schema.TicketQuery{Query: "login"})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(tickets) != 1 {
			t.Errorf("len(tickets) = %v, want 1", len(tickets))
		}
		if len(tickets) > 0 && tickets[0].Title != "Fix login bug" {
			t.Errorf("Title = %v, want Fix login bug", tickets[0].Title)
		}
	})

	t.Run("query with status filter", func(t *testing.T) {
		tickets, err := p.Query(ctx, schema.TicketQuery{Statuses: []string{"Done"}})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(tickets) != 1 {
			t.Errorf("len(tickets) = %v, want 1", len(tickets))
		}
	})

	t.Run("query with assignee filter", func(t *testing.T) {
		tickets, err := p.Query(ctx, schema.TicketQuery{Assignees: []string{"alice"}})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(tickets) != 1 {
			t.Errorf("len(tickets) = %v, want 1", len(tickets))
		}
	})

	t.Run("query with reporter filter", func(t *testing.T) {
		tickets, err := p.Query(ctx, schema.TicketQuery{Reporter: "bob"})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(tickets) != 1 {
			t.Errorf("len(tickets) = %v, want 1", len(tickets))
		}
	})

	t.Run("query with limit", func(t *testing.T) {
		tickets, err := p.Query(ctx, schema.TicketQuery{Limit: 1})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(tickets) != 1 {
			t.Errorf("len(tickets) = %v, want 1", len(tickets))
		}
	})
}

func TestUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/PROJ-1" && r.Method == "PUT" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.URL.Path == "/rest/api/3/issue/PROJ-1/transitions" && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"transitions": []map[string]any{
					{
						"id":   "11",
						"name": "To Do",
						"to":   map[string]any{"name": "To Do"},
					},
					{
						"id":   "21",
						"name": "In Progress",
						"to":   map[string]any{"name": "In Progress"},
					},
					{
						"id":   "31",
						"name": "Done",
						"to":   map[string]any{"name": "Done"},
					},
				},
			})
			return
		}

		if r.URL.Path == "/rest/api/3/issue/PROJ-1/transitions" && r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.URL.Path == "/rest/api/3/issue/PROJ-1" && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"id":  "10001",
				"key": "PROJ-1",
				"fields": map[string]any{
					"summary": "Updated title",
					"description": map[string]any{
						"content": []map[string]any{
							{
								"content": []map[string]any{
									{"text": "Updated description"},
								},
							},
						},
					},
					"status": map[string]any{
						"name": "In Progress",
					},
					"assignee": map[string]any{
						"accountId": "alice",
					},
					"created": "2025-11-21T10:00:00Z",
					"updated": "2025-11-21T11:00:00Z",
				},
			})
			return
		}

		if r.URL.Path == "/rest/api/3/issue/NOTFOUND" && r.Method == "PUT" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &JiraProvider{
		cfg: Config{
			Source:     "jira",
			Email:      "test@example.com",
			APIToken:   "test-token",
			APIURL:     server.URL,
			ProjectKey: "PROJ",
		},
		client: &http.Client{},
	}
	ctx := context.Background()

	t.Run("update title", func(t *testing.T) {
		newTitle := "Updated title"
		ticket, err := p.Update(ctx, "PROJ-1", schema.UpdateTicketInput{
			Title: &newTitle,
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if ticket.Title != "Updated title" {
			t.Errorf("Title = %v, want Updated title", ticket.Title)
		}
	})

	t.Run("update description", func(t *testing.T) {
		newDesc := "Updated description"
		ticket, err := p.Update(ctx, "PROJ-1", schema.UpdateTicketInput{
			Description: &newDesc,
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if ticket.Description != "Updated description" {
			t.Errorf("Description = %v, want Updated description", ticket.Description)
		}
	})

	t.Run("update status with transition", func(t *testing.T) {
		newStatus := "In Progress"
		ticket, err := p.Update(ctx, "PROJ-1", schema.UpdateTicketInput{
			Status: &newStatus,
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if ticket.Status != "In Progress" {
			t.Errorf("Status = %v, want In Progress", ticket.Status)
		}
	})

	t.Run("update assignees", func(t *testing.T) {
		newAssignees := []string{"alice"}
		ticket, err := p.Update(ctx, "PROJ-1", schema.UpdateTicketInput{
			Assignees: &newAssignees,
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if len(ticket.Assignees) != 1 || ticket.Assignees[0] != "alice" {
			t.Errorf("Assignees = %v, want [alice]", ticket.Assignees)
		}
	})

	t.Run("update non-existent ticket", func(t *testing.T) {
		newTitle := "Test"
		_, err := p.Update(ctx, "NOTFOUND", schema.UpdateTicketInput{
			Title: &newTitle,
		})
		if err != errNotFound {
			t.Errorf("Update() error = %v, want errNotFound", err)
		}
	})
}

func TestBuildJQL(t *testing.T) {
	tests := []struct {
		name     string
		query    schema.TicketQuery
		expected string
	}{
		{
			name:     "empty query",
			query:    schema.TicketQuery{},
			expected: "project = PROJ ORDER BY key DESC",
		},
		{
			name:     "with text search",
			query:    schema.TicketQuery{Query: "login bug"},
			expected: "project = PROJ AND text ~ \"login bug\" ORDER BY key DESC",
		},
		{
			name:     "with status filter",
			query:    schema.TicketQuery{Statuses: []string{"To Do", "In Progress"}},
			expected: "project = PROJ AND status IN (\"To Do\",\"In Progress\") ORDER BY key DESC",
		},
		{
			name:     "with assignee filter",
			query:    schema.TicketQuery{Assignees: []string{"alice", "bob"}},
			expected: "project = PROJ AND assignee IN (\"alice\",\"bob\") ORDER BY key DESC",
		},
		{
			name:     "with reporter filter",
			query:    schema.TicketQuery{Reporter: "charlie"},
			expected: "project = PROJ AND reporter = \"charlie\" ORDER BY key DESC",
		},
		{
			name: "with multiple filters",
			query: schema.TicketQuery{
				Query:     "bug",
				Statuses:  []string{"To Do"},
				Assignees: []string{"alice"},
			},
			expected: "project = PROJ AND text ~ \"bug\" AND status IN (\"To Do\") AND assignee IN (\"alice\") ORDER BY key DESC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jql := buildJQL(tt.query, "PROJ")
			if jql != tt.expected {
				t.Errorf("buildJQL() = %v, want %v", jql, tt.expected)
			}
		})
	}
}

func TestEscapeJQL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "simple text",
			expected: "simple text",
		},
		{
			input:    "text with \"quotes\"",
			expected: "text with \\\"quotes\\\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeJQL(tt.input)
			if result != tt.expected {
				t.Errorf("escapeJQL() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConvertJiraIssue(t *testing.T) {
	issue := jiraIssue{
		ID:  "10001",
		Key: "PROJ-1",
	}
	issue.Fields.Summary = "Test issue"
	issue.Fields.Status.Name = "To Do"
	issue.Fields.Description.Content = []struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}{
		{
			Content: []struct {
				Text string `json:"text"`
			}{
				{Text: "First paragraph"},
				{Text: "Second paragraph"},
			},
		},
	}
	issue.Fields.Assignee = &struct {
		AccountID   string `json:"accountId"`
		DisplayName string `json:"displayName"`
	}{
		AccountID:   "user123",
		DisplayName: "Alice",
	}
	issue.Fields.Reporter = &struct {
		AccountID   string `json:"accountId"`
		DisplayName string `json:"displayName"`
	}{
		AccountID:   "user456",
		DisplayName: "Bob",
	}
	issue.Fields.Priority = &struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{
		ID:   "2",
		Name: "Critical",
	}
	issue.Fields.IssueType.ID = "10002"
	issue.Fields.IssueType.Name = "Story"
	issue.Fields.Labels = []string{"feature", "ui"}
	issue.Fields.Components = []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{
		{ID: "10000", Name: "Frontend"},
		{ID: "10001", Name: "Backend"},
	}
	issue.Fields.Created = "2025-11-21T10:00:00Z"
	issue.Fields.Updated = "2025-11-21T11:00:00Z"

	ticket := convertJiraIssue(issue, "jira")

	if ticket.ID != "10001" {
		t.Errorf("ID = %v, want 10001", ticket.ID)
	}
	if ticket.Key != "PROJ-1" {
		t.Errorf("Key = %v, want PROJ-1", ticket.Key)
	}
	if ticket.Title != "Test issue" {
		t.Errorf("Title = %v, want Test issue", ticket.Title)
	}
	if ticket.Status != "To Do" {
		t.Errorf("Status = %v, want To Do", ticket.Status)
	}
	if ticket.Description != "First paragraph Second paragraph" {
		t.Errorf("Description = %v, want First paragraph Second paragraph", ticket.Description)
	}
	if len(ticket.Assignees) != 1 || ticket.Assignees[0] != "user123" {
		t.Errorf("Assignees = %v, want [user123]", ticket.Assignees)
	}
	if ticket.Reporter != "user456" {
		t.Errorf("Reporter = %v, want user456", ticket.Reporter)
	}
	if ticket.Metadata["source"] != "jira" {
		t.Errorf("Metadata[source] = %v, want jira", ticket.Metadata["source"])
	}
	if ticket.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if ticket.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
	if ticket.Metadata["priority"] != "Critical" {
		t.Errorf("Metadata[priority] = %v, want Critical", ticket.Metadata["priority"])
	}
	if ticket.Metadata["issue_type"] != "Story" {
		t.Errorf("Metadata[issue_type] = %v, want Story", ticket.Metadata["issue_type"])
	}
	if labels, ok := ticket.Metadata["labels"].([]string); !ok || len(labels) != 2 {
		t.Errorf("Metadata[labels] = %v, want []string{feature, ui}", ticket.Metadata["labels"])
	}
	if components, ok := ticket.Metadata["components"].([]string); !ok || len(components) != 2 {
		t.Errorf("Metadata[components] = %v, want []string{Frontend, Backend}", ticket.Metadata["components"])
	}
}
