package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ericfisherdev/GoJira/internal/claude"
	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

var jiraClient *jira.Client

// SetJiraClient sets the global Jira client
func SetJiraClient(client *jira.Client) {
	jiraClient = client
}

type CreateIssueRequest struct {
	Project     string            `json:"project" validate:"required"`
	Summary     string            `json:"summary" validate:"required"`
	Description string            `json:"description,omitempty"`
	IssueType   string            `json:"issueType" validate:"required"`
	Priority    string            `json:"priority,omitempty"`
	Assignee    string            `json:"assignee,omitempty"`
	Labels      []string          `json:"labels,omitempty"`
	Components  []string          `json:"components,omitempty"`
	Parent      string            `json:"parent,omitempty"` // For subtasks
	CustomFields map[string]interface{} `json:"customFields,omitempty"`
}

func (cir *CreateIssueRequest) Bind(r *http.Request) error {
	if cir.Project == "" {
		return fmt.Errorf("project is required")
	}
	if cir.Summary == "" {
		return fmt.Errorf("summary is required")
	}
	if cir.IssueType == "" {
		return fmt.Errorf("issueType is required")
	}
	return nil
}

type UpdateIssueRequest struct {
	Summary      *string           `json:"summary,omitempty"`
	Description  *string           `json:"description,omitempty"`
	Priority     *string           `json:"priority,omitempty"`
	Assignee     *string           `json:"assignee,omitempty"`
	Labels       []string          `json:"labels,omitempty"`
	Components   []string          `json:"components,omitempty"`
	CustomFields map[string]interface{} `json:"customFields,omitempty"`
}

func (ur *UpdateIssueRequest) Bind(r *http.Request) error {
	// No specific validation needed for update requests
	return nil
}

type TransitionIssueRequest struct {
	TransitionID string            `json:"transitionId" validate:"required"`
	Comment      string            `json:"comment,omitempty"`
	Fields       map[string]interface{} `json:"fields,omitempty"`
}

func (tir *TransitionIssueRequest) Bind(r *http.Request) error {
	if tir.TransitionID == "" {
		return fmt.Errorf("transitionId is required")
	}
	return nil
}

type SearchIssuesRequest struct {
	JQL        string   `json:"jql" validate:"required"`
	StartAt    int      `json:"startAt,omitempty"`
	MaxResults int      `json:"maxResults,omitempty"`
	Expand     []string `json:"expand,omitempty"`
}

func (sir *SearchIssuesRequest) Bind(r *http.Request) error {
	if sir.JQL == "" {
		return fmt.Errorf("jql is required")
	}
	if sir.MaxResults <= 0 {
		sir.MaxResults = 50 // Default
	}
	if sir.MaxResults > 1000 {
		sir.MaxResults = 1000 // Max allowed
	}
	return nil
}

type IssueResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

