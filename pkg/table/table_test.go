package table

import (
	"bytes"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/stretchr/testify/assert"
)

func TestNewResourceTable(t *testing.T) {
	var buf bytes.Buffer
	rt := NewResourceTable(&buf)
	assert.NotNil(t, rt)
	assert.NotNil(t, rt.table)
}

func TestAddResource(t *testing.T) {
	var buf bytes.Buffer
	rt := NewResourceTable(&buf)

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
	rt.Render()

	output := buf.String()
	assert.Contains(t, output, "TestResource")
	assert.Contains(t, output, "Azure")
	assert.Contains(t, output, "VIR")
	assert.Contains(t, output, "UNK")
	assert.Contains(t, output, "eastus")
}

func TestRender(t *testing.T) {
	var buf bytes.Buffer
	rt := NewResourceTable(&buf)

	rt.Render()

	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "PROV")
}

func stringPtr(s string) *string {
	return &s
}
