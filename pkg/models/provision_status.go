package models

// ProvisionStep represents a single step in the provisioning process
type ProvisionStep struct {
	Name        string
	Description string
	Status      string
	Error       error
}

// ProvisionProgress tracks the overall provisioning progress
type ProvisionProgress struct {
	CurrentStep  *ProvisionStep
	CompletedSteps []*ProvisionStep
	TotalSteps     int
}

// NewProvisionProgress creates a new progress tracker
func NewProvisionProgress() *ProvisionProgress {
	return &ProvisionProgress{
		CompletedSteps: make([]*ProvisionStep, 0),
		TotalSteps:     6, // Based on our SOP
	}
}

// AddStep adds a completed step to the progress
func (p *ProvisionProgress) AddStep(step *ProvisionStep) {
	p.CompletedSteps = append(p.CompletedSteps, step)
}

// SetCurrentStep updates the current step being executed
func (p *ProvisionProgress) SetCurrentStep(step *ProvisionStep) {
	p.CurrentStep = step
}

// GetProgress returns the current progress as a percentage
func (p *ProvisionProgress) GetProgress() float64 {
	return float64(len(p.CompletedSteps)) / float64(p.TotalSteps) * 100
}
