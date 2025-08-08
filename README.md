# GoJira

A lightweight, locally-hosted API server that acts as an intelligent bridge between Claude Code and Atlassian Jira. GoJira provides a simplified, unified interface for Claude Code to perform Jira operations through natural language instructions.

## Overview

GoJira acts as an intelligent middleware between Claude Code and Jira's REST API, translating natural language commands into Jira operations while handling authentication, session management, and response formatting. This enables developers to manage Jira tickets seamlessly within their AI-assisted coding environment.

## Key Features

### 🎯 Core Functionality
- **Complete Issue Management** - Create, read, update, delete issues with full field support
- **Advanced Search** - JQL queries, natural language search, and result filtering
- **Sprint Management** - Full sprint lifecycle with metrics and reporting
- **Workflow Automation** - Transition management with validation and bulk operations
- **Board Operations** - Scrum and Kanban board integration
- **Comment & Attachment Support** - Rich text comments and file handling

### 🤖 Claude Code Integration
- **Natural Language Processing** - Convert plain English to Jira operations
- **Intelligent Context Handling** - Session persistence and smart suggestions
- **Claude-Optimized APIs** - Specialized endpoints for AI workflows
- **Task Tracking Integration** - Automatic issue creation and updates

### 🚀 Advanced Features
- **Queue Management** - Priority queuing with rate limiting
- **Caching Layer** - Multi-level caching for performance
- **Workflow Analytics** - Transition metrics and state analysis
- **Batch Operations** - Bulk processing capabilities
- **Security Hardening** - Input validation, sanitization, and audit logging

## Complete API Reference

### Health & Monitoring
- `GET /health` - Basic health check
- `GET /ready` - Readiness probe for containers
- `GET /metrics` - Application metrics
- `POST /metrics/reset` - Reset metrics counters
- `GET /health/detailed` - Health check with metrics

### Authentication
- `POST /api/v1/auth/connect` - Connect to Jira instance
- `POST /api/v1/auth/disconnect` - Disconnect from Jira
- `GET /api/v1/auth/status` - Get connection status
- `POST /api/v1/auth/oauth2/start` - Start OAuth2 flow
- `GET /api/v1/auth/oauth2/callback` - OAuth2 callback handler

### Issue Management
- `POST /api/v1/issues` - Create new issue
- `GET /api/v1/issues/{key}` - Get issue details
- `PUT /api/v1/issues/{key}` - Update existing issue
- `DELETE /api/v1/issues/{key}` - Delete issue
- `GET /api/v1/issues/{key}/transitions` - Get available transitions
- `POST /api/v1/issues/{key}/transitions` - Execute transition
- `GET /api/v1/issues/{key}/links` - Get issue links
- `POST /api/v1/issues/link` - Create issue link
- `DELETE /api/v1/issues/link/{id}` - Delete issue link
- `GET /api/v1/issues/linktypes` - Get available link types
- `GET /api/v1/issues/{key}/customfields` - Get custom field values

### Search & Filtering
- `GET /api/v1/search` - Search issues with query parameters
- `POST /api/v1/search` - Search issues with JSON body
- `POST /api/v1/search/advanced` - Advanced search with filters
- `POST /api/v1/search/paginated` - Paginated search results
- `POST /api/v1/search/export` - Export search results
- `POST /api/v1/search/page` - Get specific search page
- `POST /api/v1/search/all-pages` - Get all search pages
- `GET /api/v1/search/validate` - Validate JQL query
- `GET /api/v1/search/suggestions` - Get JQL suggestions
- `GET /api/v1/search/fields` - Get available JQL fields
- `GET /api/v1/search/functions` - Get JQL functions

### Filters
- `GET /api/v1/filters` - Get all saved filters
- `GET /api/v1/filters/{id}` - Get specific filter
- `GET /api/v1/filters/{id}/search` - Execute filter search

### Sprint Management
- `GET /api/v1/sprints` - List all sprints
- `POST /api/v1/sprints` - Create new sprint
- `GET /api/v1/sprints/active` - Get active sprints
- `GET /api/v1/sprints/upcoming` - Get upcoming sprints
- `GET /api/v1/sprints/health` - Sprint health check
- `POST /api/v1/sprints/validate` - Validate sprint request
- `GET /api/v1/sprints/{id}` - Get sprint details
- `PUT /api/v1/sprints/{id}` - Update sprint
- `POST /api/v1/sprints/{id}/start` - Start sprint
- `POST /api/v1/sprints/{id}/auto-start` - Auto-start sprint
- `POST /api/v1/sprints/{id}/close` - Close sprint
- `POST /api/v1/sprints/{id}/complete` - Complete sprint with report
- `GET /api/v1/sprints/{id}/issues` - Get sprint issues
- `POST /api/v1/sprints/{id}/issues` - Move issues to sprint
- `GET /api/v1/sprints/{id}/report` - Get sprint report
- `GET /api/v1/sprints/{id}/metrics` - Get sprint metrics
- `GET /api/v1/sprints/{id}/predict` - Predict sprint success
- `POST /api/v1/sprints/{id}/clone` - Clone sprint

