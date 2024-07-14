package main

import (
	"context"
	"fmt"
	"log"

	"github.com/bacalhau-project/andaime/providers/aws"
)

func main() {
	provider := aws.NewAWSProvider()
	region := "us-west-2" // You might want to make this configurable

	amiID, err := provider.GetLatestUbuntuImage(context.Background(), region)
	if err != nil {
		log.Fatalf("Failed to get latest Ubuntu AMI: %v", err)
	}

	fmt.Printf("Latest Ubuntu AMI ID in region %s: %s\n", region, amiID)
}