func (ir *IssueResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// CreateIssue creates a new Jira issue
func CreateIssue(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	var req CreateIssueRequest
	if err := render.Bind(r, &req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Build Jira issue fields
	fields := map[string]interface{}{
		"project": map[string]interface{}{
			"key": req.Project,
		},
		"summary": req.Summary,
		"issuetype": map[string]interface{}{
			"name": req.IssueType,
		},
	}

	// Add optional fields
	if req.Description != "" {
		fields["description"] = req.Description
	}

	if req.Priority != "" {
		fields["priority"] = map[string]interface{}{
			"name": req.Priority,
		}
	}

	if req.Assignee != "" {
		fields["assignee"] = map[string]interface{}{
			"accountId": req.Assignee,
		}
	}

	if len(req.Labels) > 0 {
		fields["labels"] = req.Labels
	}

	if len(req.Components) > 0 {
		components := make([]map[string]interface{}, len(req.Components))
		for i, comp := range req.Components {
			components[i] = map[string]interface{}{"name": comp}
		}
		fields["components"] = components
	}

	if req.Parent != "" {
		fields["parent"] = map[string]interface{}{
			"key": req.Parent,
		}
	}

	// Add custom fields
	for k, v := range req.CustomFields {
		fields[k] = v
	}

	// Create the issue
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	createReq := &jira.CreateIssueRequest{
		Fields: fields,
	}

	issue, err := jiraClient.CreateIssue(ctx, createReq)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data:    issue,
	}

	render.Status(r, http.StatusCreated)
	render.Render(w, r, response)
}

// GetIssue retrieves an issue by key
func GetIssue(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	issueKey := chi.URLParam(r, "key")
	if issueKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("missing issue key")))
		return
	}

	// Parse expand parameter
	expandParam := r.URL.Query().Get("expand")
	var expand []string
	if expandParam != "" {
		expand = strings.Split(expandParam, ",")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	issue, err := jiraClient.GetIssue(ctx, issueKey, expand)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			render.Render(w, r, ErrNotFound(fmt.Sprintf("issue %s", issueKey)))
		} else {
			render.Render(w, r, ErrInternalServer(err))
		}
		return
	}

	response := &IssueResponse{
		Success: true,
		Data:    issue,
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// UpdateIssue updates an existing issue
func UpdateIssue(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	issueKey := chi.URLParam(r, "key")
	if issueKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("missing issue key")))
		return
	}

	var req UpdateIssueRequest
	if err := render.Bind(r, &req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Build update fields
	fields := make(map[string]interface{})

	if req.Summary != nil {
		fields["summary"] = *req.Summary
	}

	if req.Description != nil {
		fields["description"] = *req.Description
	}

	if req.Priority != nil {
		fields["priority"] = map[string]interface{}{
			"name": *req.Priority,
		}
	}

	if req.Assignee != nil {
		if *req.Assignee == "" {
			fields["assignee"] = nil // Unassign
		} else {
			fields["assignee"] = map[string]interface{}{
				"accountId": *req.Assignee,
			}
		}
	}

	if len(req.Labels) > 0 {
		fields["labels"] = req.Labels
	}

	if len(req.Components) > 0 {
		components := make([]map[string]interface{}, len(req.Components))
		for i, comp := range req.Components {
			components[i] = map[string]interface{}{"name": comp}
		}
		fields["components"] = components
	}

	// Add custom fields
	for k, v := range req.CustomFields {
		fields[k] = v
	}

	if len(fields) == 0 {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("no fields to update")))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	updateReq := &jira.UpdateIssueRequest{
		Fields: fields,
	}

	err := jiraClient.UpdateIssue(ctx, issueKey, updateReq)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			render.Render(w, r, ErrNotFound(fmt.Sprintf("issue %s", issueKey)))
		} else {
			render.Render(w, r, ErrInternalServer(err))
		}
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"key":     issueKey,
			"updated": true,
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// DeleteIssue deletes an issue
func DeleteIssue(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	issueKey := chi.URLParam(r, "key")
	if issueKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("missing issue key")))
		return
	}

	// Check for deleteSubtasks parameter
	deleteSubtasks := r.URL.Query().Get("deleteSubtasks") == "true"

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	err := jiraClient.DeleteIssue(ctx, issueKey, deleteSubtasks)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			render.Render(w, r, ErrNotFound(fmt.Sprintf("issue %s", issueKey)))
		} else {
			render.Render(w, r, ErrInternalServer(err))
		}
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"key":     issueKey,
			"deleted": true,
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// SearchIssues searches for issues using JQL
func SearchIssues(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	// Handle both GET with query params and POST with JSON body
	var searchReq SearchIssuesRequest

	if r.Method == "GET" {
		searchReq.JQL = r.URL.Query().Get("jql")
		if startAt := r.URL.Query().Get("startAt"); startAt != "" {
			if val, err := strconv.Atoi(startAt); err == nil {
				searchReq.StartAt = val
			}
		}
		if maxResults := r.URL.Query().Get("maxResults"); maxResults != "" {
			if val, err := strconv.Atoi(maxResults); err == nil {
				searchReq.MaxResults = val
			}
		}
		if expand := r.URL.Query().Get("expand"); expand != "" {
			searchReq.Expand = strings.Split(expand, ",")
		}
	} else {
		if err := render.Bind(r, &searchReq); err != nil {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}
	}

	if err := searchReq.Bind(r); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second) // Longer timeout for searches
	defer cancel()

	results, err := jiraClient.SearchIssues(ctx, searchReq.JQL, searchReq.StartAt, searchReq.MaxResults, searchReq.Expand)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data:    results,
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// GetIssueTransitions gets available transitions for an issue
func GetIssueTransitions(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	issueKey := chi.URLParam(r, "key")
	if issueKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("missing issue key")))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	transitions, err := jiraClient.GetIssueTransitions(ctx, issueKey)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			render.Render(w, r, ErrNotFound(fmt.Sprintf("issue %s", issueKey)))
		} else {
			render.Render(w, r, ErrInternalServer(err))
		}
		return
	}

	response := &IssueResponse{
		Success: true,
		Data:    transitions,
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// TransitionIssue transitions an issue to a new status
func TransitionIssue(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	issueKey := chi.URLParam(r, "key")
	if issueKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("missing issue key")))
		return
	}

	var req TransitionIssueRequest
	if err := render.Bind(r, &req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Build transition request
	transitionReq := &jira.TransitionRequest{
		Transition: jira.Transition{
			ID: req.TransitionID,
		},
	}

	// Add comment if provided
	if req.Comment != "" {
		transitionReq.Update = map[string]interface{}{
			"comment": []map[string]interface{}{
				{
					"add": map[string]interface{}{
						"body": req.Comment,
					},
				},
			},
		}
	}

	// Add fields if provided
	if len(req.Fields) > 0 {
		transitionReq.Fields = req.Fields
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	err := jiraClient.TransitionIssue(ctx, issueKey, transitionReq)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			render.Render(w, r, ErrNotFound(fmt.Sprintf("issue %s", issueKey)))
		} else {
			render.Render(w, r, ErrInternalServer(err))
		}
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"key":           issueKey,
			"transitioned": true,
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// GetIssueLinks retrieves links for an issue
func GetIssueLinks(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	issueKey := chi.URLParam(r, "key")
	if issueKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("missing issue key")))
		return
	}

	links, err := jiraClient.GetIssueLinks(issueKey)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"issueKey": issueKey,
			"links":    links,
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// CreateLinkRequest for creating issue links
type CreateLinkRequest struct {
	InwardIssue  string `json:"inwardIssue"`
	OutwardIssue string `json:"outwardIssue"`
	LinkType     string `json:"linkType"`
	Comment      string `json:"comment,omitempty"`
}

