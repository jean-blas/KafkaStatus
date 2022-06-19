package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Represent the config status command
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display some stats of the given cluster(s)",
	Long: `Used together with git, display some info of all clusters that are in the branch repository (e.g. ERDING_DEV)
	Note : use the --http-timeout option to increase the timeout.
	e.g. go run kstat.go --git-branch ERDING_DEV --git-login jimbert --short info `,
	Run: func(cmd *cobra.Command, args []string) {
		var servers []SERVER
		var err error
		if strings.TrimSpace(gitBranch) != "" { // Build the inventory from git branch
			servers, err = buildServersFromGit()
		} else {
			servers, err = buildServers() // Build the inventory from the command line [-c cluster1,cluster2,...]
		}
		logFatal(err)
		nodeMetrics := initNodeMetrics()
		kafkaMetrics := initKafkaMetrics()
		fillInfo(servers, nodeMetrics, kafkaMetrics)
		displayMetrics(servers, nodeMetrics, kafkaMetrics)
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
}

func initNodeMetrics() []string {
	return []string{
		"node_filesystem_avail_bytes",
		"node_filesystem_free_bytes",
		"node_filesystem_size_bytes",
	}
}

func initKafkaMetrics() []string {
	return []string{
		"kafka_app_info", // return kafka server version (e.g. 2.8.1)
	}
}

// Send a GET request to the broker on the given port at /metrics
func sendRequest(broker, port string) ([]byte, error) {
	client := &http.Client{Timeout: time.Duration(httpTimeout) * time.Millisecond}
	furl := "http://" + broker + ":" + port + "/metrics"
	log.Debug("Send request to " + furl)
	r, err := client.Get(furl)
	if r != nil {
		defer r.Body.Close()
	}
	if logErr(err) {
		return nil, err
	}
	log.Debug("request status : " + r.Status)
	// Parse the response and extract the body as []bytes
	return ioutil.ReadAll(r.Body)
}

func infoGetMetrics(body []byte, metrics []string) map[string]METRIC {
	res := make(map[string]METRIC, 0)
	lines := strings.Split(string(body), "\n")
	for _, m := range metrics {
		reHELP := regexp.MustCompile(`^#\s*HELP\s*` + m + `\s*(.*)$`)
		reNodeValue := regexp.MustCompile(`^` + m + `({.*})*\s*(.*)$`)
		reAppInfoValue := regexp.MustCompile("^" + m + "{version=\"(.*)\",}.*$")
		for _, line := range lines {
			if strings.Contains(line, m) {
				if reHELP.MatchString(line) {
					as := reHELP.FindStringSubmatch(line)
					res[m] = METRIC{h: as[1]}
				} else {
					if m == "node_filesystem_avail_bytes" || m == "node_filesystem_free_bytes" || m == "node_filesystem_size_bytes" {
						if strings.Contains(line, "/opt/kafkadata") {
							if reNodeValue.MatchString(line) {
								mm := res[m]
								mm.v = reNodeValue.FindStringSubmatch(line)[2]
								mm.l = line
								res[m] = mm
							}
						}
					} else if m == "kafka_app_info" {
						if reAppInfoValue.MatchString(line) {
							mm := res[m]
							mm.v = reAppInfoValue.FindStringSubmatch(line)[1]
							mm.l = line
							res[m] = mm
						}
					}
				}
			}
		}
	}
	return res
}

func fillBrokerMetrics(server *SERVER, nodeMetrics, kafkaMetrics []string) {
	var wg sync.WaitGroup
	brokers := strings.Split(server.bootstrap, ",")
	for _, bp := range brokers {
		b := strings.Split(bp, ":")
		wg.Add(1)
		go func(broker string) {
			m := make(map[string]METRIC, 0)
			body, err := sendRequest(broker, "50700")
			if !logErr(err) {
				m = infoGetMetrics(body, nodeMetrics)
			}
			body, err = sendRequest(broker, "50721")
			if !logErr(err) {
				m2 := infoGetMetrics(body, kafkaMetrics)
				for k, v := range m2 {
					m[k] = v
				}
			}
			server.brokermetrics = append(server.brokermetrics, BROKERMETRICS{metrics: m})
			wg.Done()
		}(b[0])
		wg.Wait()
	}
}

func fillInfo(servers []SERVER, nodeMetrics, kafkaMetrics []string) {
	var wg sync.WaitGroup
	for i := range servers {
		wg.Add(1)
		go func(s *SERVER) {
			fillBrokerMetrics(s, nodeMetrics, kafkaMetrics)
			wg.Done()
		}(&servers[i])
	}
	wg.Wait()
}

func computeLen(m []string) int {
	maxL := -1
	for _, k := range m {
		if maxL < len(k) {
			maxL = len(k)
		}
	}
	return maxL
}

func computeKafkadata(m map[string]METRIC) float64 {
	av, tot := m["node_filesystem_avail_bytes"].v, m["node_filesystem_size_bytes"].v
	if strings.TrimSpace(av) == "" || strings.TrimSpace(tot) == "" {
		return -1
	}
	fav, err := strconv.ParseFloat(av, 64)
	if logErr(err) {
		return -1
	}
	ftot, err := strconv.ParseFloat(tot, 64)
	if logErr(err) || ftot == 0. {
		return -1
	}
	return 100 - fav/ftot*100
}

func toGiga(metric string) float64 {
	if strings.TrimSpace(metric) == "" {
		return -1
	}
	const GIGA float64 = 1024. * 1024. * 1024.
	value, err := strconv.ParseFloat(metric, 64)
	if logErr(err) {
		return -1
	}
	return value / GIGA
}

func displayMetrics(servers []SERVER, nodeMetrics, kafkaMetrics []string) {
	for _, s := range servers {
		if short {
			fmt.Printf("%s : ", s.cluster)
			for _, bm := range s.brokermetrics {
				kdata := computeKafkadata(bm.metrics)
				fmt.Printf("%5.2f%%[%4.0fG] %5s  ", kdata, toGiga(bm.metrics["node_filesystem_size_bytes"].v), bm.metrics["kafka_app_info"].v)
			}
			fmt.Printf("\n")
		} else {
			fmt.Println(s.cluster)
			all := append(nodeMetrics, kafkaMetrics...)
			maxL := computeLen(all)
			for _, m := range all {
				fmt.Printf("%-*s : ", maxL, m)
				for _, bm := range s.brokermetrics {
					fmt.Printf(" %-20s ", bm.metrics[m].v)
				}
				fmt.Println(" ")
			}
		}
	}
}