### Board Operations
- `GET /api/v1/boards` - List all boards
- `GET /api/v1/boards/{id}` - Get board details
- `GET /api/v1/boards/{id}/configuration` - Get board configuration
- `GET /api/v1/boards/{id}/issues` - Get board issues
- `GET /api/v1/boards/{id}/backlog` - Get board backlog
- `GET /api/v1/boards/{id}/sprints` - Get board sprints

### Workflow Management
- `GET /api/v1/workflows` - List all workflows
- `GET /api/v1/workflows/{name}` - Get workflow details
- `GET /api/v1/workflows/{name}/cached` - Get cached workflow
- `GET /api/v1/workflows/{name}/statemachine` - Get workflow state machine
- `GET /api/v1/workflows/{name}/statemachine/advanced` - Advanced state machine
- `GET /api/v1/workflows/{name}/analytics` - Workflow analytics
- `GET /api/v1/workflows/{name}/analytics/advanced` - Advanced analytics
- `GET /api/v1/workflows/metrics` - Workflow transition metrics
- `GET /api/v1/workflows/schemes` - Get workflow schemes
- `GET /api/v1/workflows/schemes/project/{projectKey}` - Get project workflow scheme
- `POST /api/v1/workflows/validate/batch` - Batch validate transitions

### Issue Workflow Operations
- `GET /api/v1/issues/{issueKey}/workflow` - Get issue workflow
- `GET /api/v1/issues/{issueKey}/workflow/transitions` - Get available transitions
- `GET /api/v1/issues/{issueKey}/workflow/transitions/{transitionId}/validate` - Validate transition
- `GET /api/v1/issues/{issueKey}/workflow/transitions/{transitionId}/validate/advanced` - Advanced validation
- `POST /api/v1/issues/{issueKey}/workflow/transitions/{transitionId}/execute` - Execute transition
- `POST /api/v1/issues/{issueKey}/workflow/transitions/{transitionId}/execute/advanced` - Advanced execution
- `POST /api/v1/issues/{issueKey}/workflow/transitions/{transitionId}/simulate` - Simulate transition

### Claude-Optimized Endpoints
- `GET /api/v1/claude/issues/{key}` - Claude-formatted issue details
- `POST /api/v1/claude/issues` - Claude-optimized issue creation
- `POST /api/v1/claude/search` - Claude-formatted search results
- `POST /api/v1/claude/command` - Process natural language command
- `POST /api/v1/claude/jql` - Generate JQL from natural language
- `GET /api/v1/claude/suggestions` - Get command suggestions

### Natural Language Processing
- `POST /api/v1/nlp/parse` - Parse natural language command
- `POST /api/v1/nlp/entities` - Extract entities from text
- `GET /api/v1/nlp/suggestions` - Get command suggestions
- `POST /api/v1/nlp/suggestions` - Generate contextual suggestions
- `POST /api/v1/nlp/validate` - Validate natural language command
- `GET /api/v1/nlp/status` - Get parser status
- `POST /api/v1/nlp/context` - Update parsing context

### Queue Management
- `POST /api/v1/queue/jobs` - Submit job to queue
- `POST /api/v1/queue/jobs/batch` - Submit batch jobs
- `GET /api/v1/queue/jobs/result` - Get job result
- `DELETE /api/v1/queue/jobs/{jobId}` - Remove job from queue
- `GET /api/v1/queue/status` - Get queue status
- `GET /api/v1/queue/metrics` - Get queue metrics
- `DELETE /api/v1/queue/clear` - Clear entire queue
- `GET /api/v1/queue/priority` - Get priority queue status
- `GET /api/v1/queue/ratelimiter/stats` - Get rate limiter stats
- `POST /api/v1/queue/ratelimiter/reset` - Reset rate limiter

## Claude Code Integration Guide

### Setting Up Claude Code with GoJira

To enable Claude Code to use GoJira for Jira task management, add the following to your project's `CLAUDE.md` file:

