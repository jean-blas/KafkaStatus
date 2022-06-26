package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Represent the config status command
var partitionsCmd = &cobra.Command{
	Use:   "partitions",
	Short: "Display the log dir info",
	Long:  `Pretty display with the --short|-s option, else raw display`,

	Run: func(cmd *cobra.Command, args []string) {
		partitions_check()
		servers, err := initServers()
		logFatal(err)
		var wg sync.WaitGroup
		for i := range servers {
			wg.Add(1)
			go func(s *SERVER) {
				buildLogDir(s)
				wg.Done()
			}(&servers[i])
		}
		wg.Wait()
		for _, s := range servers {
			if short { // Pretty printing
				displayLogDirs(s)
			} else { // Raw printing
				fmt.Println(s.cluster + ":")
				fmt.Println(s.logdirs)
			}
		}
	},
}

var brokerList string

func init() {
	rootCmd.AddCommand(partitionsCmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
	partitionsCmd.Flags().StringVarP(&brokerList, "broker-list", "", "", "The list of brokers to be queried in the form 0,1,2. All brokers in the cluster will be queried if no broker list is specified")
}

func partitions_check() {
	if brokerList == "" {
		return
	}
	brokerIds := strings.Split(brokerList, ",")
	for _, id := range brokerIds {
		if _, err := strconv.Atoi(id); err != nil {
			logFatal(err)
		}
	}
}

func partitions_cmd(brokers string) (string, error) {
	if err := check_conn(brokers); err != nil {
		return "", errors.New("No connection to the VMs\n" + err.Error())
	}
	var ecmd *exec.Cmd
	if brokerList != "" {
		log.Debug("Run command : kafka-log-dirs.sh --bootstrap-server " + brokers + " --describe --broker-list " + brokerList)
		ecmd = exec.Command("kafka-log-dirs.sh", "--bootstrap-server", brokers, "--describe", "--broker-list", brokerList)
	} else {
		log.Debug("Run command : kafka-log-dirs.sh --bootstrap-server " + brokers + " --describe")
		ecmd = exec.Command("kafka-log-dirs.sh", "--bootstrap-server", brokers, "--describe")
	}
	var out bytes.Buffer
	ecmd.Stdout = &out
	if err := ecmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func buildLogDir(server *SERVER) error {
	temp, err := partitions_cmd(server.bootstrap)
	if err != nil {
		return err
	}
	logdirs := strings.Split(temp, "\n")[2]
	var ld LOGDIRS
	err = json.Unmarshal([]byte(logdirs), &ld)
	if err == nil {
		server.logdirs = ld
	}
	return err
}

type NUM_SIZE struct {
	num, size int
}

// Group the topics together before displaying
func groupParts(parts []PART) map[string][]NUM_SIZE {
	res := make(map[string][]NUM_SIZE, 0)
	for _, p := range parts {
		idx := strings.LastIndex(p.Partition, "-")
		name := p.Partition[:idx]
		num, err := strconv.Atoi(p.Partition[idx+1:])
		if logErr(err) {
			continue
		}
		numsize, exist := res[name]
		if !exist {
			res[name] = []NUM_SIZE{NUM_SIZE{num: num, size: p.Size}}
		} else {
			numsize = append(numsize, NUM_SIZE{num: num, size: p.Size})
			res[name] = numsize
		}
	}
	return res
}

func sortKeys(m map[string][]NUM_SIZE) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortNumSize(a *[]NUM_SIZE) {
	c := *a
	sort.Slice(*a, func(i, j int) bool {
		if c[i].num == c[j].num {
			return c[i].size < c[j].size
		}
		return c[i].num < c[j].num
	})
}

func displayLogDirs(s SERVER) {
	fmt.Println(s.cluster + ":")
	for _, b := range s.logdirs.Brokers {
		fmt.Printf("\tbroker: %d\n", b.Broker)
		gmap := groupParts(b.LogDirs[0].Partitions)
		sorted_keys := sortKeys(gmap)
		for _, k := range sorted_keys {
			fmt.Printf("\t\t%s: ", k)
			numsize := gmap[k]
			sortNumSize(&numsize)
			for _, ns := range numsize {
				fmt.Printf("[%d %d] ", ns.num, ns.size)
			}
			fmt.Println()
		}
	}
}
