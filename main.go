package main

import (
	"os"

	"github.com/sentinel-official/sentinel-go-sdk/app"

	"github.com/sentinel-official/sentinel-dvpncli/cmd"
)

func main() {
	exitCode := app.Run(cmd.NewRootCmd)
	os.Exit(exitCode)
}
