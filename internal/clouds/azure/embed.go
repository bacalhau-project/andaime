package internal_azure

import (
	"embed"
)

//go:embed azure_data.yaml
var azureData embed.FS

//go:embed arm/vm.json
var vmARM embed.FS

func GetARMTemplate() ([]byte, error) {
	data, err := vmARM.ReadFile("arm/vm.json")
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetAzureData() ([]byte, error) {
	data, err := azureData.ReadFile("azure_data.yaml")
	if err != nil {
		return nil, err
	}
	return data, nil
}
