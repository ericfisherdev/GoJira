# GoJira Phase 3 Deployment Guide

## Overview

This guide covers the deployment and configuration of GoJira Phase 3, which introduces advanced features including Sprint Management, Workflow Operations, Bulk Operations, Natural Language Processing with Claude Code integration, and Performance Optimization.

## New Features in Phase 3

### 🚀 Sprint Management
- Advanced sprint lifecycle management
- Sprint velocity tracking and burndown charts
- Automated sprint planning and completion
- Real-time sprint reporting and analytics

### 🔄 Workflow Operations
- Advanced workflow state management
- Natural language workflow queries
- Bulk workflow transitions
- Custom workflow analytics

### ⚡ Bulk Operations
- High-performance bulk updates (1000+ issues)
- Intelligent batching and queue management
- Async operation processing
- Comprehensive error handling and retry logic

### 🤖 Natural Language Processing
- Claude Code integration for natural language commands
- Intelligent command interpretation with 85%+ accuracy
- Conversational context management
- Real-time command suggestions

### 📊 Performance Optimization
- Multi-level caching system (L1: Memory, L2: Redis, L3: Disk)
- Connection pooling with health checks
- Performance monitoring with P95/P99 percentiles
- Rate limiting and request optimization

## Prerequisites

### System Requirements
- **Go**: Version 1.21 or higher
- **Memory**: Minimum 512MB, Recommended 2GB
- **Storage**: 1GB available space for caching
- **Network**: HTTPS connectivity to Jira instance

### Optional Components
- **Redis**: For L2 caching (recommended for production)
- **Docker**: For containerized deployment
- **Prometheus**: For advanced monitoring

## Configuration

### Updated Configuration Structure

Create or update your `gojira.yaml` configuration file:

```yaml
# Server Configuration
server:
  host: "0.0.0.0"
  port: 8080
  mode: "production"  # development, production
  timeout: 30s
  max_connections: 1000

# Jira Configuration
jira:
  url: "https://your-domain.atlassian.net"
  auth:
    type: "api_token"  # api_token, oauth2, pat
    email: "your-email@example.com"
    token: "${JIRA_API_TOKEN}"
  
  # Connection Pool Settings (NEW)
  connection_pool:
    min_connections: 5
    max_connections: 50
    acquire_timeout: 10s
    idle_timeout: 30m
    health_check_interval: 5m

# Feature Flags (UPDATED)
features:
  natural_language: true      # Enable Claude integration
  caching: true              # Enable multi-level caching
  auto_retry: true           # Enable automatic retry logic
  bulk_operations: true      # Enable bulk operations
  performance_monitoring: true # Enable detailed monitoring
  advanced_workflows: true   # Enable advanced workflow features
  sprint_management: true    # Enable sprint management

# NEW: Multi-Level Caching Configuration
cache:
  strategy: "aggressive"     # default, aggressive, conservative
  
  # L1 Cache (Memory)
  memory:
    max_size: 1000
    ttl: "15m"
    cleanup_interval: "5m"
  
  # L2 Cache (Redis) - Optional
  redis:
    enabled: false
    host: "localhost"
    port: 6379
    password: ""
    db: 0
    ttl: "1h"
  
  # L3 Cache (Disk) - Optional
  disk:
    enabled: true
    directory: "./cache"
    max_size: "100MB"
    ttl: "24h"
    compression: true

# NEW: Performance Configuration
performance:
  monitoring:
    enabled: true
    sample_size: 2000
    report_interval: "10m"
    slow_threshold: "2s"
    enable_percentiles: true
  
  # Batch Processing
  batch:
    worker_count: 10
    queue_size: 2000
    batch_size: 20
    flush_interval: "500ms"
    max_retries: 3
    retry_delay: "1s"
    requests_per_second: 25.0
    enable_rate_limiting: true
    timeout_duration: "30s"
  
  # Rate Limiting
  rate_limit:
    enabled: true
    requests_per_second: 100
    burst_size: 200

# NEW: Claude Configuration
claude:
  session_timeout: "30m"
  max_history: 100
  suggestion_count: 5
  enable_formatting: true
  enable_summarization: true
  confidence_threshold: 0.7

# Logging Configuration (UPDATED)
logging:
  level: "info"              # debug, info, warn, error
  format: "json"             # console, json
  output: "stdout"           # stdout, stderr, file
  file_path: "./logs/gojira.log"
  max_size: 100              # MB
  max_backups: 10
  max_age: 30                # days

# NEW: Security Configuration
security:
  enable_cors: true
  cors_origins: ["*"]
  enable_rate_limiting: true
  api_key_header: "X-API-Key"
  session_encryption: true

# NEW: Monitoring Configuration
monitoring:
  prometheus:
    enabled: false
    endpoint: "/metrics"
    namespace: "gojira"
  
  health_checks:
    enabled: true
    interval: "30s"
    timeout: "10s"
    
  alerts:
    high_error_rate_threshold: 0.05  # 5%
    slow_response_threshold: "2s"
    high_memory_usage_threshold: 0.8  # 80%
```

