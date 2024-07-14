package cmd

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func ExecuteCommand(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	_, err = root.ExecuteC()

	return buf.String(), err
}

func TestBetaListUbuntuAMIs(t *testing.T) {
	viper.Reset()
	viper.Set("aws.regions", []string{"us-east-1", "us-west-2"})

	rootCmd := &cobra.Command{
		Use: "andaime",
	}
	betaCmd := &cobra.Command{
		Use: "beta",
	}
	listCmd := &cobra.Command{
		Use: "list",
	}
	betaCmd.AddCommand(listCmd)
	rootCmd.AddCommand(betaCmd)

	output, err := ExecuteCommand(rootCmd, "beta", "list")
	if err != nil {
		t.Fatalf("Error executing command: %v", err)
	}

	expectedOutput := "UBUNTU_AMIS = {\n    \"us-east-1\": \"ami-08d4ac5b634ca7137\",\n    \"us-west-2\": \"ami-0145805b99401a49c\",\n}\n"
	if output != expectedOutput {
		t.Errorf("Unexpected output:\nExpected: %v\nGot: %v", expectedOutput, output)
	}
}