func (c *CreateLinkRequest) Bind(r *http.Request) error {
	if c.InwardIssue == "" {
		return fmt.Errorf("inwardIssue is required")
	}
	if c.OutwardIssue == "" {
		return fmt.Errorf("outwardIssue is required")
	}
	if c.LinkType == "" {
		return fmt.Errorf("linkType is required")
	}
	return nil
}

// CreateIssueLink creates a link between two issues
func CreateIssueLink(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	var req CreateLinkRequest
	if err := render.Bind(r, &req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	err := jiraClient.CreateIssueLink(req.InwardIssue, req.OutwardIssue, req.LinkType, req.Comment)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"message":      "Issue link created successfully",
			"inwardIssue":  req.InwardIssue,
			"outwardIssue": req.OutwardIssue,
			"linkType":     req.LinkType,
		},
	}

	render.Status(r, http.StatusCreated)
	render.Render(w, r, response)
}

// DeleteIssueLink deletes a link between issues
func DeleteIssueLink(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	linkID := chi.URLParam(r, "id")
	if linkID == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("missing link ID")))
		return
	}

	err := jiraClient.DeleteIssueLink(linkID)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"message": "Issue link deleted successfully",
			"linkId":  linkID,
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// GetLinkTypes retrieves available link types
func GetLinkTypes(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	linkTypes, err := jiraClient.GetIssueLinkTypes()
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"linkTypes": linkTypes,
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// GetCustomFields retrieves custom fields for a project
func GetCustomFields(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	projectKey := chi.URLParam(r, "key")
	if projectKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("missing project key")))
		return
	}

	customFields, err := jiraClient.GetCustomFields(projectKey)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"projectKey":   projectKey,
			"customFields": customFields,
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// AdvancedSearchRequest represents an advanced search request
type AdvancedSearchRequest struct {
	JQL        string   `json:"jql" binding:"required"`
	StartAt    int      `json:"startAt,omitempty"`
	MaxResults int      `json:"maxResults,omitempty"`
	Fields     []string `json:"fields,omitempty"`
	Expand     []string `json:"expand,omitempty"`
	Properties []string `json:"properties,omitempty"`
}

