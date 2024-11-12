package internal_aws

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"gopkg.in/yaml.v2"
)

type AWSData struct {
	Locations []string            `yaml:"locations"`
	VMSizes   map[string][]string `yaml:"vmsizes"`
}

func getSortedAWSData() ([]byte, error) {
	data, err := GetAWSData()
	if err != nil {
		return nil, err
	}

	var rawData map[string]interface{}
	err = yaml.Unmarshal(data, &rawData)
	if err != nil {
		return nil, err
	}

	awsData := AWSData{
		Locations: make([]string, 0),
		VMSizes:   make(map[string][]string),
	}

	// Extract locations and VM sizes
	if locations, ok := rawData["locations"].(map[interface{}]interface{}); ok {
		for region, vmSizes := range locations {
			regionStr := region.(string)
			awsData.Locations = append(awsData.Locations, regionStr)

			if sizes, ok := vmSizes.([]interface{}); ok {
				awsData.VMSizes[regionStr] = make([]string, len(sizes))
				for i, size := range sizes {
					awsData.VMSizes[regionStr][i] = size.(string)
				}
			}
		}
	}

	// Sort locations
	sort.Strings(awsData.Locations)

	// Sort VM sizes for each location
	for _, sizes := range awsData.VMSizes {
		sort.Strings(sizes)
	}

	return yaml.Marshal(awsData)
}

func IsValidAWSRegion(region string) bool {
	l := logger.Get()
	data, err := getSortedAWSData()
	if err != nil {
		l.Warnf("Failed to get sorted AWS data: %v", err)
		return false
	}

	var awsData AWSData
	err = yaml.Unmarshal(data, &awsData)
	if err != nil {
		l.Warnf("Failed to unmarshal AWS data: %v", err)
		return false
	}

	for _, loc := range awsData.Locations {
		if strings.EqualFold(loc, region) {
			return true
		}
	}

	l.Warnf("Invalid AWS region: %s", region)
	return false
}

func IsValidAWSInstanceType(region, instanceType string) bool {
	l := logger.Get()
	data, err := getSortedAWSData()
	if err != nil {
		l.Warnf("Failed to get sorted AWS data: %v", err)
		return false
	}

	var awsData AWSData
	err = yaml.Unmarshal(data, &awsData)
	if err != nil {
		l.Warnf("Failed to unmarshal AWS data: %v", err)
		return false
	}

	instanceTypes, exists := awsData.VMSizes[region]
	if !exists {
		l.Warnf("Region not found: %s", region)
		return false
	}

	for _, size := range instanceTypes {
		if size == instanceType {
			return true
		}
	}

	l.Warnf("Invalid instance type for region: %s, instanceType: %s", region, instanceType)
	return false
}

func GetAllAWSRegions() ([]string, error) {
	data, err := getSortedAWSData()
	if err != nil {
		return nil, err
	}

	var awsData AWSData
	err = yaml.Unmarshal(data, &awsData)
	if err != nil {
		return nil, err
	}

	return awsData.Locations, nil
}

func GetAWSInstanceTypes(region string) ([]string, error) {
	l := logger.Get()
	data, err := getSortedAWSData()
	if err != nil {
		l.Warnf("Failed to get sorted AWS data: %v", err)
		return nil, err
	}

	var awsData AWSData
	err = yaml.Unmarshal(data, &awsData)
	if err != nil {
		l.Warnf("Failed to unmarshal AWS data: %v", err)
		return nil, err
	}

	instanceTypes, exists := awsData.VMSizes[region]
	if !exists {
		l.Warnf("Region not found: %s", region)
		return nil, fmt.Errorf("region not found: %s", region)
	}

	return instanceTypes, nil
}
