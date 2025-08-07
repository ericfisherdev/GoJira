package jira

import (
	"fmt"
	"math"
)

// PaginationInfo represents pagination metadata
type PaginationInfo struct {
	StartAt      int  `json:"startAt"`
	MaxResults   int  `json:"maxResults"`
	Total        int  `json:"total"`
	CurrentPage  int  `json:"currentPage"`
	TotalPages   int  `json:"totalPages"`
	HasNextPage  bool `json:"hasNextPage"`
	HasPrevPage  bool `json:"hasPrevPage"`
	NextStartAt  int  `json:"nextStartAt,omitempty"`
	PrevStartAt  int  `json:"prevStartAt,omitempty"`
}

// PaginatedSearchResult extends search results with pagination info
type PaginatedSearchResult struct {
	*ExtendedSearchResult
	Pagination PaginationInfo `json:"pagination"`
}

// SearchWithPagination performs a search with enhanced pagination information
func (c *Client) SearchWithPagination(req SearchRequest) (*PaginatedSearchResult, error) {
	result, err := c.SearchIssuesAdvanced(req)
	if err != nil {
		return nil, err
	}

	pagination := calculatePaginationInfo(result.StartAt, result.MaxResults, result.Total)

	return &PaginatedSearchResult{
		ExtendedSearchResult: result,
		Pagination:          pagination,
	}, nil
}

// SearchAllPages retrieves all pages of search results
func (c *Client) SearchAllPages(req SearchRequest, maxPages int) ([]*ExtendedSearchResult, error) {
	var allResults []*ExtendedSearchResult
	
	// Set reasonable page size if not specified
	if req.MaxResults <= 0 {
		req.MaxResults = 50
	}
	
	// Limit max pages to prevent runaway queries
	if maxPages <= 0 {
		maxPages = 100 // Default max pages
	}

	currentStartAt := req.StartAt
	pageCount := 0

	for pageCount < maxPages {
		req.StartAt = currentStartAt
		
		result, err := c.SearchIssuesAdvanced(req)
		if err != nil {
			return allResults, fmt.Errorf("failed to fetch page %d: %w", pageCount+1, err)
		}

		allResults = append(allResults, result)
		pageCount++

		// Check if we have more results
		if result.StartAt+result.MaxResults >= result.Total {
			break // No more pages
		}

		currentStartAt += req.MaxResults
	}

	return allResults, nil
}

// GetNextPage retrieves the next page of results based on current result
func (c *Client) GetNextPage(currentResult *ExtendedSearchResult, originalRequest SearchRequest) (*PaginatedSearchResult, error) {
	if !hasNextPage(currentResult.StartAt, currentResult.MaxResults, currentResult.Total) {
		return nil, fmt.Errorf("no next page available")
	}

	nextRequest := originalRequest
	nextRequest.StartAt = currentResult.StartAt + currentResult.MaxResults
	nextRequest.MaxResults = currentResult.MaxResults

	return c.SearchWithPagination(nextRequest)
}

// GetPreviousPage retrieves the previous page of results
func (c *Client) GetPreviousPage(currentResult *ExtendedSearchResult, originalRequest SearchRequest) (*PaginatedSearchResult, error) {
	if !hasPrevPage(currentResult.StartAt, currentResult.MaxResults) {
		return nil, fmt.Errorf("no previous page available")
	}

	prevRequest := originalRequest
	prevRequest.StartAt = max(0, currentResult.StartAt-currentResult.MaxResults)
	prevRequest.MaxResults = currentResult.MaxResults

	return c.SearchWithPagination(prevRequest)
}

// SearchPage retrieves a specific page number
func (c *Client) SearchPage(req SearchRequest, pageNumber int) (*PaginatedSearchResult, error) {
	if pageNumber < 1 {
		return nil, fmt.Errorf("page number must be >= 1")
	}

	if req.MaxResults <= 0 {
		req.MaxResults = 50 // Default page size
	}

	req.StartAt = (pageNumber - 1) * req.MaxResults

	return c.SearchWithPagination(req)
}

// CombineSearchResults merges multiple search results into one
func CombineSearchResults(results []*ExtendedSearchResult) *ExtendedSearchResult {
	if len(results) == 0 {
		return &ExtendedSearchResult{
			Issues: []Issue{},
		}
	}

	combined := &ExtendedSearchResult{
		StartAt:    results[0].StartAt,
		MaxResults: 0,
		Total:      0,
		Issues:     []Issue{},
	}

	var allWarnings []string

	for _, result := range results {
		combined.Issues = append(combined.Issues, result.Issues...)
		combined.MaxResults += result.MaxResults
		
		// Use the highest total count found
		if result.Total > combined.Total {
			combined.Total = result.Total
		}

		// Collect all warning messages
		if len(result.WarningMessages) > 0 {
			allWarnings = append(allWarnings, result.WarningMessages...)
		}
	}

	combined.WarningMessages = allWarnings
	return combined
}

// calculatePaginationInfo computes pagination metadata
func calculatePaginationInfo(startAt, maxResults, total int) PaginationInfo {
	currentPage := (startAt / maxResults) + 1
	totalPages := int(math.Ceil(float64(total) / float64(maxResults)))
	
	hasNext := hasNextPage(startAt, maxResults, total)
	hasPrev := hasPrevPage(startAt, maxResults)

	info := PaginationInfo{
		StartAt:     startAt,
		MaxResults:  maxResults,
		Total:       total,
		CurrentPage: currentPage,
		TotalPages:  totalPages,
		HasNextPage: hasNext,
		HasPrevPage: hasPrev,
	}

	if hasNext {
		info.NextStartAt = startAt + maxResults
	}

	if hasPrev {
		info.PrevStartAt = max(0, startAt-maxResults)
	}

	return info
}

// hasNextPage checks if there are more results available
func hasNextPage(startAt, maxResults, total int) bool {
	return startAt+maxResults < total
}

// hasPrevPage checks if there are previous results available
func hasPrevPage(startAt, maxResults int) bool {
	return startAt > 0
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}