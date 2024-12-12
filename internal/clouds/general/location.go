package general

import (
	"strings"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

func NormalizeLocation(
	cloudProvider string,
	location string,
) (string, string, error) {
	l := logger.Get()
	lowerCloudProvider := strings.ToLower(cloudProvider)
	switch lowerCloudProvider {
	case "aws":
		// List of sample AWS locations:
		// https://docs.aws.amazon.com/general/latest/gr/aws-service-information.html
		// us-east-1*	N. Virginia	us-east-1a us-east-1b us-east-1c us-east-1d us-east-1e
		// us-east-2	Ohio	us-east-2a us-east-2b us-east-2c
		// us-west-1*	N. California	us-west-1a us-west-1b us-west-1c
		// us-west-2	Oregon	us-west-2a us-west-2b us-west-2c
		// eu-west-1	Ireland	eu-west-1a eu-west-1b eu-west-1c
		// eu-central-1	Frankfurt	eu-central-1a eu-central-1b
		// ap-northeast-1*	Tokyo	ap-northeast-1a ap-northeast-1b ap-northeast-1c
		// ap-northeast-2	Seoul	ap-northeast-2a ap-northeast-2c
		// ap-southeast-1	Singapore	ap-southeast-1a ap-southeast-1b
		// ap-southeast-2	Sydney	ap-southeast-2a ap-southeast-2b ap-southeast-2c
		// ap-south-1	Mumbai	ap-south-1a ap-south-1b
		// sa-east-1	Sao Paulo	sa-east-1a sa-east-1b sa-east-1c
		region, zone, err := ParseRegionZone(location)
		if err != nil {
			return "", "", err
		}

		l.Debugf("Normalized AWS location: %s -> %s, %s", location, region, zone)
		return region, zone, nil
	case "azure":
		// Azure locations are already normalized
		// United Arab Emirates      uae                  United Arab Emirates
		// United Kingdom            uk                   United Kingdom
		// United States             unitedstates         United States
		return location, location, nil
	case "gcp":
		// The GCP regions and zones are different from the AWS regions and zones.
		// Sydney, Australia	APAC	australia-southeast1-a, australia-southeast1-b, australia-southeast1-c
		// Tokyo, Japan	APAC	asia-northeast1-a, asia-northeast1-b, asia-northeast1-c
		// Frankfurt, Germany	Europe	europe-west3-a, europe-west3-b, europe-west3-c
		// Hamina, Finland	Europe	europe-north1-a, europe-north1-b, europe-north1-c
		// London, England, UK	Europe	europe-west2-a, europe-west2-b, europe-west2-c
		// Madrid, Spain	Europe	europe-southwest1-a, europe-southwest1-b, europe-southwest1-c
		region, zone, err := ParseRegionZone(location)
		if err != nil {
			return "", "", err
		}
		l.Debugf("Normalized GCP location: %s -> %s, %s", location, region, zone)
		return region, zone, nil
	}
	l.Warnf("Unknown cloud provider: %s", cloudProvider)
	return location, location, nil
}

// ParseRegionZone takes either a region (e.g., us-east-1) or zone (e.g., us-east-1a)
// and returns both the region and zone information.
// If a region is provided, the zone will be empty.
// If a zone is provided, both region and zone will be populated.
func ParseRegionZone(input string) (string, string, error) {
	// Early return for empty input
	if input == "" {
		return "", "", nil
	}

	// Handle GCP style zones (e.g., europe-west3-a)
	if parts := strings.Split(input, "-"); len(parts) > 2 && parts[len(parts)-1][0] >= 'a' &&
		parts[len(parts)-1][0] <= 'z' {
		// Input is a GCP zone, extract region by removing the last segment
		region := strings.Join(parts[:len(parts)-1], "-")
		return region, input, nil
	}

	// Check if the input is an AWS-style zone (ends with a letter a-z)
	if len(input) > 0 && input[len(input)-1] >= 'a' && input[len(input)-1] <= 'z' {
		// Input is a zone, extract region by removing the last character
		return input[:len(input)-1], input, nil
	}

	// Input is a region
	return input, input + "a", nil
}
