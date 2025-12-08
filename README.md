# OpsOrch Jira Adapter

[![Version](https://img.shields.io/github/v/release/opsorch/opsorch-jira-adapter)](https://github.com/opsorch/opsorch-jira-adapter/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/opsorch/opsorch-jira-adapter)](https://github.com/opsorch/opsorch-jira-adapter/blob/main/go.mod)
[![License](https://img.shields.io/github/license/opsorch/opsorch-jira-adapter)](https://github.com/opsorch/opsorch-jira-adapter/blob/main/LICENSE)
[![CI](https://github.com/opsorch/opsorch-jira-adapter/workflows/CI/badge.svg)](https://github.com/opsorch/opsorch-jira-adapter/actions)

This adapter integrates OpsOrch with Atlassian Jira Cloud, enabling ticket creation, querying, retrieval, and updates directly from OpsOrch workflows using the Jira REST API v3.

## Capabilities

This adapter provides the following capability:

1. **Ticket Provider**: Create, query, retrieve, and update Jira issues

## Features

- **Create Tickets**: Create new Jira issues with title, description, priority, and labels
- **Query Tickets**: Search issues using JQL with filters for status, assignees, and text search
- **Get Ticket**: Retrieve individual ticket details by ID or key
- **Update Tickets**: Modify ticket fields including title, description, status, and assignees
- **Status Transitions**: Change ticket status through Jira workflows
- **JQL Query Building**: Automatically build JQL queries from OpsOrch ticket filters

### Version Compatibility

- **Adapter Version**: 0.1.0
- **Requires OpsOrch Core**: >=0.1.0
- **Jira Cloud**: REST API v3 (2018+)
- **Go Version**: 1.21+

### Jira Cloud vs Server/Data Center

✅ **Jira Cloud**: Fully supported (uses REST API v3 with email + API token authentication)

⚠️ **Jira Server/Data Center**: Not currently supported. Self-hosted Jira uses different authentication mechanisms (username + password or Personal Access Tokens). If you need Jira Server/Data Center support, please open an issue.

## Configuration

The ticket adapter requires the following configuration:

| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `apiToken` | string | Yes | Your Jira API token for authentication | - |
| `email` | string | Yes | Email address associated with the API token | - |
| `apiURL` | string | Yes | Your Jira Cloud instance URL (e.g., `https://your-domain.atlassian.net`) | - |
| `projectKey` | string | Yes | The Jira project key where issues will be created (e.g., "PROJ", "OPS") | - |
| `defaultIssueType` | string | No | Default issue type for new tickets | `"Task"` |
| `source` | string | No | Source identifier for metadata | `"jira"` |

### Authentication Setup

#### 1. Verify You're on Jira Cloud

Your `apiURL` should look like:
```
https://your-domain.atlassian.net
```

**Note:** If you're self-hosted (Jira Server/Data Center), the authentication works differently. This adapter is designed for Jira Cloud.

#### 2. Create an Atlassian API Token

1. Log in to your Atlassian account in a browser (same account you use for Jira)
2. Go to the API tokens page: https://id.atlassian.com/manage-profile/security/api-tokens
3. Click **"Create API token"**
4. Give it a name like `opsorch-jira-adapter`
5. Click **Create**, then **Copy** the token

⚠️ **Important:** You'll only see the token once. Store it securely in your secret manager (Vault, SSM, Kubernetes secret, etc.) and inject it into OpsOrch config from there.

#### 3. Test Your Token (Optional)

Before using the adapter, you can verify your token works:

```bash
curl -u 'your-email@example.com:YOUR_API_TOKEN' \
  -X GET \
  -H 'Accept: application/json' \
  'https://your-domain.atlassian.net/rest/api/3/myself'
```

If the token is correct, you'll get your user info back. If not, you'll get a 401 error.

### Example Configuration

**JSON format:**
```json
{
  "source": "jira",
  "apiToken": "your-api-token-here",
  "email": "your-email@example.com",
  "apiURL": "https://your-domain.atlassian.net",
  "projectKey": "OPS",
  "defaultIssueType": "Task"
}
```

**YAML format:**
```yaml
ticketProviders:
  jira:
    source: "jira"
    apiToken: "<PASTE_API_TOKEN_HERE>"
    email: "your-atlassian-login-email@example.com"
    apiURL: "https://your-domain.atlassian.net"
    projectKey: "OPS"
    defaultIssueType: "Task"
```

**Environment variables:**
```bash
export OPSORCH_TICKET_PLUGIN=/path/to/bin/ticketplugin
export OPSORCH_TICKET_CONFIG='{"apiToken":"...","email":"...","apiURL":"...","projectKey":"PROJ"}'
```

## Field Mapping

### Ticket Operations

#### Query Mapping

How OpsOrch query fields map to Jira JQL parameters:

| OpsOrch Field | Jira JQL | Transformation | Notes |
|---------------|----------|----------------|-------|
| `query` | `text ~ "search term"` | Wrapped in JQL text search | Full-text search across issue fields |
| `statuses` | `status IN ("To Do", "In Progress")` | Array to JQL IN clause | Status names must match Jira workflow |
| `assignees` | `assignee IN ("user1", "user2")` | Array to JQL IN clause | Uses Jira user identifiers |
| `reporter` | `reporter = "user"` | Direct mapping | Single reporter filter |
| `projectKey` (config) | `project = "PROJ"` | Automatically added to all queries | Scopes queries to configured project |

#### Response Normalization

How Jira issue fields map to OpsOrch ticket schema:

| Jira Field | OpsOrch Field | Transformation | Notes |
|------------|---------------|----------------|-------|
| `id` | `ID` | Direct mapping | Jira issue ID |
| `key` | `Key` | Direct mapping | Human-readable issue key (e.g., "PROJ-123") |
| `fields.summary` | `Title` | Direct mapping | Issue title/summary |
| `fields.description` | `Description` | Direct mapping | Issue description |
| `fields.status.name` | `Status` | Direct mapping | Current workflow status |
| `fields.priority.name` | `Priority` | Stored in `Fields["priority"]` | Priority level (High, Medium, Low) |
| `fields.issuetype.name` | `IssueType` | Stored in `Fields["issueType"]` | Issue type (Task, Bug, Story, etc.) |
| `fields.labels` | `Labels` | Array mapping | Issue labels |
| `fields.assignee` | `Assignees` | User object to string array | Assigned users |
| `fields.created` | `CreatedAt` | ISO 8601 timestamp | Creation timestamp |
| `fields.updated` | `UpdatedAt` | ISO 8601 timestamp | Last update timestamp |

#### Metadata Fields

Provider-specific fields stored in the `metadata` map:

| Metadata Key | Source Field | Type | Description |
|--------------|--------------|------|-------------|
| `source` | N/A | string | Always set to "jira" |
| `self` | `self` | string | Jira API URL for the issue |
| `components` | `fields.components` | array | Issue components |
| `reporter` | `fields.reporter.displayName` | string | Issue reporter name |

#### Known Limitations

1. **JQL Complexity**: Complex JQL queries must be constructed manually; the adapter supports basic filters only
2. **Custom Fields**: Custom fields are not automatically mapped; they must be accessed via the Fields map
3. **Attachments**: File attachments are not currently supported
4. **Comments**: Issue comments are not included in ticket responses
5. **Workflow Transitions**: Status updates must use valid transition names from the project's workflow

## Usage

### In-Process Mode

Import the adapter for side effects to register it with OpsOrch Core:

```go
import _ "github.com/opsorch/opsorch-jira-adapter/ticket"
```

Configure via environment variables:

```bash
export OPSORCH_TICKET_PROVIDER=jira
export OPSORCH_TICKET_CONFIG='{"apiToken":"...","email":"...","apiURL":"https://your-domain.atlassian.net","projectKey":"PROJ"}'
```

### Plugin Mode

Build the plugin binary:

```bash
make plugin
```

Configure OpsOrch Core to use the plugin:

```bash
export OPSORCH_TICKET_PLUGIN=/path/to/bin/ticketplugin
export OPSORCH_TICKET_CONFIG='{"apiToken":"...","email":"...","apiURL":"https://your-domain.atlassian.net","projectKey":"PROJ"}'
```

### Docker Deployment

Download pre-built plugin binaries from [GitHub Releases](https://github.com/opsorch/opsorch-jira-adapter/releases):

```dockerfile
FROM ghcr.io/opsorch/opsorch-core:latest
WORKDIR /opt/opsorch

# Download plugin binary
ADD https://github.com/opsorch/opsorch-jira-adapter/releases/download/v0.1.0/ticketplugin-linux-amd64 ./plugins/ticketplugin
RUN chmod +x ./plugins/ticketplugin

# Configure plugin
ENV OPSORCH_TICKET_PLUGIN=/opt/opsorch/plugins/ticketplugin
```

## Development

### Prerequisites

- Go 1.21 or later
- A Jira Cloud instance with API access
- Jira project with permission to create/update issues
- Atlassian account email

### Building

```bash
# Download dependencies
go mod download

# Run unit tests
make test

# Build all packages
make build

# Build plugin binary
make plugin

# Run integration tests (requires credentials)
make integ
```

### Testing

**Unit Tests:**
```bash
make test
```

**Integration Tests:**

Integration tests run against a live Jira Cloud instance.

**Prerequisites:**
- A Jira Cloud instance (e.g., `https://your-domain.atlassian.net`)
- An API token (see [Authentication Setup](#authentication-setup))
- A project with permission to create/update issues
- Your Atlassian account email

**Setup:**
```bash
# Set required environment variables
export JIRA_API_TOKEN="your-api-token"
export JIRA_API_URL="https://your-domain.atlassian.net"
export JIRA_PROJECT_KEY="PROJ"
export JIRA_USER_EMAIL="your-email@example.com"

# Run all integration tests
make integ

# Or run only ticket integration tests
make integ-ticket
```

**What the tests do:**
1. **Query Tickets** - Fetches tickets and validates metadata (priority, labels, components, issue type)
2. **Create Ticket** - Creates a test ticket with priority and labels
3. **Get Ticket** - Retrieves the created ticket by key
4. **Update Ticket** - Modifies title and description
5. **Search** - Queries tickets using text search
6. **Cleanup** - Attempts to transition ticket to Done/Closed

**Expected behavior:**
- All tests should pass if credentials are valid
- Test tickets are created in the specified project
- Tests clean up created tickets after completion
- Some cleanup may fail if workflow doesn't allow transitions

**Example test output:**
```
=================================
Jira Adapter Integration Test
=================================
API URL: https://your-domain.atlassian.net
Project Key: PROJ

=== Test 1: Query All Tickets ===
Found 25 tickets
  [1] Key: PROJ-123, Title: Fix bug, Status: To Do
       Priority: High
       Issue Type: Bug
       Labels: [backend urgent]
✅ Query all tickets passed

=== Test 2: Create New Ticket ===
✅ Create ticket passed
...

Test Summary
=================================
Total Tests: 5
Passed: 5 ✅
Failed: 0 ❌
Success Rate: 100.0%
```

### Project Structure

```
opsorch-jira-adapter/
├── ticket/                    # Ticket provider implementation
│   ├── jira_provider.go      # Core provider logic
│   └── jira_provider_test.go # Unit tests
├── cmd/
│   └── ticketplugin/         # Plugin entrypoint
│       └── main.go
├── integ/                     # Integration tests
│   └── ticket.go
├── version.go                 # Adapter version metadata
├── Makefile                   # Build targets
├── go.mod
└── README.md
```

**Key Components:**

- **ticket/jira_provider.go**: Implements ticket.Provider interface, handles JQL query building and Jira API interactions
- **cmd/ticketplugin**: JSON-RPC plugin wrapper for ticket provider
- **integ/ticket.go**: End-to-end integration tests against live Jira instance

## CI/CD & Pre-Built Binaries

The repository includes GitHub Actions workflows:

- **CI** (`ci.yml`): Runs tests and linting on every push/PR to main
- **Release** (`release.yml`): Manual workflow that:
  - Runs tests and linting
  - Creates version tags (patch/minor/major)
  - Builds multi-arch binaries (linux-amd64, linux-arm64, darwin-amd64, darwin-arm64)
  - Publishes binaries as GitHub release assets

### Downloading Pre-Built Binaries

Pre-built plugin binaries are available from [GitHub Releases](https://github.com/opsorch/opsorch-jira-adapter/releases).

**Supported platforms:**
- Linux (amd64, arm64)
- macOS (amd64, arm64)

**Example usage in Dockerfile:**
```dockerfile
FROM ghcr.io/opsorch/opsorch-core:latest
WORKDIR /opt/opsorch

ADD https://github.com/opsorch/opsorch-jira-adapter/releases/download/v0.1.0/ticketplugin-linux-amd64 ./plugins/ticketplugin
RUN chmod +x ./plugins/ticketplugin

ENV OPSORCH_TICKET_PLUGIN=/opt/opsorch/plugins/ticketplugin
```

## Plugin RPC Contract

OpsOrch Core communicates with the plugin over stdin/stdout using JSON-RPC.

### Message Format

**Request:**
```json
{
  "method": "ticket.create",
  "config": { /* decrypted OPSORCH_TICKET_CONFIG */ },
  "payload": { /* method-specific body */ }
}
```

**Response:**
```json
{
  "result": { /* method-specific result */ },
  "error": "optional error message"
}
```

### Configuration Injection

The `config` field contains the decrypted configuration map from `OPSORCH_TICKET_CONFIG`. The plugin receives this on every request, so it never stores secrets on disk.

### Supported Methods

#### ticket.query

Query tickets with filters.

**Request:**
```json
{
  "method": "ticket.query",
  "config": { "apiToken": "...", "email": "...", "apiURL": "...", "projectKey": "PROJ" },
  "payload": {
    "query": "login bug",
    "statuses": ["To Do", "In Progress"],
    "limit": 10
  }
}
```

**Response:**
```json
{
  "result": [
    {
      "id": "10001",
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

Get a single ticket by ID or key.

**Request:**
```json
{
  "method": "ticket.get",
  "config": { "apiToken": "...", "email": "...", "apiURL": "...", "projectKey": "PROJ" },
  "payload": { "id": "PROJ-1" }
}
```

**Response:**
```json
{
  "result": {
    "id": "10001",
    "key": "PROJ-1",
    "title": "Fix login bug",
    "description": "Users cannot login to the application",
    "status": "To Do",
    "priority": "High",
    "createdAt": "2025-11-20T10:00:00Z",
    "updatedAt": "2025-11-20T10:00:00Z"
  }
}
```

#### ticket.create

Create a new ticket.

**Request:**
```json
{
  "method": "ticket.create",
  "config": { "apiToken": "...", "email": "...", "apiURL": "...", "projectKey": "PROJ" },
  "payload": {
    "title": "Fix login bug",
    "description": "Users cannot login to the application",
    "fields": { "priority": "High" },
    "metadata": { "team": "backend" }
  }
}
```

**Response:**
```json
{
  "result": {
    "id": "10001",
    "key": "PROJ-1",
    "title": "Fix login bug",
    "status": "To Do",
    "createdAt": "2025-11-20T10:00:00Z"
  }
}
```

#### ticket.update

Update an existing ticket.

**Request:**
```json
{
  "method": "ticket.update",
  "config": { "apiToken": "...", "email": "...", "apiURL": "...", "projectKey": "PROJ" },
  "payload": {
    "id": "PROJ-1",
    "input": {
      "title": "Fix critical login bug",
      "status": "In Progress",
      "assignees": ["alice@example.com"]
    }
  }
}
```

**Response:**
```json
{
  "result": {
    "id": "10001",
    "key": "PROJ-1",
    "title": "Fix critical login bug",
    "status": "In Progress",
    "updatedAt": "2025-11-20T11:00:00Z"
  }
}
```

## Security Considerations

1. **Never log the API token**: Avoid logging the config or token in the plugin or application logs
2. **Rotate tokens regularly**: Rotate the `apiToken` at the cadence required by your organization's security policy
3. **Use environment variables**: Store the `OPSORCH_TICKET_CONFIG` in a secure environment variable or secrets management system
4. **Restrict file permissions**: If storing config in files, ensure proper file permissions (e.g., 0600)
5. **Use Jira API tokens**: Use Jira API tokens instead of passwords for authentication
6. **Validate TLS certificates**: The adapter validates TLS certificates by default; do not disable in production

## Jira API Integration

This adapter integrates with the Jira REST API v3:

- **Create** → `POST /rest/api/3/issue` - Creates new Jira issues
- **Get** → `GET /rest/api/3/issue/{issueIdOrKey}` - Retrieves issue details
- **Query** → `GET /rest/api/3/search` - Searches issues using JQL (Jira Query Language)
- **Update** → `PUT /rest/api/3/issue/{issueIdOrKey}` - Updates issue fields
- **Transitions** → `POST /rest/api/3/issue/{issueIdOrKey}/transitions` - Changes issue status

### Authentication

The adapter uses Basic Authentication with Jira API tokens. The email and API token are combined and sent as a Bearer token.

### JQL Query Building

The Query method automatically builds JQL queries from the TicketQuery filters:
- `query` → `text ~ "search term"`
- `statuses` → `status IN ("To Do", "In Progress")`
- `assignees` → `assignee IN ("user1", "user2")`
- `reporter` → `reporter = "user"`

All queries are automatically scoped to the configured project: `project = "PROJ" AND ...`

## License

Apache 2.0

See LICENSE file for details.
