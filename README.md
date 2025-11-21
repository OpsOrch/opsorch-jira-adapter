# OpsOrch Jira Adapter

This module integrates OpsOrch with Atlassian Jira using the Jira REST API v3. It implements the ticket provider interface to create, query, retrieve, and update Jira issues directly from OpsOrch workflows.

## Layout
- `ticket/jira_provider.go`: ticket provider implementation plus config helpers and registry wiring.
- `cmd/ticketplugin/main.go`: JSON-RPC plugin entrypoint so the adapter can run out-of-process.
- `version.go`: adapter version + minimum OpsOrch Core requirement.
- `Makefile`: build/test/plugin shortcuts with a local module cache.

## Development
```bash
make test      # runs go test ./...
make build     # go build ./...
make plugin    # builds ./bin/ticketplugin
```

`go.mod` points at the sibling `opsorch-core` repo for local iteration. Remove the replace directive when publishing.

## Configuration Contract
The stub provider accepts (and defaults) the fields shown below. Swap/extend these when adding real Jira integration settings.

```json
{
  "source": "jira",
  "apiToken": "your_jira_api_token",
  "apiURL": "https://your-domain.atlassian.net",
  "projectKey": "PROJ",
  "defaultIssueType": "Task"
}
```

- `apiToken` is required; this is your Jira API token for authentication.
- `apiURL` defaults to "https://your-domain.atlassian.net" but should be set to your actual Jira instance URL.
- `projectKey` is required; this is the Jira project key where issues will be created (e.g., "PROJ", "OPS").
- `defaultIssueType` defaults to "Task" but can be set to other issue types like "Bug", "Story", etc.

## Using In OpsOrch Core

### In-Process Mode
Import the module for side effects and select it via `OPSORCH_TICKET_PROVIDER=jira`:

```go
import _ "github.com/opsorch/opsorch-jira-adapter/ticket"
```

Set environment variables:
```bash
export OPSORCH_TICKET_PROVIDER=jira
export OPSORCH_TICKET_CONFIG='{"apiToken":"...","projectKey":"PROJ"}'
```

### Plugin Mode
Build the plugin and set `OPSORCH_TICKET_PLUGIN` to the binary path:

```bash
make plugin
export OPSORCH_TICKET_PLUGIN=/path/to/bin/ticketplugin
export OPSORCH_TICKET_CONFIG='{"apiToken":"...","projectKey":"PROJ"}'
```

## Plugin RPC Contract
OpsOrch Core talks to the plugin over stdin/stdout using JSON objects shaped like:

```json
{
  "method": "ticket.create",
  "config": { /* decrypted OPSORCH_TICKET_CONFIG */ },
  "payload": { /* method-specific body */ }
}
```

- `config` is the decrypted map described above; Core injects it on every call so the plugin never stores secrets on disk.
- `payload` matches the schema from `opsorch-core` for the requested method (e.g., `schema.CreateTicketInput` for `ticket.create`).
- Responses mirror `{ "result": any }` or `{ "error": string }` for success/failure.

### Supported Methods

#### ticket.query
Query tickets with filters.

Request:
```json
{
  "method": "ticket.query",
  "config": { "apiToken": "...", "projectKey": "PROJ" },
  "payload": {
    "query": "login bug",
    "statuses": ["To Do", "In Progress"],
    "limit": 10
  }
}
```

Response:
```json
{
  "result": [
    {
      "id": "jira-1",
      "key": "PROJ-1",
      "title": "Fix login bug",
      "status": "To Do",
      "createdAt": "2025-11-20T10:00:00Z",
      "updatedAt": "2025-11-20T10:00:00Z"
    }
  ]
}
```

#### ticket.get
Get a single ticket by ID.

Request:
```json
{
  "method": "ticket.get",
  "config": { "apiToken": "...", "projectKey": "PROJ" },
  "payload": { "id": "jira-1" }
}
```

#### ticket.create
Create a new ticket.

Request:
```json
{
  "method": "ticket.create",
  "config": { "apiToken": "...", "projectKey": "PROJ" },
  "payload": {
    "title": "Fix login bug",
    "description": "Users cannot login to the application",
    "fields": { "priority": "high" },
    "metadata": { "team": "backend" }
  }
}
```

#### ticket.update
Update an existing ticket.

Request:
```json
{
  "method": "ticket.update",
  "config": { "apiToken": "...", "projectKey": "PROJ" },
  "payload": {
    "id": "jira-1",
    "input": {
      "status": "In Progress",
      "assignees": ["alice", "bob"]
    }
  }
}
```

## Security Considerations

Because the protocol stays on-box (pipes between Core and the plugin), Jira credentials remain local. Follow these security best practices:

1. **Never log the API token** - Avoid logging the config or token in the plugin or application logs.
2. **Rotate tokens regularly** - Rotate the `apiToken` at the cadence required by your organization's security policy.
3. **Use environment variables** - Store the `OPSORCH_TICKET_CONFIG` in a secure environment variable or secrets management system.
4. **Restrict file permissions** - If storing config in files, ensure proper file permissions (e.g., 0600).
5. **Use Jira API tokens** - Use Jira API tokens instead of passwords for authentication.

## Jira API Integration

This adapter integrates with the Jira REST API v3:

- **Create** → POST `/rest/api/3/issue` - Creates new Jira issues
- **Get** → GET `/rest/api/3/issue/{issueIdOrKey}` - Retrieves issue details
- **Query** → GET `/rest/api/3/search` - Searches issues using JQL (Jira Query Language)
- **Update** → PUT `/rest/api/3/issue/{issueIdOrKey}` - Updates issue fields
- **Transitions** → POST `/rest/api/3/issue/{issueIdOrKey}/transitions` - Changes issue status

### Authentication

The adapter uses Bearer token authentication with Jira API tokens. Generate an API token from your Atlassian account settings:
1. Go to https://id.atlassian.com/manage-profile/security/api-tokens
2. Create a new API token
3. Use the token in the `apiToken` configuration field

### JQL Query Building

The Query method automatically builds JQL queries from the TicketQuery filters:
- `query` → `text ~ "search term"`
- `statuses` → `status IN ("To Do", "In Progress")`
- `assignees` → `assignee IN ("user1", "user2")`
- `reporter` → `reporter = "user"`

All queries are scoped to the configured project automatically.
