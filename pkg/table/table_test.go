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
	provisioningState := "Succeeded"
	createdTime := "2023-05-01T12:00:00Z"

	resource := armresources.GenericResource{
		Name:     &name,
		Type:     &resourceType,
		Location: &location,
		ID:       &id,
		Properties: map[string]interface{}{
			"provisioningState": provisioningState,
			"creationTime":      createdTime,
		},
		Tags: map[string]*string{
			"key1": stringPtr("value1"),
			"key2": stringPtr("value2"),
		},
	}

	rt.AddResource(resource, "Azure")

	var buf bytes.Buffer
	rt.table.SetOutput(&buf)
	rt.Render()

	output := buf.String()
	assert.Contains(t, output, "TestResource")
	assert.Contains(t, output, "virtualMachines")
	assert.Contains(t, output, "Succeeded")
	assert.Contains(t, output, "eastus")
	assert.Contains(t, output, "2023-05-01")
	assert.Contains(t, output, "/subscriptions/sub-...")
	assert.Contains(t, output, "key1:value1, key2:v...")
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
	assert.Contains(t, output, "Type")
	assert.Contains(t, output, "Prov State")
	assert.Contains(t, output, "Location")
	assert.Contains(t, output, "Created")
	assert.Contains(t, output, "ID")
	assert.Contains(t, output, "Tags")
	assert.Contains(t, output, "Provider")
}

func TestShortenResourceType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Microsoft.Compute/virtualMachines", "virtualMachines"},
		{"Microsoft.Network/virtualNetworks", "virtualNetworks"},
		{"SimpleType", "SimpleType"},
	}

	for _, test := range tests {
		result := shortenResourceType(test.input)
		assert.Equal(t, test.expected, result)
	}
}

func TestFormatTags(t *testing.T) {
	tags := map[string]*string{
		"key1": stringPtr("value1"),
		"key2": stringPtr("value2"),
	}

	result := formatTags(tags)
	assert.Contains(t, result, "key1:value1")
	assert.Contains(t, result, "key2:value2")
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"Short", 10, "Short"},
		{"LongStringToTruncate", 10, "LongStr..."},
		{"ExactlyTen", 10, "ExactlyTen"},
	}

	for _, test := range tests {
		result := truncate(test.input, test.maxLen)
		assert.Equal(t, test.expected, result)
	}
}

func stringPtr(s string) *string {
	return &s
}
