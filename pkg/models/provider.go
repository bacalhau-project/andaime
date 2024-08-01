package models

// GetProviderAbbreviation returns the abbreviation for a given provider
func GetProviderAbbreviation(provider string) string {
	switch provider {
	case "Azure":
		return "AZ"
	case "AWS":
		return "AWS"
	default:
		return "UNK"
	}
}
