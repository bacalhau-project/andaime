package azure

import (
	"reflect"
	"testing"
)

func TestStripAndParseJSON(t *testing.T) {
	input := `01:22:21.385 | INF pkg/repo/fs.go:99 > Initializing repo at '/home/azureuser/.bacalhau' for environment 'production'
[{"Info":{"NodeID":"n-d620a0f0-0ee8-4d13-9120-6cf2a6898c80","NodeType":"Requester","Labels":{"Architecture":"amd64","DISK_GB":"29","HOSTNAME":"uu5tba-vm","IP":"13.89.100.89","LOCATION":"centralus","MACHINE_TYPE":"Standard_DS1_v2","MEMORY_GB":"3.3","NODE_TYPE":"requester","ORCHESTRATORS":"0.0.0.0","Operating-System":"linux","VCPU_COUNT":"1"},"BacalhauVersion":{"Major":"1","Minor":"4","GitVersion":"v1.4.0","GitCommit":"081eabfba0d723fbd3889d8e4e59c1ffc126ad0f","BuildDate":"2024-06-28T10:14:58Z","GOOS":"linux","GOARCH":"amd64"}},"Membership":"APPROVED","Connection":"DISCONNECTED"}]`

	expected := []map[string]interface{}{
		{
			"Info": map[string]interface{}{
				"NodeID":   "n-d620a0f0-0ee8-4d13-9120-6cf2a6898c80",
				"NodeType": "Requester",
				"Labels": map[string]interface{}{
					"Architecture":     "amd64",
					"DISK_GB":          "29",
					"HOSTNAME":         "uu5tba-vm",
					"IP":               "13.89.100.89",
					"LOCATION":         "centralus",
					"MACHINE_TYPE":     "Standard_DS1_v2",
					"MEMORY_GB":        "3.3",
					"NODE_TYPE":        "requester",
					"ORCHESTRATORS":    "0.0.0.0",
					"Operating-System": "linux",
					"VCPU_COUNT":       "1",
				},
				"BacalhauVersion": map[string]interface{}{
					"Major":      "1",
					"Minor":      "4",
					"GitVersion": "v1.4.0",
					"GitCommit":  "081eabfba0d723fbd3889d8e4e59c1ffc126ad0f",
					"BuildDate":  "2024-06-28T10:14:58Z",
					"GOOS":       "linux",
					"GOARCH":     "amd64",
				},
			},
			"Membership": "APPROVED",
			"Connection": "DISCONNECTED",
		},
	}

	result, err := stripAndParseJSON(input)
	if err != nil {
		t.Fatalf("Error parsing JSON: %v", err)
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf(
			"Parsed JSON does not match expected output.\nGot: %+v\nWant: %+v",
			result,
			expected,
		)
	}
}
