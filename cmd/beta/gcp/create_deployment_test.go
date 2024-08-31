package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateDeploymentCmd(t *testing.T) {
	cmd := createDeploymentCmd()
	assert.NotNil(t, cmd)
	assert.Equal(t, "create-deployment", cmd.Use)
	assert.Equal(t, "Create a new deployment in GCP", cmd.Short)
}