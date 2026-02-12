package logparser

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "log-parser",
		Short: "Log Parser is a tool for parsing and analyzing log files",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Log Parser is a tool for parsing and analyzing log files")
		},
	}

	rootCmd.AddCommand(parseCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var parseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Parse log files",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Parse log files")
	},
}
