package table

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/olekukonko/tablewriter"
)

type ResourceTable struct {
	table *tablewriter.Table
}

func NewResourceTable() *ResourceTable {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Type", "Prov State", "Location", "Created", "ID", "Tags"})
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

func (rt *ResourceTable) AddResource(resource armresources.GenericResource) {
	provisioningState := "Unknown"
	createdTime := "Unknown"

	if resource.Properties != nil {
		props := resource.Properties.(map[string]interface{})
		if ps, ok := props["provisioningState"].(string); ok {
			provisioningState = ps
		}
		if ct, ok := props["creationTime"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ct); err == nil {
				createdTime = t.Format("2006-01-02")
			}
		}
	}

	resourceType := shortenResourceType(*resource.Type)
	tags := formatTags(resource.Tags)

	row := []string{
		truncate(*resource.Name, 20),
		truncate(resourceType, 15),
		truncate(provisioningState, 10),
		truncate(*resource.Location, 10),
		truncate(createdTime, 10),
		truncate(*resource.ID, 20),
		truncate(tags, 30),
	}
	rt.table.Append(row)
}

func (rt *ResourceTable) Render() {
	rt.table.Render()
}

// Helper functions remain the same
func shortenResourceType(resourceType string) string {
	parts := strings.Split(resourceType, "/")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return resourceType
}

func formatTags(tags map[string]*string) string {
	var tagStrings []string
	for k, v := range tags {
		if v != nil {
			tagStrings = append(tagStrings, fmt.Sprintf("%s:%s", k, *v))
		}
	}
	return strings.Join(tagStrings, ", ")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
