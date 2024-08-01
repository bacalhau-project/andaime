package table

import (
	"bytes"
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

	var buf bytes.Buffer
	rt.table.SetOutput(&buf)
	rt.Render()

	output := buf.String()
	assert.Contains(t, output, "TestResource")
	assert.Contains(t, output, "Azure")
}

func TestRender(t *testing.T) {
	rt := NewResourceTable()

	var buf bytes.Buffer
	rt.table.SetOutput(&buf)

	rt.Render()

	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "Provider")
}

func stringPtr(s string) *string {
	return &s
}
