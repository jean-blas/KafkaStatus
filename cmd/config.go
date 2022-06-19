package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Represent the config status command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Display the config (static and dynamic) for the given cluster",

	Run: func(cmd *cobra.Command, args []string) {
		var err error
		servers := brokername
		if servers == "" {
			servers, err = clusterToBootstrap(clustername)
			logFatal(err)
		}
		result, err := config_cmd(servers)
		logFatal(err)
		fmt.Println("Config of", clustername)
		conf := extractConf(result)
		fmt.Println(config_toString(conf))
	},
}

var config_broker int
var with_null bool

func init() {
	rootCmd.AddCommand(configCmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
	configCmd.Flags().IntVarP(&config_broker, "number", "n", 0, "Broker ID")
	configCmd.Flags().BoolVarP(&with_null, "null", "", false, "Display the keys which have null value")
}

func config_cmd(servers string) (string, error) {
	if err := check_conn(servers); err != nil {
		return "", errors.New("No connection to the VMs\n" + err.Error())
	}
	log.Debug("Run command : kafka-configs.sh --bootstrap-server" + servers + " --describe --broker " + strconv.Itoa(config_broker) + " --all")
	ecmd := exec.Command("kafka-configs.sh", "--bootstrap-server", servers, "--describe", "--broker", strconv.Itoa(config_broker), "--all")
	var out bytes.Buffer
	ecmd.Stdout = &out
	if err := ecmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

type CONF struct {
	key, value string
	sensitive  bool
	synonym    string
}

func sortConf(a *[]CONF) {
	c := *a
	sort.Slice(*a, func(i, j int) bool {
		if c[i].key == c[j].key {
			return c[i].sensitive
		}
		return c[i].key < c[j].key
	})
}

func config_toString(c []CONF) string {
	if len(c) > 1 {
		sortConf(&c)
	}
	s := ""
	for _, cc := range c {
		if cc.value != "null" || with_null {
			s += fmt.Sprintln(cc.key, ":", cc.value)
			if !short {
				s += fmt.Sprintln("synonym=", cc.synonym)
			}
		}
	}
	return s
}

func extractConf(sconf string) []CONF {
	conf := make([]CONF, 0)
	aconf := strings.Split(sconf, "\n")
	for i, c := range aconf {
		if i == 0 {
			continue
		}
		kvs := strings.SplitN(strings.TrimSpace(c), " ", 3)
		if len(kvs) != 3 {
			continue
		}
		i := strings.Split(kvs[0], "=")
		b, err := strconv.ParseBool(strings.Split(kvs[1], "=")[1])
		if err != nil {
			logErr(errors.New("Bad value for kvs " + kvs[1]))
		}
		y := strings.SplitN(strings.TrimSpace(c), "synonyms=", 2)
		cc := CONF{key: i[0], value: i[1], synonym: y[1], sensitive: b}
		conf = append(conf, cc)
	}
	return conf
}
