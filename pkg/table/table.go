package table

import (
	"io"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	NameWidth      = 60
	TypeWidth      = 5
	ProvStateWidth = 7 // 3 letters + 1 space + 1 emoji + 1 space
	LocationWidth  = 15
)

type ResourceTable struct {
	table *tablewriter.Table
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

func (rt *ResourceTable) AddResource(
	resource *armresources.GenericResource,
	provider models.ProviderAbbreviation,
) {
	provisioningState := models.StatusUnknown

	log := logger.Get()
	log.Debugf(
		"Adding resource: Name=%s, Type=%s, Provider=%s",
		*resource.Name,
		*resource.Type,
		provider,
	)
	log.Debugf("Resource Properties: %+v", resource.Properties)

	if resource.Properties != nil {
		log.Debugf("Resource Properties type: %T", resource.Properties)
		if props, ok := resource.Properties.(map[string]interface{}); ok {
			log.Debugf("Properties as map: %+v", props)
			if ps, ok := props["provisioningState"].(string); ok {
				provisioningState = models.GetStatusCode(models.StatusString(ps))
				log.Debugf("Provisioning State found: %s, StatusCode: %s", ps, provisioningState)
			} else {
				log.Debug("Provisioning State not found in properties")
			}
		} else {
			log.Debugf("Properties is not a map[string]interface{}, it's a %T", resource.Properties)
		}
	} else {
		log.Debug("Resource Properties is nil")
	}

	resourceType := abbreviateResourceType(*resource.Type)

	log.Debugf("Final values: ProvisioningState=%s, ResourceType=%s, ProviderAbbr=%s",
		provisioningState, resourceType, provider)

	row := []string{
		truncate(*resource.Name, NameWidth),
		truncate(resourceType, TypeWidth),
		truncate(string(provisioningState), ProvStateWidth),
		truncate(*resource.Location, LocationWidth),
		string(provider),
	}
	rt.table.Append(row)
}

func (rt *ResourceTable) Render() {
	rt.table.Render()
}

func abbreviateResourceType(resourceType string) string {
	parts := strings.Split(resourceType, "/")
	if len(parts) > 1 {
		lastPart := strings.ToLower(parts[len(parts)-1])
		switch {
		case strings.Contains(lastPart, "disk"):
			return "Disk "
		case strings.Contains(lastPart, "networkinterface"):
			return " NIC "
		case strings.Contains(lastPart, "virtualnetwork"):
			return "VNet "
		case strings.Contains(lastPart, "subnet"):
			return "SNet "
		case strings.Contains(lastPart, "virtualmachine"):
			return " VM  "
		case strings.Contains(lastPart, "publicipaddress"):
			return " IP  "
		case strings.Contains(lastPart, "loadbalancer"):
			return " LB  "
		case strings.Contains(lastPart, "storageaccount"):
			return "Filer"
		case strings.Contains(lastPart, "keyvault"):
			return "Vault"
		case strings.Contains(lastPart, "sqlserver"):
			return " SQL "
		case strings.Contains(lastPart, "webapp"):
			return " App "
		case strings.Contains(lastPart, "networksecuritygroup"):
			return " NSG "
		default:
			if len(lastPart) >= TypeWidth {
				return cases.Title(language.Und).String(lastPart[:TypeWidth])
			}
			return centerText(cases.Title(language.Und).String(lastPart), TypeWidth)
		}
	}
	return "Unk "
}

func centerText(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	leftPad := (width - len(s)) / 2 //nolint:gomnd
	rightPad := width - len(s) - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

// Remove the abbreviateProvider and formatTags functions as they are no longer needed

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	//nolint:gomnd
	if maxLen > 3 {
		return s[:maxLen-3] + "..."
	}
	return s[:maxLen]
}
