package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Represent the inventory command
var inventoryCmd = &cobra.Command{
	Use:   "inventory",
	Short: "[ERDING] Build a ansible-like inventory based on a git branch",
	Long: `Used together with the -c|--cluster option, restrains the inventory to the given cluster
	e.g. go run kstat.go --git-branch ERDING_DEV --git-login jimbert -c bkt28 inventory `,
	Run: func(cmd *cobra.Command, args []string) {
		var inv string
		var err error
		check()
		if strings.TrimSpace(gitBranch) != "" { // Build the inventory from git branch
			inv, err = buildInventoryFromGit(invType)
			logFatal(err)
		} else {
			logFatal(errors.New("No git branch defined : please use the --git-branch command line option"))
		}
		if outfile != "" {
			writeTofile(outfile, inv)
		}
		if writeToStdin {
			fmt.Println(inv)
		}
	},
}

var invType, outfile string
var writeToStdin bool

func init() {
	rootCmd.AddCommand(inventoryCmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
	inventoryCmd.Flags().StringVarP(&invType, "inventory-type", "", "kafka", "Create the inventory for kafka, zookeeper or connect. ")
	inventoryCmd.Flags().StringVarP(&outfile, "outfile", "o", "", "Output file name ")
	inventoryCmd.Flags().BoolVarP(&writeToStdin, "stdin", "", true, "Write the inventory to stdin ")
}

func check() {
	if invType != "kafka" && invType != "zookeeper" && invType != "connect" {
		logFatal(errors.New("Bad value for inventory-type. Allowed values are kafka, zookeeper and connect"))
	}
}
