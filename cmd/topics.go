package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
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

// Represent the acls status command
var topicsCmd = &cobra.Command{
	Use:   "topic",
	Short: "Display topic info of a cluster",

	Run: func(cmd *cobra.Command, args []string) {
		servers, err := buildServers()
		log.Debug(servers)
		logFatal(err)

		if bURP || bUMISR || bUAV || bAMISR || all { // Display the broker health and exit
			checkServersHealth(servers)
			return
		}

		if strings.TrimSpace(topics_topic) != "" { // If topic defined display only these topics for all clusters
			tpcs := strings.ReplaceAll(strings.TrimSpace(topics_topic), ",", "\n") //input like topic1,topic2,topic3
			for i := range servers {
				servers[i].inputtopics = tpcs
			}
		} else { // Look for all topics in all clusters
			getTopicsFromClusters(servers)
		}
		if summary { // Display the topics properties for all clusters and exit
			for _, t := range servers {
				fmt.Println(strings.TrimSpace(t.inputtopics))
			}
		} else { // Display the topics list with details for all clusters
			displayTopicWithDetails(servers)
		}
	},
}

var topics_topic string
var bURP, bUMISR, bUAV, bAMISR, summary, all bool

func init() {
	rootCmd.AddCommand(topicsCmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
	topicsCmd.Flags().StringVarP(&topics_topic, "topic", "t", "", "Topic names using comma as separator (e.g. topic1,topic2)")
	topicsCmd.Flags().BoolVarP(&bURP, "urp", "", false, "Look for under replicated partitions")
	topicsCmd.Flags().BoolVarP(&bUMISR, "umisr", "", false, "Look for under min in sync partitions")
	topicsCmd.Flags().BoolVarP(&bAMISR, "amisr", "", false, "Look for at min in sync partitions")
	topicsCmd.Flags().BoolVarP(&bUAV, "uav", "", false, "Look for partitions whose leader is unavailable")
	topicsCmd.Flags().BoolVarP(&summary, "sum", "s", false, "Display only a summary of the output")
	topicsCmd.Flags().BoolVarP(&all, "all", "a", false, "Check all health options")
}

type topicDetails struct {
	name, config            string
	partitions, replication int
}

type topics struct {
	cluster, bootstrp string
	inputtopics       string
}

func displayTopicWithDetails(servers []topics) {
	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(s topics) {
			topicsDetailed, err := getDetails(s.bootstrp, strings.Split(strings.TrimSpace(s.inputtopics), "\n"))
			if err != nil {
				log.Error("Error:", s.cluster, ":", err)
			} else {
				sortTopicsDetails(&topicsDetailed)
				fmt.Println(strings.Join([]string{s.cluster, toString(topicsDetailed)}, "\n"))
			}
			wg.Done()
		}(s)
	}
	wg.Wait()
}

// Construct the struct of bootstrap servers for each cluster
func buildServers() ([]topics, error) {
	tpcs := make([]topics, 0)
	fqdn := brokername
	if fqdn == "" {
		clusternames := splitClustername(clustername)
		for _, c := range clusternames {
			server, err := bootstrap(c)
			if err != nil {
				return nil, err
			}
			tpcs = append(tpcs, topics{cluster: c, bootstrp: server})
		}
	} else {
		cluster, err := toCluster(fqdn)
		if err != nil {
			return nil, err
		}
		tpcs = append(tpcs, topics{cluster: cluster, bootstrp: fqdn})
	}
	return tpcs, nil
}

func getTopicsFromClusters(servers []topics) {
	var wg sync.WaitGroup
	for i := range servers {
		wg.Add(1)
		go func(t *topics) {
			tpcs, err := topics_cmdList(t.bootstrp)
			if err != nil {
				log.Error(t.cluster, err)
			} else {
				t.inputtopics = tpcs
			}
			wg.Done()
		}(&servers[i])
	}
	wg.Wait()
}

