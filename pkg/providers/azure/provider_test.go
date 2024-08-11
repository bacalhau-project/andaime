package azure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNetworkProperties(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]interface{}
		propType   string
		expected   map[string]interface{}
	}{
		{
			name: "NSG properties",
			properties: map[string]interface{}{
				"provisioningState": "Succeeded",
				"securityRules": []interface{}{
					map[string]interface{}{
						"name": "AllowSSH",
						"properties": map[string]interface{}{
							"protocol":                 "TCP",
							"sourcePortRange":          "*",
							"destinationPortRange":     "22",
							"sourceAddressPrefix":      "*",
							"destinationAddressPrefix": "*",
							"access":                   "Allow",
							"priority":                 float64(1000),
							"direction":                "Inbound",
						},
					},
				},
			},
			propType: "NSG",
			expected: map[string]interface{}{
				"provisioningState": "Succeeded",
				"securityRules": []*armnetwork.SecurityRule{
					{
						Name: utils.ToPtr("AllowSSH"),
						Properties: &armnetwork.SecurityRulePropertiesFormat{
							Protocol:                 (*armnetwork.SecurityRuleProtocol)(utils.ToPtr("TCP")),
							SourcePortRange:          utils.ToPtr("*"),
							DestinationPortRange:     utils.ToPtr("22"),
							SourceAddressPrefix:      utils.ToPtr("*"),
							DestinationAddressPrefix: utils.ToPtr("*"),
							Access:                   (*armnetwork.SecurityRuleAccess)(utils.ToPtr("Allow")),
							Priority:                 utils.ToPtr(int32(1000)),
							Direction:                (*armnetwork.SecurityRuleDirection)(utils.ToPtr("Inbound")),
						},
					},
				},
			},
		},
		{
			name: "PIP properties",
			properties: map[string]interface{}{
				"provisioningState": "Succeeded",
				"ipAddress":         "10.0.0.1",
			},
			propType: "PIP",
			expected: map[string]interface{}{
				"provisioningState": "Succeeded",
				"ipAddress":         "10.0.0.1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNetworkProperties(tt.properties, tt.propType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseSecurityRules(t *testing.T) {
	rules := []interface{}{
		map[string]interface{}{
			"name": "AllowHTTP",
			"properties": map[string]interface{}{
				"protocol":                 "TCP",
				"sourcePortRange":          "*",
				"destinationPortRange":     "80",
				"sourceAddressPrefix":      "*",
				"destinationAddressPrefix": "*",
				"access":                   "Allow",
				"priority":                 float64(1001),
				"direction":                "Inbound",
			},
		},
	}

	expected := []*armnetwork.SecurityRule{
		{
			Name: utils.ToPtr("AllowHTTP"),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Protocol:                 (*armnetwork.SecurityRuleProtocol)(utils.ToPtr("TCP")),
				SourcePortRange:          utils.ToPtr("*"),
				DestinationPortRange:     utils.ToPtr("80"),
				SourceAddressPrefix:      utils.ToPtr("*"),
				DestinationAddressPrefix: utils.ToPtr("*"),
				Access:                   (*armnetwork.SecurityRuleAccess)(utils.ToPtr("Allow")),
				Priority:                 utils.ToPtr(int32(1001)),
				Direction:                (*armnetwork.SecurityRuleDirection)(utils.ToPtr("Inbound")),
			},
		},
	}

	result := parseSecurityRules(rules)
	assert.Equal(t, expected, result)
}

func TestParseIPConfigurations(t *testing.T) {
	configs := []interface{}{
		map[string]interface{}{
			"name": "ipconfig1",
			"properties": map[string]interface{}{
				"privateIPAddress": "192.168.0.4",
			},
		},
	}

	expected := []*armnetwork.InterfaceIPConfiguration{
		{
			Name: utils.ToPtr("ipconfig1"),
			Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
				PrivateIPAddress: utils.ToPtr("192.168.0.4"),
			},
		},
	}

	result := parseIPConfigurations(configs)
	assert.Equal(t, expected, result)
}

func TestParseStringSlice(t *testing.T) {
	slice := []interface{}{"10.0.0.0/16", "192.168.0.0/24"}
	expected := []*string{utils.ToPtr("10.0.0.0/16"), utils.ToPtr("192.168.0.0/24")}

	result := parseStringSlice(slice)
	assert.Equal(t, expected, result)
}
