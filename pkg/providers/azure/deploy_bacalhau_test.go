package azure

import (
	"context"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/stretchr/testify/assert"
)

// mockGetGlobalModel replaces the global GetGlobalModel function with a mock
func mockGetGlobalModel(mockModel *display.DisplayModel) func() {
	original := display.GetGlobalModel
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return mockModel
	}
	return func() {
		display.GetGlobalModelFunc = original
	}
}

func TestDeployBacalhauOrchestrator(t *testing.T) {
	tests := []struct {
		name          string
		machines      []models.Machine
		expectError   bool
		errorContains string
	}{
		{
			name:          "No orchestrator node and no orchestrator IP",
			machines:      []models.Machine{},
			expectError:   true,
			errorContains: "no orchestrator node found",
		},
		{
			name: "No orchestrator node, but orchestrator IP specified",
			machines: []models.Machine{
				{OrchestratorIP: "NOT_EMPTY"},
			},
			expectError:   true,
			errorContains: "orchestrator IP set",
		},
		{
			name: "One orchestrator node, no other machines",
			machines: []models.Machine{
				{Orchestrator: true},
			},
			expectError: false,
		},
		{
			name: "One orchestrator node and many other machines",
			machines: []models.Machine{
				{Orchestrator: true},
				{},
				{},
			},
			expectError: false,
		},
		{
			name: "Multiple orchestrator nodes (should error)",
			machines: []models.Machine{
				{Orchestrator: true},
				{Orchestrator: true},
			},
			expectError:   true,
			errorContains: "multiple orchestrator nodes found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock model with the test machines
			mockModel := &display.DisplayModel{
				Deployment: &models.Deployment{
					Machines: tt.machines,
				},
			}

			// Replace the global GetGlobalModel with our mock
			cleanup := mockGetGlobalModel(mockModel)
			defer cleanup()

			// Create an AzureProvider instance
			provider := &AzureProvider{}

			// Call the method
			err := provider.DeployBacalhauOrchestrator(context.Background())

			// Check the result
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
