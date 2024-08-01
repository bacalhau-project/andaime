package table

import (
	"bytes"
	"strings"
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

	// Check if the resource was added correctly
	assert.Equal(t, 1, len(rt.table.GetRows()))
	row := rt.table.GetRows()[0]

	assert.Equal(t, "TestResource", row[0])
	assert.Equal(t, "virtualMachines", row[1])
	assert.Equal(t, "Succeeded", row[2])
	assert.Equal(t, "eastus", row[3])
	assert.Equal(t, "2023-05-01", row[4])
	assert.Contains(t, row[5], "/subscriptions/sub-id/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/TestResource")
	assert.Contains(t, row[6], "key1:value1")
	assert.Contains(t, row[6], "key2:value2")
	assert.Equal(t, "Azure", row[7])
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
	assert.True(t, strings.Contains(result, "key1:value1"))
	assert.True(t, strings.Contains(result, "key2:value2"))
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
