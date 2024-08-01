package table

import (
	"io"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/olekukonko/tablewriter"
)

const (
	NameWidth      = 60
	TypeWidth      = 4
	ProvStateWidth = 7 // 3 letters + 1 space + 1 emoji + 1 space
	LocationWidth  = 15
)

type ResourceTable struct {
	table  *tablewriter.Table
	writer io.Writer
}

func NewResourceTable(w io.Writer) *ResourceTable {
	if w == nil {
		w = os.Stdout
	}
	table := tablewriter.NewWriter(w)
	table.SetHeader(
		[]string{"Name", "Type", "State", "Location", "Prov"},
	)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)

	return &ResourceTable{table: table}
}

func (rt *ResourceTable) AddResource(resource armresources.GenericResource, provider string) {
	provisioningState := "UNK"
	stateEmoji := "❓"

	if resource.Properties != nil {
		props := resource.Properties.(map[string]interface{})
		if ps, ok := props["provisioningState"].(string); ok {
			provisioningState = abbreviateProvisioningState(ps)
			stateEmoji = getStateEmoji(ps)
		}
	}

	resourceType := abbreviateResourceType(*resource.Type)

	row := []string{
		truncate(*resource.Name, NameWidth),
		truncate(resourceType, TypeWidth),
		truncate(provisioningState+" "+stateEmoji, ProvStateWidth),
		truncate(*resource.Location, LocationWidth),
		provider,
	}
	rt.table.Append(row)
}

func (rt *ResourceTable) Render() {
	rt.table.Render()
}

// Helper functions
func abbreviateResourceType(resourceType string) string {
	parts := strings.Split(resourceType, "/")
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		if len(lastPart) >= 3 {
			return strings.ToUpper(lastPart[:3])
		}
		return strings.ToUpper(lastPart)
	}
	return "UNK"
}

func abbreviateProvisioningState(state string) string {
	switch strings.ToLower(state) {
	case "succeeded":
		return "SUC"
	case "failed":
		return "FAI"
	case "creating":
		return "CRE"
	case "updating":
		return "UPD"
	case "deleting":
		return "DEL"
	default:
		return "UNK"
	}
}

func getStateEmoji(state string) string {
	switch strings.ToLower(state) {
	case "succeeded":
		return "✅"
	case "failed":
		return "❌"
	case "creating", "updating":
		return "🔄"
	case "deleting":
		return "🗑️"
	default:
		return "❓"
	}
}

// Remove the abbreviateProvider and formatTags functions as they are no longer needed

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen > 3 {
		return s[:maxLen-3] + "..."
	}
	return s[:maxLen]
}
