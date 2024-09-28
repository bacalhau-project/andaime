package gcp

import (
	"context"
	"fmt"
	"sort"

	"cloud.google.com/go/asset/apiv1/assetpb"
	gcp_provider "github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func GetListAllAssetsInProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-assets <project-id>",
		Short: "List all assets in a GCP project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			organizationID := viper.GetString("gcp.organization_id")
			if organizationID == "" {
				return fmt.Errorf("organization_id is not set in the configuration")
			}
			return listAllAssetsInProject(projectID)
		},
	}
	return cmd
}

func listAllAssetsInProject(projectID string) error {
	ctx := context.Background()
	gcpProvider, err := gcp_provider.NewGCPProviderFunc(
		ctx,
		viper.GetString("gcp.organization_id"),
		viper.GetString("gcp.billing_account_id"),
	)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	resources, err := gcpProvider.ListAllAssetsInProject(ctx, projectID)
	if err != nil {
		return handleGCPError(err)
	}

	prog := tea.NewProgram(initialModel(resources))
	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("error running program: %v", err)
	}

	return nil
}

type model struct {
	table table.Model
}

func initialModel(resources []*assetpb.Asset) model {
	columns := []table.Column{
		{Title: "Type", Width: 30},
		{Title: "Name", Width: 50},
		{Title: "State", Width: 20},
	}

	rows := make([]table.Row, 0, len(resources))
	for _, r := range resources {
		rows = append(rows, table.Row{
			r.AssetType,
			r.Name,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i][0] < rows[j][0]
	})

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(len(rows)),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return model{
		table: t,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return m.table.View() + "\nPress q to quit\n"
}