func (a *AdvancedSearchRequest) Bind(r *http.Request) error {
	if a.JQL == "" {
		return fmt.Errorf("jql is required")
	}
	if a.MaxResults <= 0 {
		a.MaxResults = 50 // Default
	}
	if a.MaxResults > 1000 {
		a.MaxResults = 1000 // Max allowed
	}
	return nil
}

// AdvancedSearchIssues performs advanced search with full request body support
func AdvancedSearchIssues(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	var req AdvancedSearchRequest
	if err := render.Bind(r, &req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	searchReq := jira.SearchRequest{
		JQL:        req.JQL,
		StartAt:    req.StartAt,
		MaxResults: req.MaxResults,
		Fields:     req.Fields,
		Expand:     req.Expand,
		Properties: req.Properties,
	}

	result, err := jiraClient.SearchIssuesAdvanced(searchReq)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"startAt":         result.StartAt,
			"maxResults":      result.MaxResults,
			"total":           result.Total,
			"issues":          result.Issues,
			"warningMessages": result.WarningMessages,
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// ValidateJQL validates a JQL query
func ValidateJQL(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	jqlQuery := r.URL.Query().Get("jql")
	if jqlQuery == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("jql parameter is required")))
		return
	}

	isValid, errors, err := jiraClient.ValidateJQL(jqlQuery)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"jql":     jqlQuery,
			"valid":   isValid,
			"errors":  errors,
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// GetJQLSuggestions gets JQL autocomplete suggestions
func GetJQLSuggestions(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	fieldName := r.URL.Query().Get("fieldName")
	if fieldName == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("fieldName parameter is required")))
		return
	}

	fieldValue := r.URL.Query().Get("fieldValue")
	
	suggestions, err := jiraClient.GetJQLSuggestions(fieldName, fieldValue)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"fieldName":   fieldName,
			"fieldValue":  fieldValue,
			"suggestions": suggestions,
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// GetAllFilters gets all saved filters
func GetAllFilters(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	filters, err := jiraClient.GetAllFilters()
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"filters": filters,
			"count":   len(filters),
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// GetFilter gets a specific filter by ID
func GetFilter(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	filterID := chi.URLParam(r, "id")
	if filterID == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("missing filter ID")))
		return
	}

	filter, err := jiraClient.GetFilter(filterID)
	if err != nil {
		if err.Error() == "filter not found" {
			render.Render(w, r, ErrNotFound("filter"))
			return
		}
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data:    filter,
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// SearchWithFilter searches using a saved filter
func SearchWithFilter(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	filterID := chi.URLParam(r, "id")
	if filterID == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("missing filter ID")))
		return
	}

	// Parse pagination parameters
	startAt := 0
	maxResults := 50

	if startAtStr := r.URL.Query().Get("startAt"); startAtStr != "" {
		if val, err := strconv.Atoi(startAtStr); err == nil && val >= 0 {
			startAt = val
		}
	}

	if maxResultsStr := r.URL.Query().Get("maxResults"); maxResultsStr != "" {
		if val, err := strconv.Atoi(maxResultsStr); err == nil && val > 0 && val <= 1000 {
			maxResults = val
		}
	}

	result, err := jiraClient.SearchWithFilter(filterID, startAt, maxResults)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"filterID":        filterID,
			"startAt":         result.StartAt,
			"maxResults":      result.MaxResults,
			"total":           result.Total,
			"issues":          result.Issues,
			"warningMessages": result.WarningMessages,
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// GetJQLFields gets available JQL fields for autocomplete
func GetJQLFields(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	fields, err := jiraClient.GetJQLFields()
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"fields": fields,
			"count":  len(fields),
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// GetJQLFunctions gets available JQL functions for autocomplete
func GetJQLFunctions(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	functions, err := jiraClient.GetJQLFunctions()
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"functions": functions,
			"count":     len(functions),
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// SearchWithPaginationHandler performs search with enhanced pagination
func SearchWithPaginationHandler(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	var req AdvancedSearchRequest
	if err := render.Bind(r, &req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	searchReq := jira.SearchRequest{
		JQL:        req.JQL,
		StartAt:    req.StartAt,
		MaxResults: req.MaxResults,
		Fields:     req.Fields,
		Expand:     req.Expand,
		Properties: req.Properties,
	}

	result, err := jiraClient.SearchWithPagination(searchReq)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data:    result,
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// ExportSearchResults exports search results in various formats
func ExportSearchResults(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	// Parse search request
	var searchReq AdvancedSearchRequest
	if err := render.Bind(r, &searchReq); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Parse export format from query parameter
	formatStr := r.URL.Query().Get("format")
	if formatStr == "" {
		formatStr = "json"
	}

	exportReq := jira.ExportRequest{
		Format: jira.ExportFormat(formatStr),
		Fields: searchReq.Fields,
	}

	// Validate export request
	if err := jira.ValidateExportRequest(exportReq); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Perform the search
	jiraSearchReq := jira.SearchRequest{
		JQL:        searchReq.JQL,
		StartAt:    searchReq.StartAt,
		MaxResults: searchReq.MaxResults,
		Fields:     searchReq.Fields,
		Expand:     searchReq.Expand,
		Properties: searchReq.Properties,
	}

	searchResult, err := jiraClient.SearchIssuesAdvanced(jiraSearchReq)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Export the results
	exportResult, err := jiraClient.ExportSearchResults(searchResult, exportReq)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Set appropriate headers and return file
	w.Header().Set("Content-Type", exportResult.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", exportResult.Filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", exportResult.Size))

	w.WriteHeader(http.StatusOK)
	w.Write(exportResult.Data)
}

// GetSearchPage retrieves a specific page of search results
func GetSearchPage(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	var req AdvancedSearchRequest
	if err := render.Bind(r, &req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		if val, err := strconv.Atoi(pageStr); err == nil && val > 0 {
			page = val
		}
	}

	searchReq := jira.SearchRequest{
		JQL:        req.JQL,
		StartAt:    req.StartAt,
		MaxResults: req.MaxResults,
		Fields:     req.Fields,
		Expand:     req.Expand,
		Properties: req.Properties,
	}

	result, err := jiraClient.SearchPage(searchReq, page)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	response := &IssueResponse{
		Success: true,
		Data:    result,
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// GetAllSearchPages retrieves all pages of search results
func GetAllSearchPages(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	var req AdvancedSearchRequest
	if err := render.Bind(r, &req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	maxPagesStr := r.URL.Query().Get("maxPages")
	maxPages := 10 // Default limit
	if maxPagesStr != "" {
		if val, err := strconv.Atoi(maxPagesStr); err == nil && val > 0 && val <= 100 {
			maxPages = val
		}
	}

	searchReq := jira.SearchRequest{
		JQL:        req.JQL,
		StartAt:    req.StartAt,
		MaxResults: req.MaxResults,
		Fields:     req.Fields,
		Expand:     req.Expand,
		Properties: req.Properties,
	}

	allResults, err := jiraClient.SearchAllPages(searchReq, maxPages)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Combine all results
	combined := jira.CombineSearchResults(allResults)

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"totalPages":   len(allResults),
			"totalIssues":  len(combined.Issues),
			"searchTotal":  combined.Total,
			"issues":       combined.Issues,
			"warnings":     combined.WarningMessages,
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// Claude-optimized handlers

// NLRequest represents a natural language command request
type NLRequest struct {
	Command string `json:"command"`
}

func (req *NLRequest) Bind(r *http.Request) error {
	if req.Command == "" {
		return fmt.Errorf("command is required")
	}
	return nil
}

// JQLRequest represents a JQL generation request
type JQLRequest struct {
	Query string `json:"query"`
}

func (req *JQLRequest) Bind(r *http.Request) error {
	if req.Query == "" {
		return fmt.Errorf("query is required")
	}
	return nil
}

// ClaudeGetIssue returns a Claude-optimized issue response
func ClaudeGetIssue(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("jira client not initialized")))
		return
	}

	claudeManager := claude.GetManager()
	formatter := claudeManager.GetFormatter()

	issueKey := chi.URLParam(r, "key")
	if issueKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("issue key is required")))
		return
	}

	issue, err := jiraClient.GetIssue(context.Background(), issueKey, nil)
	if err != nil {
		response := formatter.FormatErrorResponse(err, "Get Issue")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response)
		return
	}

	response := formatter.FormatIssueResponse(issue, "get")
	render.Status(r, http.StatusOK)
	render.JSON(w, r, response)
}

