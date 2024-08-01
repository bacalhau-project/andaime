package table

import (
	"io"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/stretchr/testify/assert"
)

func TestNewResourceTable(t *testing.T) {
	rt := NewResourceTable()
	assert.NotNil(t, rt)
	assert.NotNil(t, rt.table)
}

func TestAddResource(t *testing.T) {
	rt := NewResourceTable()

	name := "TestResource"
	resourceType := "Microsoft.Compute/virtualMachines"
	location := "eastus"
	id := "/subscriptions/sub-id/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/TestResource"

	resource := armresources.GenericResource{
		Name:     &name,
		Type:     &resourceType,
		Location: &location,
		ID:       &id,
	}

	rt.AddResource(resource, "Azure")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rt.Render()

	w.Close()
	os.Stdout = oldStdout

	output, _ := io.ReadAll(r)
	assert.Contains(t, string(output), "TestResource")
	assert.Contains(t, string(output), "Azure")
}

func TestRender(t *testing.T) {
	rt := NewResourceTable()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rt.Render()

	w.Close()
	os.Stdout = oldStdout

	output, _ := io.ReadAll(r)
	assert.NotEmpty(t, string(output))
	assert.Contains(t, string(output), "Name")
	assert.Contains(t, string(output), "Provider")
}

func stringPtr(s string) *string {
	return &s
}
