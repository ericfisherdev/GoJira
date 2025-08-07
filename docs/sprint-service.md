# Sprint Service Documentation

The Sprint Service provides advanced business logic and intelligent operations for Sprint management in GoJira.

## Features

### 1. Sprint Validation
- Validates sprint name, goal, and duration constraints
- Ensures proper date ordering and reasonable sprint lengths
- Configurable validation rules

### 2. Active Sprint Management
- Get all active sprints across boards
- Health checks for sprint progress
- Auto-start sprints when conditions are met

### 3. Sprint Metrics and Analytics
- Detailed sprint metrics calculation
- Burndown rate tracking
- Completion percentage analysis
- Status distribution tracking

### 4. Sprint Predictions
- AI-powered sprint success predictions
- Risk level assessment (Low, Medium, High)
- Required burndown rate calculations
- Automated recommendations

### 5. Sprint Completion
- Intelligent sprint closure with reports
- Automated handling of incomplete issues
- Completion rate and velocity calculations

## API Endpoints

### Basic Sprint Operations
- `GET /api/v1/sprints` - List sprints for a board
- `POST /api/v1/sprints` - Create new sprint
- `GET /api/v1/sprints/{id}` - Get sprint details
- `PUT /api/v1/sprints/{id}` - Update sprint
- `DELETE /api/v1/sprints/{id}` - Delete sprint

### Advanced Sprint Operations
- `GET /api/v1/sprints/active` - Get all active sprints
- `GET /api/v1/sprints/upcoming?boardId={id}` - Get upcoming sprints
- `GET /api/v1/sprints/health` - Sprint health check
- `POST /api/v1/sprints/validate` - Validate sprint request

### Sprint Lifecycle
- `POST /api/v1/sprints/{id}/start` - Start sprint manually
- `POST /api/v1/sprints/{id}/auto-start` - Auto-start sprint
- `POST /api/v1/sprints/{id}/close` - Close sprint
- `POST /api/v1/sprints/{id}/complete` - Complete with report

### Analytics and Insights
- `GET /api/v1/sprints/{id}/metrics` - Detailed metrics
- `GET /api/v1/sprints/{id}/predict` - Success prediction
- `GET /api/v1/sprints/{id}/report` - Sprint report

### Utility Operations
- `POST /api/v1/sprints/{id}/clone` - Clone sprint
- `POST /api/v1/sprints/{id}/issues` - Move issues to sprint

## Sprint Validation Rules

### Default Constraints
- **Name**: Required, max 255 characters
- **Goal**: Optional, max 1000 characters  
- **Duration**: Minimum 7 days, maximum 30 days
- **Board ID**: Required, must be positive integer

### Date Validation
- Start date must be before end date
- Duration must be within acceptable range
- Future dates are recommended for planning

## Sprint Metrics

### Calculated Metrics
- **Total Issues**: Count of all issues in sprint
- **Completed Issues**: Issues in "done" status category
- **In Progress Issues**: Issues in "indeterminate" status category
- **Todo Issues**: Issues in "new" status category
- **Completion Percentage**: (Completed / Total) * 100
- **Burndown Rate**: Issues completed per day
- **Status Distribution**: Count by status name

### Prediction Algorithms
The service uses simple heuristics to predict sprint success:

1. **Current Progress**: Completion percentage vs time elapsed
2. **Burndown Rate**: Current vs required velocity
3. **Risk Assessment**: Based on progress patterns

## Usage Examples

### Create Sprint with Validation
```bash
curl -X POST /api/v1/sprints/validate \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Sprint 42",
    "goal": "Complete user authentication",
    "originBoardId": 1
  }'
```

### Get Sprint Health Check
```bash
curl -X GET /api/v1/sprints/health
```

### Auto-start Sprint
```bash
curl -X POST /api/v1/sprints/123/auto-start
```

### Get Sprint Predictions
```bash
curl -X GET /api/v1/sprints/123/predict
```

### Complete Sprint with Report
```bash
curl -X POST /api/v1/sprints/123/complete \
  -H "Content-Type: application/json" \
  -d '{"moveIncomplete": "backlog"}'
```

## Error Handling

The service provides structured error responses with:
- Error codes and messages
- Validation failure details
- HTTP status codes (400, 404, 500)
- Logging for debugging

## Testing

### Unit Tests
- Sprint validation logic
- Metrics calculations
- Date handling edge cases

### Integration Tests
- API endpoint functionality
- Service layer integration
- Mock client interactions

### Test Coverage
- Validation scenarios
- Business logic paths
- Error conditions
- Edge cases

## Performance Considerations

### Caching
- In-memory sprint cache with TTL
- Board-to-sprint mappings
- Metrics calculation caching

### Optimization
- Batch operations for multiple sprints
- Lazy loading of sprint details
- Efficient date calculations

## Future Enhancements

1. **Machine Learning**: Advanced prediction models
2. **Historical Analysis**: Sprint performance trends
3. **Team Velocity**: Cross-sprint velocity tracking
4. **Automated Planning**: AI-powered sprint planning
5. **Integration**: External calendar and notification systems