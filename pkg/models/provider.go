package models

// GetProviderAbbreviation returns the abbreviation for a given provider
func GetProviderAbbreviation(provider string) string {
	switch provider {
	case "Azure":
		return "AZU"
	case "AWS":
		return "AWS"
	case "GCP":
		return "GCP"
	default:
		return "UNK"
	}
}
