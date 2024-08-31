package main

import (
	"fmt"

	"github.com/bacalhau-project/andaime/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		return
	}
}
