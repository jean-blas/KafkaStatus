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

// Represent the topics status command
var topicsCmd = &cobra.Command{
	Use:   "topic",
	Short: "Display topic info of a cluster",

	Run: func(cmd *cobra.Command, args []string) {
		servers, err := initServers()
		logFatal(err)

		if strings.TrimSpace(topics_topic) != "" { // If topic defined display only these topics for all clusters
			tpcs := strings.ReplaceAll(strings.TrimSpace(topics_topic), ",", "\n") //input like topic1,topic2,topic3
			for i := range servers {
				servers[i].topics = tpcs
			}
		} else { // Look for all topics in all clusters
			getTopicsFromClusters(servers)
		}
		if short { // Display the topics properties for all clusters and exit
			for _, s := range servers {
				fmt.Printf("\n%s:\n", s.cluster)
				fmt.Println(strings.TrimSpace(s.topics))
			}
		} else { // Display the topics list with details for all clusters
			displayTopicWithDetails(servers)
		}
	},
}

var topics_topic string
var topics_describe bool

func init() {
	rootCmd.AddCommand(topicsCmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
	topicsCmd.Flags().StringVarP(&topics_topic, "topic", "t", "", "Topic names using comma as separator (e.g. topic1,topic2)")
	topicsCmd.Flags().BoolVarP(&topics_describe, "describe", "d", false, "Show the details of partitions")
}

type topicDetails struct {
	name, config                string
	nbOfPartitions, replication int
	partitions                  []string
}

func displayTopicWithDetails(servers []SERVER) {
	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(s SERVER) {
			topicsDetailed, err := getDetails(s.bootstrap, strings.Split(strings.TrimSpace(s.topics), "\n"))
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

func getTopicsFromClusters(servers []SERVER) {
	var wg sync.WaitGroup
	for i := range servers {
		wg.Add(1)
		go func(t *SERVER) {
			tpcs, err := topics_cmdList(t.bootstrap)
			if err != nil {
				log.Error(t.cluster, err)
			} else {
				t.topics = tpcs
			}
			wg.Done()
		}(&servers[i])
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
	return fmt.Sprintf("%s : p=%d  r=%d  c=%s\n", t.name, t.nbOfPartitions, t.replication, t.config)
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
		s += fmt.Sprintf("%-*s : p=%d  r=%d  c=%s\n", maxLName, t.name, t.nbOfPartitions, t.replication, t.config)
		if topics_describe {
			s += partitionsToString(t.partitions)
			s += "\n"
		}
	}
	return s
}

func partitionsToString(p []string) string {
	return strings.Join(p, "\n")
}

func sortTopicsDetails(a *[]topicDetails) {
	c := *a
	sort.Slice(*a, func(i, j int) bool {
		if c[i].name == c[j].name {
			return c[i].nbOfPartitions < c[j].nbOfPartitions
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
			lines := strings.Split(res, "\n")
			for i := 0; i < len(lines); {
				line := strings.TrimSpace(lines[i])
				if re.MatchString(line) {
					as := re.FindStringSubmatch(line)
					if len(as) == 5 {
						p, _ := strconv.Atoi(as[2])
						r, _ := strconv.Atoi(as[3])
						partitions := make([]string, p)
						for j := 0; j < p; j++ {
							partitions[j] = lines[j+i+1]
						}
						details := topicDetails{name: strings.TrimSpace(as[1]), nbOfPartitions: p, replication: r, config: as[4], partitions: partitions}
						*td = append(*td, details)
						i += p + 1
					}
					break
				} else {
					i += 1
				}
			}
			wg.Done()
		}(topic, &tds)
	}
	wg.Wait()
	return tds, nil
}
