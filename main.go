package main

import (
	"fmt"

	"github.com/IrineSistiana/mosdns/v5/mlog"
	"github.com/NikoTung/guber/cmd"
	"github.com/spf13/cobra"
)

var (
	version = "dev/unknown"
)

func init() {
	cmd.AddSubCmd(&cobra.Command{
		Use:   "version",
		Short: "Print out version info and exit.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	})
}

func main() {
	if err := cmd.Run(); err != nil {
		mlog.S().Fatal(err)
	}
}
