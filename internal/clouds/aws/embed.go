package internal_aws

import (
	"embed"
)

//go:embed aws_data.yaml
var awsData embed.FS

func GetAWSData() ([]byte, error) {
	data, err := awsData.ReadFile("aws_data.yaml")
	if err != nil {
		return nil, err
	}
	return data, nil
}