### Environment Variables

All configuration values can be overridden using environment variables with the `GOJIRA_` prefix:

```bash
# Core Configuration
export GOJIRA_JIRA_URL="https://your-domain.atlassian.net"
export GOJIRA_JIRA_AUTH_EMAIL="your-email@example.com"
export GOJIRA_JIRA_AUTH_TOKEN="your-api-token"

# Performance Settings
export GOJIRA_PERFORMANCE_BATCH_WORKER_COUNT="15"
export GOJIRA_PERFORMANCE_BATCH_REQUESTS_PER_SECOND="30.0"
export GOJIRA_CACHE_STRATEGY="aggressive"

# Feature Flags
export GOJIRA_FEATURES_NATURAL_LANGUAGE="true"
export GOJIRA_FEATURES_CACHING="true"
export GOJIRA_FEATURES_BULK_OPERATIONS="true"

# Security
export GOJIRA_SECURITY_API_KEY_HEADER="X-GoJira-Key"

# Monitoring
export GOJIRA_MONITORING_PROMETHEUS_ENABLED="true"
```

## Deployment Options

### Option 1: Direct Binary Deployment

1. **Build the application**:
   ```bash
   make build
   # Or for specific platform
   make build-linux  # For Linux
   make build-darwin # For macOS
   make build-windows # For Windows
   ```

2. **Create configuration**:
   ```bash
   cp configs/gojira.example.yaml configs/gojira.yaml
   # Edit the configuration file with your settings
   ```

3. **Run the application**:
   ```bash
   ./dist/gojira serve --config configs/gojira.yaml
   ```

4. **Verify deployment**:
   ```bash
   curl http://localhost:8080/health
   curl http://localhost:8080/ready
   ```

### Option 2: Docker Deployment

1. **Build Docker image**:
   ```bash
   make docker-build
   ```

2. **Run with Docker**:
   ```bash
   docker run -d \
     --name gojira \
     -p 8080:8080 \
     -e GOJIRA_JIRA_URL="https://your-domain.atlassian.net" \
     -e GOJIRA_JIRA_AUTH_EMAIL="your-email@example.com" \
     -e GOJIRA_JIRA_AUTH_TOKEN="your-api-token" \
     -v $(pwd)/cache:/app/cache \
     -v $(pwd)/logs:/app/logs \
     gojira:latest
   ```

3. **Using Docker Compose**:
   ```bash
   # For development
   docker-compose -f docker/docker-compose.dev.yml up

   # For production
   docker-compose -f docker/docker-compose.yml up -d
   ```

### Option 3: Production Deployment with Redis

1. **Docker Compose with Redis**:
   ```yaml
   version: '3.8'
   services:
     gojira:
       image: gojira:latest
       ports:
         - "8080:8080"
       environment:
         - GOJIRA_CACHE_REDIS_ENABLED=true
         - GOJIRA_CACHE_REDIS_HOST=redis
       depends_on:
         - redis
       volumes:
         - ./cache:/app/cache
         - ./logs:/app/logs

     redis:
       image: redis:7-alpine
       ports:
         - "6379:6379"
       volumes:
         - redis_data:/data
       command: redis-server --appendonly yes

   volumes:
     redis_data:
   ```

## Performance Configuration Presets

### Development Environment
```yaml
performance:
  batch:
    worker_count: 3
    queue_size: 100
    batch_size: 5
    requests_per_second: 10.0

cache:
  strategy: "default"
  memory:
    max_size: 500
    ttl: "5m"

monitoring:
  enabled: true
  report_interval: "1m"
```

### Production Environment  
```yaml
performance:
  batch:
    worker_count: 15
    queue_size: 5000
    batch_size: 50
    requests_per_second: 50.0

cache:
  strategy: "aggressive"
  memory:
    max_size: 5000
    ttl: "30m"
  redis:
    enabled: true
    ttl: "2h"

monitoring:
  enabled: true
  report_interval: "5m"
  prometheus:
    enabled: true
```

### High-Volume Environment
```yaml
performance:
  batch:
    worker_count: 25
    queue_size: 10000
    batch_size: 100
    requests_per_second: 100.0

cache:
  strategy: "aggressive"
  memory:
    max_size: 10000
    ttl: "1h"
  redis:
    enabled: true
    ttl: "4h"
  disk:
    enabled: true
    max_size: "1GB"

connection_pool:
  min_connections: 10
  max_connections: 100
```

## API Usage Examples

### Sprint Management

