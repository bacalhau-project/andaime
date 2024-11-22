package provision_test

import (
	"context"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/cmd/beta/provision"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestProvisionerSimulation(t *testing.T) {
	config := &provision.NodeConfig{
		IPAddress:  "192.168.1.100",
		Username:   "test-user",
		PrivateKey: "/path/to/key",
	}

	p, err := provision.NewProvisioner(config)
	if err != nil {
		t.Fatalf("Failed to create provisioner: %v", err)
	}

	updates := make(chan *models.DisplayStatus)
	done := make(chan bool)

	// Start a goroutine to collect and verify updates
	go func() {
		expectedSteps := []string{
			"🚀 Starting node provisioning process",
			"📡 Establishing SSH connection...",
			"✅ SSH connection established successfully",
			"🏠 Provisioning base packages...",
			"✅ Base packages provisioned successfully",
			"🍽️ Setting up node configuration...",
			"✅ Node configuration completed",
			"📦 Installing Bacalhau...",
			"✅ Bacalhau binary installed successfully",
			"📝 Installing Bacalhau service script...",
			"✅ Bacalhau service script installed",
			"🔧 Setting up Bacalhau systemd service...",
			"✅ Bacalhau systemd service installed and started",
			"🔍 Verifying Bacalhau node is running...",
			"✅ Bacalhau node verified and running",
			"✅ Successfully provisioned node on 192.168.1.100",
		}

		stepIndex := 0
		for status := range updates {
			if stepIndex < len(expectedSteps) {
				assert.Contains(t, status.StatusMessage, expectedSteps[stepIndex])
				stepIndex++
			}
			if status.Progress == 100 {
				close(done)
				return
			}
		}
	}()

	ctx := context.Background()
	err = p.ProvisionWithCallback(ctx, func(status *models.DisplayStatus) {
		updates <- status
		// Add small delay to simulate real deployment timing
		time.Sleep(500 * time.Millisecond)
	})

	assert.NoError(t, err)
	<-done
}
