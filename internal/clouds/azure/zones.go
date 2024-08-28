package azure

import (
	"fmt"
	"strings"
)

var validAzureZones = []string{
	"westus", "eastus", "northeurope", "westeurope", "eastasia", "southeastasia",
	"northcentralus", "southcentralus", "centralus", "eastus2", "japaneast",
	"japanwest", "brazilsouth", "australiaeast", "australiasoutheast", "centralindia",
	"southindia", "westindia", "canadacentral", "canadaeast", "westcentralus",
	"westus2", "ukwest", "uksouth", "koreacentral", "koreasouth", "francecentral",
	"australiacentral", "southafricanorth", "uaenorth", "switzerlandnorth",
	"germanywestcentral", "norwayeast", "westus3", "jioindiawest", "swedencentral",
	"qatarcentral", "polandcentral", "italynorth", "israelcentral", "mexicocentral",
	"spaincentral",
}

func IsValidAzureZone(zone string) bool {
	for _, validZone := range validAzureZones {
		if strings.EqualFold(zone, validZone) {
			return true
		}
	}
	return false
}

func SetMachineZone(machine *Machine, zone string) error {
	if !IsValidAzureZone(zone) {
		return fmt.Errorf("invalid Azure zone: %s", zone)
	}
	machine.Zone = zone
	return nil
}

type Machine struct {
	Zone string
}