#### Create a Sprint
```bash
curl -X POST http://localhost:8080/api/v1/sprints \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "name": "Phase 3 Development Sprint",
    "goal": "Implement advanced features",
    "boardId": "10",
    "startDate": "2024-02-01T09:00:00Z",
    "endDate": "2024-02-14T17:00:00Z"
  }'
```

#### Start a Sprint
```bash
curl -X POST http://localhost:8080/api/v1/sprints/1/start \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "startDate": "2024-02-01T09:00:00Z"
  }'
```

#### Get Sprint Report
```bash
curl -X GET http://localhost:8080/api/v1/sprints/1/report \
  -H "X-API-Key: your-api-key"
```

### Bulk Operations

#### Bulk Update Issues
```bash
curl -X POST http://localhost:8080/api/v1/bulk/update \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "issueKeys": ["PROJ-1", "PROJ-2", "PROJ-3"],
    "update": {
      "priority": {"name": "High"},
      "assignee": {"accountId": "user123"}
    },
    "comment": "Bulk priority update"
  }'
```

#### Bulk Transition Issues
```bash
curl -X POST http://localhost:8080/api/v1/bulk/transition \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "issueKeys": ["PROJ-1", "PROJ-2"],
    "transitionId": "21",
    "comment": "Moving to In Progress"
  }'
```

### Natural Language Processing

#### Interpret Command
```bash
curl -X POST http://localhost:8080/api/v1/claude/interpret \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "command": "Create a critical bug for SQL injection in auth.go line 145",
    "context": {
      "currentProject": "SECURITY",
      "currentUser": "john.doe@company.com"
    }
  }'
```

#### Execute Interpreted Command
```bash
curl -X POST http://localhost:8080/api/v1/claude/execute \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "sessionId": "session-123",
    "actionId": "action-456",
    "confirmExecution": true
  }'
```

### Advanced Workflow Queries

#### Natural Language Workflow Query
```bash
curl -X POST http://localhost:8080/api/v1/workflows/advanced/query \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "query": "Find all issues in TODO state older than 7 days",
    "projectKey": "PROJ",
    "maxResults": 50,
    "useNaturalLanguage": true
  }'
```

## Performance Monitoring

### Get Performance Metrics
```bash
curl -X GET http://localhost:8080/api/v1/performance/metrics \
  -H "X-API-Key: your-api-key"
```

### Cache Statistics
```bash
curl -X GET http://localhost:8080/api/v1/performance/cache \
  -H "X-API-Key: your-api-key"
```

### Clear Cache
```bash
curl -X DELETE http://localhost:8080/api/v1/performance/cache \
  -H "X-API-Key: your-api-key"
```

## Performance Benchmarks

### Expected Performance Metrics

| Operation | Target Performance | Notes |
|-----------|-------------------|--------|
| Bulk Operations | < 100ms per issue | For batches of 1000+ issues |
| NLP Processing | < 500ms per command | Natural language interpretation |
| Sprint Report Generation | < 2 seconds | Complete sprint analytics |
| Cache Hit Rate | > 70% | Multi-level caching system |
| Concurrent Operations | 50+ simultaneous | Connection pooling enabled |
| API Response Time | < 200ms | 95th percentile for single operations |
| Memory Usage | < 500MB | Baseline without heavy caching |

### Performance Testing

Run performance tests to verify your deployment:

```bash
# Run integration tests
make test-integration

# Run performance benchmarks
make test-benchmark

# Run load testing (if available)
make test-load
```

## Troubleshooting

### Common Issues

#### 1. High Memory Usage
**Symptoms**: Memory usage continuously increasing
**Solutions**:
- Reduce cache sizes in configuration
- Enable cache cleanup intervals
- Monitor for memory leaks in logs

```yaml
cache:
  memory:
    max_size: 500  # Reduce from default 1000
    cleanup_interval: "2m"  # More frequent cleanup
```

#### 2. Slow API Responses
**Symptoms**: API responses taking > 2 seconds
**Solutions**:
- Enable caching if not already enabled
- Increase connection pool size
- Check Jira API performance

```yaml
jira:
  connection_pool:
    max_connections: 100  # Increase pool size
    acquire_timeout: 5s   # Reduce timeout

performance:
  rate_limit:
    requests_per_second: 50  # Reduce if Jira is slow
```

#### 3. Claude Integration Errors
**Symptoms**: Natural language processing failing
**Solutions**:
- Check confidence threshold settings
- Verify session management
- Enable debug logging

```yaml
claude:
  confidence_threshold: 0.5  # Lower threshold
  session_timeout: "60m"     # Longer timeout

logging:
  level: "debug"  # Enable debug logging
```

#### 4. Cache Miss Rate Too High
**Symptoms**: Cache hit rate < 50%
**Solutions**:
- Increase cache TTL values
- Enable additional cache levels
- Review cache strategy

