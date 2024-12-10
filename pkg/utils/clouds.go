package utils

// NormalizeRegion removes the trailing letter from a region if it exists
func NormalizeRegion(region string) string {
	if len(region) > 0 && region[len(region)-1] >= 'a' && region[len(region)-1] <= 'z' {
		return region[:len(region)-1]
	}
	return region
}
