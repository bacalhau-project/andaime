package utils

import (
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

func EnsureAzureTags(tags map[string]*string, projectID, uniqueID string) map[string]*string {
	if tags == nil {
		tags = map[string]*string{}
	}
	if tags["andaime"] == nil {
		tags["andaime"] = to.Ptr("true")
	}
	if tags["deployed-by"] == nil {
		tags["deployed-by"] = to.Ptr("andaime")
	}
	if tags["andaime-resource-tracking"] == nil {
		tags["andaime-resource-tracking"] = to.Ptr("true")
	}
	if tags["unique-id"] == nil {
		tags["unique-id"] = to.Ptr(uniqueID)
	}
	if tags["project-id"] == nil {
		tags["project-id"] = to.Ptr(projectID)
	}
	if tags["andaime-project"] == nil {
		tags["andaime-project"] = to.Ptr(fmt.Sprintf("%s-%s", uniqueID, projectID))
	}
	return tags
}

func EnsureGCPLabels(labels map[string]string, projectID, uniqueID string) map[string]string {
	if labels == nil {
		labels = map[string]string{}
	}
	if _, ok := labels["andaime"]; !ok {
		labels["andaime"] = "true"
	}
	if _, ok := labels["deployed-by"]; !ok {
		labels["deployed-by"] = "andaime"
	}
	if _, ok := labels["andaime-resource-tracking"]; !ok {
		labels["andaime-resource-tracking"] = "true"
	}
	if _, ok := labels["unique-id"]; !ok {
		labels["unique-id"] = uniqueID
	}
	if _, ok := labels["project-id"]; !ok {
		labels["project-id"] = projectID
	}
	if _, ok := labels["andaime-project"]; !ok {
		labels["andaime-project"] = fmt.Sprintf("%s-%s", uniqueID, projectID)
	}
	return labels
}

func StructToMap(obj interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
