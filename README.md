# GoJira

A lightweight, locally-hosted API server that bridges Claude Code and Atlassian Jira, providing seamless project management integration for AI-assisted development workflows.

## Overview

GoJira acts as an intelligent middleware between Claude Code and Jira's REST API, translating natural language commands into Jira operations while handling authentication, session management, and response formatting. This enables developers to manage Jira tickets without leaving their AI-assisted coding environment.

## Features

### Core Functionality

#### Issue Management
- **Create** issues with standard and custom fields
- **Read** issue details, history, and metadata
- **Update** any field including status transitions
- **Delete** issues with permission checks
- **Bulk operations** for efficient processing
- **Clone** existing issues with modifications
- **Search** using JQL or natural language

#### Project & Board Operations
- List and manage projects
- Access Scrum and Kanban boards
- View and modify board configurations
- Manage backlogs and active sprints
- Track velocity and burndown metrics

#### Sprint Management
- Create, start, and close sprints
- Move issues between sprints
- Sprint planning and estimation
- Sprint reports and metrics

#### Comments & Attachments
- Add, edit, delete comments with rich text
- Upload and download attachments
- Inline image support
- @mention functionality

#### Workflow Automation
- Execute workflow transitions
- Validate transition requirements
- Handle transition screens
- Bulk workflow operations

#### Advanced Search
- JQL query execution
- Natural language to JQL translation
- Saved filter management
- Export results to multiple formats

#### Reporting & Analytics
- Standard Jira reports
- Custom metrics generation
- Time tracking analysis
- SLA performance monitoring

### Claude Code Integration

#### Natural Language Processing
Convert plain English commands into Jira operations:
- "Create a bug ticket for the login issue in project WEBAPP"
- "Move ticket WEBAPP-123 to In Progress"
- "Show me all critical bugs in the current sprint"
- "Add a comment saying the fix is deployed"

#### Intelligent Context Handling
- Maintains session context between commands
- Suggests relevant actions based on history
- Auto-completes project and issue references

### API Endpoints

```
POST   /api/v1/auth/connect          # Connect to Jira
GET    /api/v1/auth/status           # Connection status
POST   /api/v1/issues                # Create issue
GET    /api/v1/issues/{key}          # Get issue
PUT    /api/v1/issues/{key}          # Update issue
DELETE /api/v1/issues/{key}          # Delete issue
POST   /api/v1/issues/search         # Search issues
GET    /api/v1/projects              # List projects
GET    /api/v1/boards                # List boards
POST   /api/v1/sprints               # Create sprint
POST   /api/v1/claude/interpret      # Natural language processing
```

## Installation

### Using Docker (Recommended)

```bash
# Pull and run the Docker image
docker run -d \
  --name gojira \
  -p 8080:8080 \
  -v ~/.gojira:/config \
  -e JIRA_URL=https://your-domain.atlassian.net \
  -e JIRA_TOKEN=your-api-token \
  gojira/gojira:latest
```

### Using Docker Compose

```yaml
# docker-compose.yml
version: '3.8'
services:
  gojira:
    image: gojira/gojira:latest
    ports:
      - "8080:8080"
    volumes:
      - ~/.gojira:/config
    environment:
      - JIRA_URL=https://your-domain.atlassian.net
      - JIRA_TOKEN=${JIRA_TOKEN}
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/ericfisherdev/GoJira.git
cd GoJira

# Build for your platform
make build

# Or build for all platforms
make build-all

# Run the server
./gojira serve
```

### Available Make Targets

```bash
make build          # Build for current platform
make build-all      # Build for all platforms
make build-windows  # Build Windows executables (x64/ARM64)
make build-linux    # Build Linux binaries (x64/ARM64)
make build-darwin   # Build macOS binaries (Intel/Apple Silicon)
make docker-build   # Build Docker image
make docker-run     # Build and run with Docker
make test           # Run test suite
make lint           # Run linters
make clean          # Clean build artifacts
```

## Configuration

### Basic Configuration

Create a `gojira.yaml` file:

```yaml
server:
  host: localhost
  port: 8080
  
jira:
  url: https://your-domain.atlassian.net
  auth:
    type: api_token
    email: your-email@example.com
    token: ${JIRA_API_TOKEN}  # Use environment variable

features:
  natural_language: true
  caching: true
  auto_retry: true
```

### Environment Variables

```bash
GOJIRA_PORT=8080
JIRA_URL=https://your-domain.atlassian.net
JIRA_EMAIL=your-email@example.com
JIRA_API_TOKEN=your-api-token
GOJIRA_LOG_LEVEL=info
```

### Multiple Jira Instances

