package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	URP   = "--under-replicated-partitions"
	UMISR = "--under-min-isr-partitions"
	AMISR = "--at-min-isr-partitions"
	UNAV  = "--unavailable-partitions"
)

// Represent the health status command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check health info of a cluster",
	Long: `Used together with git, check the health of all clusters that are in the branch repository (e.g. ERDING_DEV)
	Note : if no option is selected (like --urp or --umisr), then all options will be checked.
	e.g. go run kstat.go --git-branch ERDING_DEV --git-login jimbert --short health
	You may as well reference the bootstrap servers from the git branch:
	e.g. go run kstat.go --git-branch ERDING_DEV --git-login jimbert --short --cluster bkt28 health `,

	Run: func(cmd *cobra.Command, args []string) {
		var servers []SERVER
		var err error
		if strings.TrimSpace(gitBranch) != "" { // Build the inventory from git branch
			servers, err = buildServersFromGit()
		} else {
			servers, err = buildServers() // Build the inventory from the command line [-c cluster1,cluster2,...]
		}
		logFatal(err)
		checkServersHealth(servers)
	},
}

var bURP, bUMISR, bUAV, bAMISR bool

func init() {
	rootCmd.AddCommand(healthCmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
	healthCmd.Flags().BoolVarP(&bURP, "urp", "", false, "Look only for under replicated partitions")
	healthCmd.Flags().BoolVarP(&bUMISR, "umisr", "", false, "Look only for under min in sync partitions")
	healthCmd.Flags().BoolVarP(&bAMISR, "amisr", "", false, "Look only for at min in sync partitions")
	healthCmd.Flags().BoolVarP(&bUAV, "uav", "", false, "Look only for partitions whose leader is unavailable")
}

func checkServersHealth(servers []SERVER) {
	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(c, s string) {
			checkBrokerHealth(c, s)
			wg.Done()
		}(s.cluster, s.bootstrap)
	}
	wg.Wait()
}

// Check the URP, UMISR, AMISR and UNAV for the given broker
func checkBrokerHealth(cluster, server string) {
	if err := check_conn(server); err != nil {
		log.Error(server + " " + err.Error())
		return
	}
	all := !bURP && !bUMISR && !bAMISR && !bUAV // if no option at all <=> all options selected
	var errURP, errUMISR, errAMISR, errUNAV error
	var resURP, resUMISR, resAMISR, resUNAV string
	var wg sync.WaitGroup
	if all || bURP {
		topics_runCmdForHealthCheckAsync(&wg, server, URP, &resURP, &errURP)
	}
	if all || bUMISR {
		topics_runCmdForHealthCheckAsync(&wg, server, UMISR, &resUMISR, &errUMISR)
	}
	if all || bAMISR {
		topics_runCmdForHealthCheckAsync(&wg, server, AMISR, &resAMISR, &errAMISR)
	}
	if all || bUAV {
		topics_runCmdForHealthCheckAsync(&wg, server, UNAV, &resUNAV, &errUNAV)
	}
	wg.Wait()
	if logErr(errURP, errUMISR, errAMISR, errUNAV) {
		return
	}
	nURP, nUMISR, nAMISR, nUNAV := nbLines(resURP, resUMISR, resAMISR, resUNAV)
	fmt.Printf("%s: URP: %3d, UMISR: %3d, AMISR: %3d, UNAV: %3d\n", cluster, nURP, nUMISR, nAMISR, nUNAV)
	if !short {
		fmt.Println("\n URP:\n", resURP, "\n UMISR:\n", resUMISR, "\n AMISR:\n", resAMISR, "\n UNAV:\n", resUNAV)
	}
}

func topics_runCmdForHealthCheckAsync(wg *sync.WaitGroup, broker, option string, res *string, err *error) {
	wg.Add(1)
	go func(res *string, err *error) {
		*res, *err = topics_cmdForHealthCheck(broker, option)
		wg.Done()
	}(res, err)
}

func topics_cmdForHealthCheck(broker, option string) (string, error) {
	log.Debug("Run command : kafka-topics.sh --bootstrap-server " + broker + " --describe " + option)
	ecmd := exec.Command("kafka-topics.sh", "--bootstrap-server", broker, "--describe", option)
	var out bytes.Buffer
	ecmd.Stdout = &out
	if err := ecmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func nbLines(res1, res2, res3, res4 string) (int, int, int, int) {
	lines1 := strings.Split(res1, "\n")
	lines2 := strings.Split(res2, "\n")
	lines3 := strings.Split(res3, "\n")
	lines4 := strings.Split(res4, "\n")
	return len(lines1) - 1, len(lines2) - 1, len(lines3) - 1, len(lines4) - 1
}