```markdown
## Jira Integration with GoJira

### Project Configuration
- **Default Project Key**: `YOUR_PROJECT_KEY` (replace with your actual project key)
- **Issue Types**: Task, Bug, Story, Epic, Subtask
- **GoJira URL**: http://localhost:8080 (or your server IP for cross-WSL access)

### Task Management Commands
When working on complex tasks, use GoJira to create and track Jira issues:

# Create new issue
curl -X POST http://localhost:8080/api/v1/issues \
  -H "Content-Type: application/json" \
  -d '{
    "project": "YOUR_PROJECT_KEY",
    "summary": "Task description",
    "description": "Detailed task information",
    "issueType": "Task"
  }'

# Update issue with progress
curl -X POST http://localhost:8080/api/v1/issues/YOUR_PROJECT_KEY-123/comments \
  -H "Content-Type: application/json" \
  -d '{"body": "Progress update or completion status"}'

# Search for issues
curl -X GET "http://localhost:8080/api/v1/search?jql=project=YOUR_PROJECT_KEY+AND+status=Open"

### Natural Language Commands
Use Claude-optimized endpoints for natural language processing:

# Process natural language commands
curl -X POST http://localhost:8080/api/v1/claude/command \
  -H "Content-Type: application/json" \
  -d '{"command": "Create a bug ticket for login issue with high priority"}'

# Generate JQL from natural language
curl -X POST http://localhost:8080/api/v1/claude/jql \
  -H "Content-Type: application/json" \
  -d '{"query": "Show me all critical bugs in the current sprint"}'
```

### Cross-WSL Instance Access

If using multiple WSL instances, configure GoJira to bind to all interfaces:

1. **Update configuration** in `configs/default.yaml`:
   ```yaml
   server:
     host: 0.0.0.0  # Bind to all interfaces instead of localhost
     port: 8080
   ```

2. **Find your WSL IP address**:
   ```bash
   ip addr show eth0 | grep 'inet ' | awk '{print $2}' | cut -d/ -f1
   ```

3. **Update Claude instructions** to use your WSL IP:
   ```bash
   # Replace localhost with your WSL IP (e.g., 172.26.23.28)
   http://172.26.23.28:8080/api/v1/...
   ```

## Installation & Setup

### Quick Start with Docker

```bash
# Build and run with Docker Compose
docker-compose up -d

# Or run directly with Docker
docker run -d \
  --name gojira \
  -p 8080:8080 \
  -v $(pwd)/configs:/app/configs \
  -e JIRA_URL=https://your-domain.atlassian.net \
  gojira:latest
```

### Building from Source

```bash
# Clone repository
git clone https://github.com/ericfisherdev/GoJira.git
cd GoJira

# Build for current platform
make build

# Build for all platforms
make build-all

# Run the server
./dist/gojira serve
```

### Available Make Targets

```bash
make build          # Build for current platform
make build-all      # Build for all platforms (Windows, Linux, macOS)
make build-windows  # Build Windows executables (x64/ARM64)
make build-linux    # Build Linux binaries (x64/ARM64)
make build-darwin   # Build macOS binaries (Intel/Apple Silicon)
make docker-build   # Build Docker image
make docker-run     # Build and run with Docker
make test           # Run all tests
make test-coverage  # Run tests with coverage report
make test-benchmark # Run benchmark tests
make lint           # Run code linters
make fmt            # Format code
make clean          # Clean build artifacts
make deps           # Tidy Go module dependencies
```

## Configuration

### Basic Configuration

Create `configs/gojira.yaml`:

```yaml
server:
  host: localhost  # Use 0.0.0.0 for cross-WSL access
  port: 8080
  mode: development

jira:
  url: https://your-domain.atlassian.net
  timeout: 30
  retries: 3
  auth:
    type: api_token
    email: your-email@example.com
    token: ${JIRA_API_TOKEN}  # Use environment variable

features:
  natural_language: true
  caching: true
  auto_retry: true

logging:
  level: info
  format: json
  output: stdout

security:
  rate_limit: 100
  enable_cors: true
  allowed_origins:
    - "http://localhost:*"
    - "https://localhost:*"
```

### Environment Variables

All configuration can be overridden with environment variables:

```bash
export GOJIRA_SERVER_HOST=0.0.0.0
export GOJIRA_SERVER_PORT=8080
export JIRA_URL=https://your-domain.atlassian.net
export JIRA_EMAIL=your-email@example.com
export JIRA_API_TOKEN=your-api-token
export GOJIRA_LOG_LEVEL=info
```

## Authentication

