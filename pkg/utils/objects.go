package utils

import (
	"encoding/json"
	"fmt"
)

func EnsureAzureTags(tags map[string]string, projectID, uniqueID string) map[string]string {
	if tags == nil {
		tags = map[string]string{}
	}
	if _, ok := tags["andaime"]; !ok {
		tags["andaime"] = "true" //nolint:goconst
	}
	if _, ok := tags["deployed-by"]; !ok {
		tags["deployed-by"] = "andaime"
	}
	if _, ok := tags["andaime-resource-tracking"]; !ok {
		tags["andaime-resource-tracking"] = "true" //nolint:goconst
	}
	if _, ok := tags["unique-id"]; !ok {
		tags["unique-id"] = uniqueID
	}
	if _, ok := tags["project-id"]; !ok {
		tags["project-id"] = projectID
	}
	if _, ok := tags["andaime-project"]; !ok {
		tags["andaime-project"] = fmt.Sprintf("%s-%s", uniqueID, projectID)
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
