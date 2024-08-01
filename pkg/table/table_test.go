package table

import (
	"bytes"
	"os"
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

	// Capture table output
	buf := new(bytes.Buffer)
	rt.table.SetOutput(buf)

	rt.AddResource(resource, "Azure")
	rt.Render()

	output := buf.String()

	assert.Contains(t, output, "TestResource")
	assert.Contains(t, output, "virtualMachines")
	assert.Contains(t, output, "Succeeded")
	assert.Contains(t, output, "eastus")
	assert.Contains(t, output, "2023-05-01")
	assert.Contains(t, output, "/subscriptions/sub-id/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/TestResource")
	assert.Contains(t, output, "key1:value1, key2:value2")
	assert.Contains(t, output, "Azure")
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
