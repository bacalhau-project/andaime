package cmd

import (
	"flag"
	"fmt"
)

func PrintUsage() {
	fmt.Println("Usage: ./main <create|destroy|list> [options]")
	fmt.Println("Commands:")
	fmt.Println("  create              Create AWS resources")
	fmt.Println("  destroy             Destroy AWS resources")
	fmt.Println("  list                List AWS resources tagged with 'project: andaime'")
	fmt.Println("Options:")
	flag.PrintDefaults()
}
