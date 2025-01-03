package models

import (
	"sync"
)

// ProvisionStep represents a single step in the provisioning process
type ProvisionStep struct {
	Name        string
	Description string
	Status      string
	Error       error
}

// ProvisionStepStatus represents the status of a provisioning step
type ProvisionStepStatus string

const (
	ProvisionStepStatusPending   ProvisionStepStatus = "PENDING"
	ProvisionStepStatusRunning   ProvisionStepStatus = "RUNNING"
	ProvisionStepStatusCompleted ProvisionStepStatus = "COMPLETED"
	ProvisionStepStatusFailed    ProvisionStepStatus = "FAILED"
)

// ProvisionProgress tracks the overall provisioning progress
type ProvisionProgress struct {
	CurrentStep    *ProvisionStep
	CompletedSteps []*ProvisionStep
	TotalSteps     int
	mutex          sync.Mutex
}

// NewProvisionProgress creates a new progress tracker
func NewProvisionProgress() *ProvisionProgress {
	return &ProvisionProgress{
		CompletedSteps: make([]*ProvisionStep, 0),
		TotalSteps:     1,
		mutex:          sync.Mutex{},
	}
}

// AddStep adds a completed step to the progress
func (p *ProvisionProgress) AddStep(step *ProvisionStep) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Prevent duplicates
	for _, s := range p.CompletedSteps {
		if s.Name == step.Name {
			return
		}
	}

	p.CompletedSteps = append(p.CompletedSteps, step)
}

// SetCurrentStep updates the current step being executed
func (p *ProvisionProgress) SetCurrentStep(step *ProvisionStep) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if step == nil {
		return
	}

	p.CurrentStep = step
}

func (p *ProvisionProgress) GetProgress() float64 {
	// Lock the mutex to ensure thread safety
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.TotalSteps == 0 {
		return 1
	}

	completed := len(p.CompletedSteps)
	if completed > p.TotalSteps {
		return 100 //nolint:mnd
	}

	return float64(completed) / float64(p.TotalSteps) * 100 //nolint:mnd
}