// ClaudeSearchIssues returns Claude-optimized search results
func ClaudeSearchIssues(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("jira client not initialized")))
		return
	}

	claudeManager := claude.GetManager()
	formatter := claudeManager.GetFormatter()

	var req jira.SearchRequest
	if err := render.Bind(r, &req); err != nil {
		response := formatter.FormatErrorResponse(err, "Search Issues")
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response)
		return
	}

	result, err := jiraClient.SearchIssuesAdvanced(req)
	if err != nil {
		response := formatter.FormatErrorResponse(err, "Search Issues")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response)
		return
	}

	response := formatter.FormatSearchResponse(result, req.JQL)
	render.Status(r, http.StatusOK)
	render.JSON(w, r, response)
}

// ClaudeCreateIssue returns Claude-optimized issue creation response
func ClaudeCreateIssue(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("jira client not initialized")))
		return
	}

	claudeManager := claude.GetManager()
	formatter := claudeManager.GetFormatter()

	var req CreateIssueRequest
	if err := render.Bind(r, &req); err != nil {
		response := formatter.FormatErrorResponse(err, "Create Issue")
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response)
		return
	}

	fields := map[string]interface{}{
		"project": map[string]string{
			"key": req.Project,
		},
		"summary": req.Summary,
		"issuetype": map[string]string{
			"name": req.IssueType,
		},
	}

	if req.Description != "" {
		fields["description"] = req.Description
	}

	if req.Priority != "" {
		fields["priority"] = map[string]string{"name": req.Priority}
	}

	if req.Assignee != "" {
		fields["assignee"] = map[string]string{"name": req.Assignee}
	}

	if len(req.Labels) > 0 {
		fields["labels"] = req.Labels
	}

	if len(req.Components) > 0 {
		components := make([]map[string]string, len(req.Components))
		for i, compName := range req.Components {
			components[i] = map[string]string{"name": compName}
		}
		fields["components"] = components
	}

	createReq := &jira.CreateIssueRequest{
		Fields: fields,
	}

	issue, err := jiraClient.CreateIssue(context.Background(), createReq)
	if err != nil {
		response := formatter.FormatErrorResponse(err, "Create Issue")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response)
		return
	}

	response := formatter.FormatIssueResponse(issue, "create")
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, response)
}