### API Token (Recommended for Jira Cloud)

1. Generate an API token at https://id.atlassian.com/manage/api-tokens
2. Configure in `gojira.yaml` or set `JIRA_API_TOKEN` environment variable
3. **Never commit tokens to source control**

### OAuth 2.0

1. Register OAuth app in Jira administration
2. Configure client ID and secret in environment variables
3. GoJira handles the OAuth flow automatically

### Personal Access Token (Jira Server/Data Center)

1. Generate PAT in Jira user settings
2. Configure as bearer token in authentication settings

## System Requirements

### Minimum Requirements
- **OS**: Windows 10+, macOS 11+, Ubuntu 20.04+, or WSL2
- **RAM**: 512MB available memory
- **Disk**: 50MB storage space
- **Network**: Internet connection for Jira API access

### Supported Platforms
- Windows (x64, ARM64)
- Linux (x64, ARM64) 
- macOS (Intel, Apple Silicon)
- Docker containers
- WSL2 environments

### Jira Compatibility
- Jira Cloud (latest)
- Jira Server 8.x+
- Jira Data Center 8.x+

## Performance Characteristics

- **Response Time**: <500ms for single operations
- **Throughput**: 100+ concurrent requests supported
- **Memory Usage**: <100MB baseline memory footprint
- **Startup Time**: <2 seconds cold start
- **Cache Hit Rate**: >90% for repeated queries

## Development

### Project Structure
```
GoJira/
├── cmd/gojira/           # Main application entry point
├── internal/             # Internal packages (not importable)
│   ├── api/              # HTTP API handlers and routes
│   │   ├── handlers/     # Request handlers
│   │   └── routes/       # Route definitions
│   ├── auth/             # Authentication implementations
│   ├── config/           # Configuration management
│   ├── jira/             # Jira API client
│   ├── cache/            # Caching implementations
│   ├── queue/            # Job queue management
│   ├── nlp/              # Natural language processing
│   └── monitoring/       # Metrics and monitoring
├── pkg/                  # Public packages (importable)
├── tests/                # Test suites
│   ├── integration/      # Integration tests
│   ├── benchmarks/       # Performance benchmarks
│   └── e2e/              # End-to-end tests
├── docker/               # Docker configurations
├── configs/              # Configuration files
└── docs/                 # Documentation
```

### Running Tests

```bash
# Run all tests
make test

# Run with coverage report
make test-coverage

# Run integration tests only
make test-integration

# Run benchmark tests
make test-benchmark

# Run specific test package
go test ./internal/jira/...
```

### Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature-name`
3. Make your changes with tests
4. Run the test suite: `make test`
5. Run linters: `make lint`
6. Commit your changes: `git commit -m "Description"`
7. Push to the branch: `git push origin feature-name`
8. Open a Pull Request

## Security

### Security Features
- **Input Validation**: All inputs validated and sanitized
- **Rate Limiting**: Protection against abuse and DoS
- **TLS Enforcement**: HTTPS required for all external communications
- **Credential Protection**: Sensitive data never logged or exposed
- **CORS Support**: Configurable cross-origin request handling
- **Audit Logging**: All operations logged for security monitoring

### Security Best Practices
- Store API tokens in environment variables, never in code
- Use strong authentication methods (OAuth 2.0 preferred)
- Enable rate limiting in production environments
- Monitor logs for suspicious activity
- Keep GoJira updated to latest version

## Troubleshooting

### Common Issues

**Connection Failed**
```bash
# Check GoJira status
curl http://localhost:8080/health

# Test Jira connectivity
curl http://localhost:8080/api/v1/auth/status
```

**Authentication Issues**
- Verify API token is valid and not expired
- Check Jira URL format (https://domain.atlassian.net)
- Ensure user has appropriate Jira permissions

**Cross-WSL Connectivity**
- Ensure server host is set to `0.0.0.0` instead of `localhost`
- Use WSL IP address instead of localhost from other instances
- Check Windows firewall settings

**Performance Issues**
- Enable caching in configuration
- Increase rate limits if needed
- Monitor memory usage with `/metrics` endpoint

### Getting Help

- **Issues**: [GitHub Issues](https://github.com/ericfisherdev/GoJira/issues)
- **Documentation**: [docs/](docs/)
- **Examples**: [examples/](examples/)
- **Integration Guide**: [docs/local/claude.example.md](docs/local/claude.example.md)

## License

MIT License - see [LICENSE](LICENSE) file for details.

---

**Built to enhance the Claude Code development experience by providing seamless Jira integration.**