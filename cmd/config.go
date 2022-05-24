package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// Represent the acls status command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Display the config (static and dynamic) for the given cluster",

	Run: func(cmd *cobra.Command, args []string) {
		var err error
		broker := brokername
		if broker == "" {
			broker, err = bootstrap(clustername)
			logFatal(err)
		}
		fmt.Println("Display config of", clustername)

		result, err := config_cmd(broker)
		logFatal(err)
		conf := extractConf(result)
		fmt.Println(config_toString(conf))
	},
}

var config_broker int
var with_null, with_synonym bool

func init() {
	rootCmd.AddCommand(configCmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
	configCmd.Flags().IntVarP(&config_broker, "number", "n", 0, "Broker ID)")
	configCmd.Flags().BoolVarP(&with_null, "null", "u", false, "Display the keys which have null value")
	configCmd.Flags().BoolVarP(&with_synonym, "synonym", "s", false, "Display the keys which correspond to synonym as well")
}

func config_cmd(broker string) (string, error) {
	ecmd := exec.Command("kafka-configs.sh", "--bootstrap-server", broker, "--describe", "--broker", strconv.Itoa(config_broker), "--all")
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
			if with_synonym {
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
		kvs := strings.SplitN(strings.TrimSpace(c), " ", 2)
		if len(kvs) != 2 {
			continue
		}
		i := strings.Split(kvs[0], "=")
		b, err := strconv.ParseBool(strings.Split(kvs[1], "=")[1])
		if err != nil {
			fmt.Println("Bad value for kvs", kvs)
		}
		y := strings.Split(strings.TrimSpace(c), "synonyms=")
		cc := CONF{key: i[0], value: i[1], synonym: y[1], sensitive: b}
		conf = append(conf, cc)
	}
	return conf
}
