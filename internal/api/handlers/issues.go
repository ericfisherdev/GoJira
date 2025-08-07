package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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