```yaml
cache:
  strategy: "aggressive"
  memory:
    ttl: "30m"  # Increase TTL
  redis:
    enabled: true  # Enable L2 cache
    ttl: "2h"
```

### Diagnostic Commands

```bash
# Check application health
curl http://localhost:8080/health

# Get detailed status
curl http://localhost:8080/ready

# View performance metrics
curl http://localhost:8080/api/v1/performance/metrics

# Check cache statistics
curl http://localhost:8080/api/v1/performance/cache

# View configuration (if enabled)
./dist/gojira --show-config
```

### Log Analysis

Monitor these log patterns for issues:

```bash
# Check for performance warnings
grep "Slow operation detected" logs/gojira.log

# Monitor error rates
grep "ERROR" logs/gojira.log | wc -l

# Check cache performance
grep "cache" logs/gojira.log | grep -E "(hit|miss|evict)"

# Monitor connection pool
grep "connection pool" logs/gojira.log
```

## Security Considerations

### API Security
- Use HTTPS in production
- Implement proper API key management
- Enable CORS restrictions for web clients
- Monitor for unusual API usage patterns

### Jira Credentials
- Store credentials in environment variables
- Use Jira API tokens instead of passwords
- Rotate credentials regularly
- Implement proper access controls

### Caching Security
- Ensure cached data doesn't contain sensitive information
- Use encryption for session data
- Implement proper cache invalidation
- Monitor cache for unauthorized access patterns

## Monitoring and Alerting

### Key Metrics to Monitor

1. **Performance Metrics**
   - API response times (P95, P99)
   - Error rates
   - Throughput (requests/second)
   - Cache hit ratios

2. **System Metrics**
   - Memory usage
   - CPU utilization
   - Disk space (for disk cache)
   - Network connectivity to Jira

3. **Application Metrics**
   - Active sessions
   - Queue depths
   - Connection pool utilization
   - Failed operations

### Prometheus Integration

If using Prometheus monitoring:

```yaml
monitoring:
  prometheus:
    enabled: true
    endpoint: "/metrics"
    namespace: "gojira"
```

Sample Prometheus queries:
```promql
# API response time 95th percentile
histogram_quantile(0.95, rate(gojira_request_duration_seconds_bucket[5m]))

# Error rate
rate(gojira_errors_total[5m]) / rate(gojira_requests_total[5m])

# Cache hit ratio
gojira_cache_hits_total / (gojira_cache_hits_total + gojira_cache_misses_total)
```

## Scaling Considerations

### Horizontal Scaling
- GoJira can be run in multiple instances
- Use Redis for shared caching across instances
- Implement load balancing
- Monitor connection pool limits across instances

### Vertical Scaling
- Increase memory for larger caches
- Add CPU cores for higher concurrency
- Optimize disk I/O for disk caching
- Monitor resource utilization

### Database Considerations
- Jira API rate limits may be the bottleneck
- Consider implementing request queuing
- Monitor Jira instance performance
- Implement circuit breakers for Jira API calls

## Migration from Previous Versions

### From Phase 2 to Phase 3

1. **Update Configuration**: Add new Phase 3 configuration sections
2. **Test Features**: Verify new features work with your Jira instance
3. **Performance Tuning**: Adjust cache and performance settings
4. **Monitor**: Watch for any performance regressions

### Configuration Migration Script

```bash
#!/bin/bash
# migrate-config.sh

echo "Migrating GoJira configuration from Phase 2 to Phase 3..."

# Backup existing configuration
cp configs/gojira.yaml configs/gojira.yaml.backup

# Add Phase 3 configuration sections
cat >> configs/gojira.yaml << 'EOF'

# Phase 3 additions
cache:
  strategy: "default"
  memory:
    max_size: 1000
    ttl: "15m"

performance:
  monitoring:
    enabled: true
  batch:
    worker_count: 5

claude:
  session_timeout: "30m"
  max_history: 100

features:
  natural_language: true
  bulk_operations: true
  advanced_workflows: true
  sprint_management: true
EOF

echo "Configuration migrated. Please review and adjust settings as needed."
echo "Backup saved as configs/gojira.yaml.backup"
```

## Support and Maintenance

### Regular Maintenance Tasks

1. **Log Rotation**: Ensure logs are rotated to prevent disk space issues
2. **Cache Cleanup**: Monitor cache sizes and clean up if needed
3. **Performance Review**: Regular review of performance metrics
4. **Security Updates**: Keep dependencies updated
5. **Configuration Review**: Periodic review of configuration settings

### Getting Support

- **Documentation**: Check this guide and API documentation
- **Logs**: Always include relevant log entries when reporting issues
- **Configuration**: Provide configuration details (sanitized)
- **Environment**: Include deployment environment details

This deployment guide covers all aspects of deploying and configuring GoJira Phase 3. For additional support or questions, refer to the project documentation or contact the development team.