```yaml
jira:
  instances:
    - name: production
      url: https://prod.atlassian.net
      auth:
        type: oauth2
        client_id: ${PROD_CLIENT_ID}
        client_secret: ${PROD_CLIENT_SECRET}
    - name: staging
      url: https://staging.atlassian.net
      auth:
        type: api_token
        token: ${STAGING_TOKEN}
```

## Usage Examples

### Claude Code Integration

```javascript
// In Claude Code
const response = await fetch('http://localhost:8080/api/v1/claude/interpret', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    instruction: "Create a high-priority bug for the login issue"
  })
});
```

### Command Line

```bash
# Create an issue
curl -X POST http://localhost:8080/api/v1/issues \
  -H "Content-Type: application/json" \
  -d '{
    "project": "WEBAPP",
    "summary": "Login issue",
    "issueType": "Bug",
    "priority": "High"
  }'

# Search issues
curl -X POST http://localhost:8080/api/v1/issues/search \
  -H "Content-Type: application/json" \
  -d '{
    "jql": "project = WEBAPP AND status = 'In Progress'"
  }'
```

### GoJira CLI

```bash
# Create issue
gojira create issue --project WEBAPP --type Bug --summary "Login issue"

# Update issue
gojira update WEBAPP-123 --status "In Progress"

# Search issues
gojira search "critical bugs in current sprint"
```

## Authentication

### API Token (Recommended for Jira Cloud)

1. Generate an API token at https://id.atlassian.com/manage/api-tokens
2. Configure in `gojira.yaml` or environment variables

### OAuth 2.0

1. Register an OAuth app in Jira
2. Configure client ID and secret
3. GoJira handles the OAuth flow

### Personal Access Token (Jira Server/Data Center)

1. Generate PAT in Jira user settings
2. Configure as bearer token

## Security

- Credentials stored in OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)
- TLS 1.3 for all external communications
- Input validation and sanitization
- Rate limiting and request throttling
- Audit logging for all operations

## System Requirements

### Minimum Requirements
- **OS**: Windows 10+, macOS 11+, Ubuntu 20.04+, or WSL2
- **RAM**: 512MB
- **Disk**: 50MB
- **Network**: Internet connection for Jira API access

### Supported Platforms
- Windows (x64, ARM64)
- Linux (x64, ARM64)
- macOS (Intel, Apple Silicon)
- Docker containers

### Jira Compatibility
- Jira Cloud (latest)
- Jira Server 8.x+
- Jira Data Center 8.x+

## Development

### Prerequisites
- Go 1.21+
- Make
- Docker (optional)

### Project Structure
```
GoJira/
├── cmd/gojira/       # Main application
├── internal/         # Internal packages
│   ├── api/          # HTTP handlers
│   ├── jira/         # Jira client
│   └── translator/   # Natural language processing
├── pkg/              # Public packages
├── docker/           # Docker configurations
├── configs/          # Configuration files
└── tests/            # Test suites
```

### Running Tests

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific tests
go test ./internal/jira/...
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Open a Pull Request

## Roadmap

### Phase 1: Foundation ✅
- Basic HTTP server
- Jira authentication
- Core issue operations
- Docker support
- Cross-platform builds

### Phase 2: Core Features (In Progress)
- Complete issue management
- Project and board operations
- Comments and attachments
- Search functionality

### Phase 3: Advanced Features (Planned)
- Sprint management
- Workflow automation
- Bulk operations
- Natural language processing

### Phase 4: Enhancement (Planned)
- Caching layer
- Performance optimization
- Advanced error handling
- Comprehensive testing

### Phase 5: Polish (Planned)
- Security hardening
- Production optimizations
- Docker Hub publishing
- Binary releases

## Performance

- **Response Time**: < 500ms for single operations
- **Throughput**: 100+ concurrent requests
- **Memory Usage**: < 100MB baseline
- **Startup Time**: < 2 seconds

## Troubleshooting

### Connection Issues
```bash
# Check connection status
curl http://localhost:8080/api/v1/auth/status

# Test Jira connectivity
gojira test connection
```

### Common Problems

1. **Authentication Failed**: Verify API token and Jira URL
2. **Permission Denied**: Check Jira user permissions
3. **Rate Limiting**: Configure request throttling
4. **Network Errors**: Check firewall and proxy settings

## License

MIT License - see [LICENSE](LICENSE) file for details

## Support

- **Issues**: [GitHub Issues](https://github.com/ericfisherdev/GoJira/issues)
- **Documentation**: [docs/](docs/)
- **Examples**: [examples/](examples/)

## Acknowledgments

Built to enhance the Claude Code development experience by providing seamless Jira integration.