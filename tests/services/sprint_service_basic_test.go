package services

import (
	"testing"
	"time"

	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/ericfisherdev/GoJira/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSprintValidationBasic(t *testing.T) {
	// Test with nil client since we're only testing validation logic
	service := services.NewSprintService(nil)
	
	t.Run("Valid sprint request", func(t *testing.T) {
		req := &jira.CreateSprintRequest{
			Name:          "Sprint 1",
			Goal:          "Complete user stories",
			OriginBoardID: 1,
		}
		
		err := service.ValidateSprint(req)
		assert.NoError(t, err)
	})
	
	t.Run("Missing name", func(t *testing.T) {
		req := &jira.CreateSprintRequest{
			OriginBoardID: 1,
		}
		
		err := service.ValidateSprint(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})
	
	t.Run("Missing board ID", func(t *testing.T) {
		req := &jira.CreateSprintRequest{
			Name: "Sprint 1",
		}
		
		err := service.ValidateSprint(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "board ID is required")
	})
	
	t.Run("Name too long", func(t *testing.T) {
		longName := make([]byte, 300)
		for i := range longName {
			longName[i] = 'a'
		}
		
		req := &jira.CreateSprintRequest{
			Name:          string(longName),
			OriginBoardID: 1,
		}
		
		err := service.ValidateSprint(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum length")
	})
	
	t.Run("Invalid date order", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, 7)
		endDate := time.Now()
		
		req := &jira.CreateSprintRequest{
			Name:          "Sprint 1",
			OriginBoardID: 1,
			StartDate:     &startDate,
			EndDate:       &endDate,
		}
		
		err := service.ValidateSprint(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "start date must be before end date")
	})
	
	t.Run("Sprint too short", func(t *testing.T) {
		startDate := time.Now()
		endDate := startDate.AddDate(0, 0, 3) // 3 days - too short
		
		req := &jira.CreateSprintRequest{
			Name:          "Sprint 1",
			OriginBoardID: 1,
			StartDate:     &startDate,
			EndDate:       &endDate,
		}
		
		err := service.ValidateSprint(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "sprint duration must be at least")
	})
	
	t.Run("Sprint too long", func(t *testing.T) {
		startDate := time.Now()
		endDate := startDate.AddDate(0, 2, 0) // 2 months - too long
		
		req := &jira.CreateSprintRequest{
			Name:          "Sprint 1",
			OriginBoardID: 1,
			StartDate:     &startDate,
			EndDate:       &endDate,
		}
		
		err := service.ValidateSprint(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "sprint duration cannot exceed")
	})
	
	t.Run("Valid sprint with dates", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, 1)
		endDate := startDate.AddDate(0, 0, 14) // 2 weeks
		
		req := &jira.CreateSprintRequest{
			Name:          "Sprint 1",
			Goal:          "Complete features",
			OriginBoardID: 1,
			StartDate:     &startDate,
			EndDate:       &endDate,
		}
		
		err := service.ValidateSprint(req)
		assert.NoError(t, err)
	})
}

func TestDefaultSprintValidation(t *testing.T) {
	validation := services.DefaultSprintValidation()
	
	assert.NotNil(t, validation)
	assert.Equal(t, 7*24*time.Hour, validation.MinDuration)
	assert.Equal(t, 30*24*time.Hour, validation.MaxDuration)
	assert.Equal(t, 255, validation.MaxNameLength)
	assert.Equal(t, 1000, validation.MaxGoalLength)
}