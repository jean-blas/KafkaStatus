package cmd

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

var kTopicCmd = &cobra.Command{
	Use:   "ktopic",
	Short: "[PaaS] Display topics info inside a PaaS",

	Run: func(cmd *cobra.Command, args []string) {
		getClientsetOrDie()
		if namespace == "" {
			getKafkaNs(nil)
		} else {
			getKafkaNs(strings.Split(namespace, ","))
		}
		getPodsAllNs()
		describeTopicsAllNs()
		var tpcs []string = nil
		if topics != "" {
			tpcs = strings.Split(strings.TrimSpace(topics), ",") //input like topic1,topic2,topic3
		}
		if short {
			for _, ns := range Namespaces {
				nT, nP, nPR := sumTopicsDetails(ns.TopicDetails)
				fmt.Printf("%s [t=%d p=%d pr=%d]\n", ns.Name(), nT, nP, nPR)
				for _, t := range ns.TopicDetails {
					if topics == "" || inArray(tpcs, t.name) {
						fmt.Printf("\t%s\n", t.name)
					}
				}
			}
		} else {
			for _, ns := range Namespaces {
				nT, nP, nPR := sumTopicsDetails(ns.TopicDetails)
				fmt.Printf("%s [t=%d p=%d pr=%d]\n", ns.Name(), nT, nP, nPR)
				fmt.Println(toString(ns.TopicDetails, tpcs))
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(kTopicCmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
}

func sumTopicsDetails(at []topicDetails) (int, int, int) {
	sumP, sumPR := 0, 0
	for _, t := range at {
		sumP += t.nbOfPartitions
		sumPR += t.replication * t.nbOfPartitions
	}
	return len(at), sumP, sumPR
}

func describeTopicsAllNs() {
	var wg sync.WaitGroup
	for i := range Namespaces {
		wg.Add(1)
		go func(i int) {
			err := Namespaces[i].describeTopics()
			logErr(err)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

// Describe all topics of the given namespace
func (n *NAMESPACE) describeTopics() error {
	kpods := n.getKafkaPods()
	if len(kpods) == 0 {
		return errors.New("No kafka pods in " + n.Name())
	}
	const command = "bin/kafka-topics.sh --bootstrap-server localhost:9092 --describe"
	stdout, stderr, err := execToPod(command, "kafka", kpods[0].Name, n.Name(), nil)
	if len(stderr) != 0 {
		return errors.New(stderr)
	}
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`^Topic:\s(.*)\s*PartitionCount:\s(\d*)\s*ReplicationFactor:\s(\d*)\s*Configs:\s(.*)$`)
	tds := make([]topicDetails, 0)
	lines := strings.Split(stdout, "\n")
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
				tds = append(tds, details)
				i += p + 1
			}
		} else {
			i += 1
		}
	}
	sortTopicsDetails(&tds)
	n.TopicDetails = tds

	return nil
}
