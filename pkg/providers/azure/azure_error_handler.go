package azure

import (
	"fmt"
	"strings"
)

// HandleAzureError checks if the error is related to exceeding the regional cores quota
// and returns a more user-friendly error message if it is.
func HandleAzureError(err error) error {
	if strings.Contains(err.Error(), "OperationNotAllowed") && strings.Contains(err.Error(), "exceeding approved Total Regional Cores quota") {
		return fmt.Errorf("Azure Quota Exceeded: The operation could not be completed as it would exceed the approved Total Regional Cores quota. Please request a quota increase for your subscription at https://aka.ms/ProdportalCRP/#blade/Microsoft_Azure_Capacity/UsageAndQuota.ReactView")
	}
	return err
}