func checkServersHealth(servers []topics) {
	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(c, s string) {
			checkBrokerHealth(c, s)
			wg.Done()
		}(s.cluster, s.bootstrp)
	}
	wg.Wait()
}

// List all topics of the given cluster
func topics_cmdList(broker string) (string, error) {
	log.Debug("Run command : kafka-topics.sh --bootstrap-server " + broker + " --list")
	ecmd := exec.Command("kafka-topics.sh", "--bootstrap-server", broker, "--list")
	var out bytes.Buffer
	ecmd.Stdout = &out
	if err := ecmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (t topicDetails) String() string {
	return fmt.Sprintf("%s : p=%d  r=%d  c=%s\n", t.name, t.partitions, t.replication, t.config)
}

func toString(at []topicDetails) string {
	maxLName := -1
	for _, t := range at {
		if len(t.name) > maxLName {
			maxLName = len(t.name)
		}
	}
	s := ""
	for _, t := range at {
		s += fmt.Sprintf("%-*s : p=%d  r=%d  c=%s\n", maxLName, t.name, t.partitions, t.replication, t.config)
	}
	return s
}

func sortTopicsDetails(a *[]topicDetails) {
	c := *a
	sort.Slice(*a, func(i, j int) bool {
		if c[i].name == c[j].name {
			return c[i].partitions < c[j].partitions
		}
		return c[i].name < c[j].name
	})
}

func topics_cmdForTopic(broker, topic string) (string, error) {
	log.Debug("Run command : kafka-topics.sh --bootstrap-server " + broker + " --describe --topic " + topic)
	ecmd := exec.Command("kafka-topics.sh", "--bootstrap-server", broker, "--describe", "--topic", topic)
	var out bytes.Buffer
	ecmd.Stdout = &out
	if err := ecmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func getDetails(broker string, topics []string) ([]topicDetails, error) {
	re := regexp.MustCompile(`^Topic:\s(.*)\s*PartitionCount:\s(\d*)\s*ReplicationFactor:\s(\d*)\s*Configs:\s(.*)$`)
	tds := make([]topicDetails, 0)
	var wg sync.WaitGroup
	for _, topic := range topics {
		wg.Add(1)
		go func(t string, td *[]topicDetails) {
			res, err := topics_cmdForTopic(broker, t)
			if err != nil {
				log.Println(err)
				wg.Done()
				return
			}
			for _, l := range strings.Split(res, "\n") {
				line := strings.TrimSpace(l)
				if re.MatchString(line) {
					as := re.FindStringSubmatch(line)
					if len(as) == 5 {
						p, _ := strconv.Atoi(as[2])
						r, _ := strconv.Atoi(as[3])
						*td = append(*td, topicDetails{name: strings.TrimSpace(as[1]), partitions: p, replication: r, config: as[4]})
					}
					break
				}
			}
			wg.Done()
		}(topic, &tds)
	}
	wg.Wait()
	return tds, nil
}

// Check the URP, UMISR, AMISR and UNAV for the given broker
func checkBrokerHealth(cluster, server string) {
	if err := check_conn(server); err != nil {
		log.Error(server + " " + err.Error())
		return
	}
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
	if !summary {
		fmt.Println("\n URP:\n", resURP, "\n UMISR:\n", resUMISR, "\n AMISR:\n", resAMISR, "\n UNAV:\n", resUNAV)
	}
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

func topics_runCmdForHealthCheckAsync(wg *sync.WaitGroup, broker, option string, res *string, err *error) {
	wg.Add(1)
	go func(res *string, err *error) {
		*res, *err = topics_cmdForHealthCheck(broker, option)
		wg.Done()
	}(res, err)
}

func nbLines(res1, res2, res3, res4 string) (int, int, int, int) {
	lines1 := strings.Split(res1, "\n")
	lines2 := strings.Split(res2, "\n")
	lines3 := strings.Split(res3, "\n")
	lines4 := strings.Split(res4, "\n")
	return len(lines1) - 1, len(lines2) - 1, len(lines3) - 1, len(lines4) - 1
}
