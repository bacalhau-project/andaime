package azure

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/bacalhau-project/andaime/pkg/models"
	azure_provider "github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/providers/factory"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var azureListSKUsCmd = &cobra.Command{
	Use:   "list-skus [location]",
	Short: "List Azure SKUs for a given location",
	Args:  cobra.ExactArgs(1),
	RunE:  runListAzureSKUs,
}

func GetAzureListSKUsCmd() *cobra.Command {
	return azureListSKUsCmd
}

type model struct {
	table table.Model
}

func (m model) Init() tea.Cmd {
	return tea.Quit
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, tea.Quit
}

func (m model) View() string {
	return m.table.View()
}

func runListAzureSKUs(cmd *cobra.Command, args []string) error {
	location := args[0]
	if location == "" {
		return fmt.Errorf("location is required")
	}

	p, err := factory.GetProvider(cmd.Context(), models.DeploymentTypeAzure)
	if err != nil {
		return err
	}
	azureProvider, ok := p.(azure_provider.AzureProviderer)
	if !ok {
		return fmt.Errorf("failed to assert provider to common.AzureProviderer")
	}

	skus, err := azureProvider.GetSKUsByLocation(cmd.Context(), location)
	if err != nil {
		return err
	}

	// Filter and sort SKUs
	var filteredSKUs []armcompute.ResourceSKU
	for _, sku := range skus {
		if sku.ResourceType != nil && *sku.ResourceType == "virtualMachines" {
			filteredSKUs = append(filteredSKUs, sku)
		}
	}
	sort.Slice(filteredSKUs, func(i, j int) bool {
		return *filteredSKUs[i].Name < *filteredSKUs[j].Name
	})

	// Create table
	//nolint:mnd
	columns := []table.Column{
		{Title: "Name", Width: 30},
		{Title: "Family", Width: 30},
		{Title: "vCPUs", Width: 10},
		{Title: "Memory", Width: 15},
		{Title: "Tier", Width: 15},
	}

	rows := []table.Row{}
	for _, sku := range filteredSKUs {
		family := "-"
		if sku.Family != nil {
			family = *sku.Family
		}
		vCPUs := "-"
		memory := "-"
		for _, capability := range sku.Capabilities {
			if *capability.Name == "vCPUs" {
				vCPUs = *capability.Value
			}
			if *capability.Name == "MemoryGB" {
				memory = *capability.Value + " GB"
			}
		}
		tier := "-"
		if sku.Tier != nil {
			tier = *sku.Tier
		}
		rows = append(rows, table.Row{*sku.Name, family, vCPUs, memory, tier})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(false),
		table.WithHeight(len(rows)),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)

	t.SetStyles(s)

	m := model{table: t}

	fmt.Printf("Azure SKUs for location: %s\n\n", strings.ToUpper(location))

	prog := tea.NewProgram(m)
	if _, err := prog.Run(); err != nil {
		return err
	}

	return nil
}
