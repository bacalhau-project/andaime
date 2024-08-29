package main

import (
	"fmt"
	"os"

	"github.com/bacalhau-project/andaime/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
