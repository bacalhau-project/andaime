package table

import (
	"bytes"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/olekukonko/tablewriter"
	"github.com/stretchr/testify/assert"
)

// MockTable is a mock implementation of the tablewriter.Table
type MockTable struct {
	mock.Mock
}

func (m *MockTable) Append(row []string)                    { m.Called(row) }
func (m *MockTable) SetHeader(headers []string)             { m.Called(headers) }
func (m *MockTable) SetAutoWrapText(wrap bool)              { m.Called(wrap) }
func (m *MockTable) SetAutoFormatHeaders(format bool)       { m.Called(format) }
func (m *MockTable) SetHeaderAlignment(alignment int)       { m.Called(alignment) }
func (m *MockTable) SetAlignment(alignment int)             { m.Called(alignment) }
func (m *MockTable) SetCenterSeparator(sep string)          { m.Called(sep) }
func (m *MockTable) SetColumnSeparator(sep string)          { m.Called(sep) }
func (m *MockTable) SetRowSeparator(sep string)             { m.Called(sep) }
func (m *MockTable) SetHeaderLine(line bool)                { m.Called(line) }
func (m *MockTable) SetBorder(border bool)                  { m.Called(border) }
func (m *MockTable) SetTablePadding(padding string)         { m.Called(padding) }
func (m *MockTable) SetNoWhiteSpace(noWhiteSpace bool)      { m.Called(noWhiteSpace) }
func (m *MockTable) Render()                                { m.Called() }

func TestNewResourceTable(t *testing.T) {
	mockTable := new(MockTable)
	
	mockTable.On("SetHeader", mock.Anything).Return()
	mockTable.On("SetAutoWrapText", false).Return()
	mockTable.On("SetAutoFormatHeaders", true).Return()
	mockTable.On("SetHeaderAlignment", tablewriter.ALIGN_LEFT).Return()
	mockTable.On("SetAlignment", tablewriter.ALIGN_LEFT).Return()
	mockTable.On("SetCenterSeparator", "").Return()
	mockTable.On("SetColumnSeparator", "").Return()
	mockTable.On("SetRowSeparator", "").Return()
	mockTable.On("SetHeaderLine", false).Return()
	mockTable.On("SetBorder", false).Return()
	mockTable.On("SetTablePadding", "\t").Return()
	mockTable.On("SetNoWhiteSpace", true).Return()

	rt := NewResourceTable()
	assert.NotNil(t, rt)
	assert.NotNil(t, rt.table)

	// We can't directly compare the mock table with rt.table
	// because rt.table is of type *tablewriter.Table
	// Instead, we'll check if the mock expectations were met
	mockTable.AssertExpectations(t)
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

	expectedRow := []string{
		"TestResource",
		"virtualMachines",
		"Succeeded",
		"eastus",
		"2023-05-01",
		"/subscriptions/sub-...",
		"key1:value1, key2:v...",
		"Azure",
	}

	mockTable.On("Append", expectedRow).Return()

	rt.AddResource(resource, "Azure")

	mockTable.AssertExpectations(t)
}

func TestRender(t *testing.T) {
	mockTable := new(MockTable)
	rt := &ResourceTable{table: mockTable}

	mockTable.On("Render").Return()

	rt.Render()

	mockTable.AssertExpectations(t)
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
