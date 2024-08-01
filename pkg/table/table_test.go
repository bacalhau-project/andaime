package table

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/stretchr/testify/assert"
)

type testWriter struct {
	bytes.Buffer
}

func (tw *testWriter) Close() error {
	return nil
}

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

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rt.Render()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "TestResource")
	assert.Contains(t, output, "Azure")
}

func TestRender(t *testing.T) {
	rt := NewResourceTable()

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rt.Render()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "Provider")
}

func stringPtr(s string) *string {
	return &s
}