// ProcessNaturalLanguageCommand processes natural language commands and returns Claude-optimized responses
func ProcessNaturalLanguageCommand(w http.ResponseWriter, r *http.Request) {
	claudeManager := claude.GetManager()
	formatter := claudeManager.GetFormatter()
	processor := claudeManager.GetProcessor()

	var req NLRequest
	if err := render.Bind(r, &req); err != nil {
		response := formatter.FormatErrorResponse(err, "Process Command")
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response)
		return
	}

	parsedCmd, err := processor.ParseCommand(req.Command)
	if err != nil {
		response := formatter.FormatErrorResponse(err, "Parse Command")
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response)
		return
	}

	// Validate command
	missingParams := processor.ValidateCommand(parsedCmd)
	if len(missingParams) > 0 {
		response := &claude.ClaudeResponse{
			Success: false,
			Summary: fmt.Sprintf("Missing required parameters: %s", strings.Join(missingParams, ", ")),
			Details: parsedCmd,
			Suggestions: []string{
				"Please provide all required parameters",
				"Try a more specific command",
			},
			Context: map[string]interface{}{
				"parsedCommand":    parsedCmd.Action,
				"missingParams":    missingParams,
				"originalCommand":  req.Command,
			},
		}
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response)
		return
	}

	response := formatter.FormatGenericResponse(
		parsedCmd,
		fmt.Sprintf("✅ Successfully parsed command: %s (confidence: %.2f)", parsedCmd.Action, parsedCmd.Confidence),
		"Parse Command",
	)

	// Add suggestions for next steps
	response.Suggestions = []string{
		"Execute the parsed command",
		"Refine the command if needed",
		"Ask for help with command syntax",
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, response)
}

// GenerateJQLFromNaturalLanguage converts natural language to JQL using Claude processing
func GenerateJQLFromNaturalLanguage(w http.ResponseWriter, r *http.Request) {
	claudeManager := claude.GetManager()
	formatter := claudeManager.GetFormatter()
	processor := claudeManager.GetProcessor()

	var req JQLRequest
	if err := render.Bind(r, &req); err != nil {
		response := formatter.FormatErrorResponse(err, "Generate JQL")
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response)
		return
	}

	jql, err := processor.GenerateJQLFromNaturalLanguage(req.Query)
	if err != nil {
		response := formatter.FormatErrorResponse(err, "Generate JQL")
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response)
		return
	}

	response := formatter.FormatGenericResponse(
		map[string]interface{}{
			"originalQuery": req.Query,
			"generatedJQL":  jql,
		},
		fmt.Sprintf("✅ Generated JQL: %s", jql),
		"Generate JQL",
	)

	response.Suggestions = []string{
		"Use this JQL to search for issues",
		"Refine the natural language query for better results",
		"Test the generated JQL",
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, response)
}

// GetCommandSuggestions provides command suggestions for partial input
func GetCommandSuggestions(w http.ResponseWriter, r *http.Request) {
	claudeManager := claude.GetManager()
	formatter := claudeManager.GetFormatter()
	processor := claudeManager.GetProcessor()

	partialInput := r.URL.Query().Get("input")
	if partialInput == "" {
		response := formatter.FormatErrorResponse(fmt.Errorf("input parameter is required"), "Get Suggestions")
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response)
		return
	}

	suggestions := processor.SuggestCommands(partialInput)

	response := formatter.FormatGenericResponse(
		map[string]interface{}{
			"partialInput": partialInput,
			"suggestions":  suggestions,
		},
		fmt.Sprintf("Found %d command suggestions", len(suggestions)),
		"Get Command Suggestions",
	)

	render.Status(r, http.StatusOK)
	render.JSON(w, r, response)
}