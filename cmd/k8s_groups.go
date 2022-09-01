package cmd

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

var kGroupCmd = &cobra.Command{
	Use:   "kgroup",
	Short: "[PaaS] Display groups info inside a PaaS",

	Run: func(cmd *cobra.Command, args []string) {
		getClientsetOrDie()
		if namespace == "" {
			getKafkaNs(nil)
		} else {
			getKafkaNs(strings.Split(namespace, ","))
		}
		getPodsAllNs()
		if strings.TrimSpace(groups) == "" { // List all groups
			stateGroupsAllNs()
			for _, ns := range Namespaces {
				fmt.Println(ns.Name())
				printGroupList(ns.Groups)
			}
		} else { // Describe/members the given group
			for i := range Namespaces {
				for _, g := range strings.Split(groups, ",") {
					Namespaces[i].Groups = append(Namespaces[i].Groups, GROUP{name: g})
				}
			}
			fillGivenGroupsAllNs()
			for _, ns := range Namespaces {
				printGroupAllNs(ns)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(kGroupCmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
}

func printGroupAllNs(ns NAMESPACE) {
	fmt.Println("Namespace: ", ns.Name())
	sortGroups(&ns.Groups)
	printGroupList(ns.Groups)
	for _, group := range ns.Groups {
		for _, m := range group.members {
			fmt.Println(m)
		}
		for _, d := range group.describe {
			fmt.Println(d)
		}
	}
}

func fillGivenGroupsAllNs() {
	var wg sync.WaitGroup
	for i := range Namespaces {
		wg.Add(3)
		go func(i int) {
			Namespaces[i].stateGivenGroups()
			wg.Done()
		}(i)
		go func(i int) {
			Namespaces[i].getGivenGroups(OPT_GROUP_MEMBERS)
			wg.Done()
		}(i)
		go func(i int) {
			Namespaces[i].getGivenGroups(OPT_GROUP_DESCRIBE)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func (n *NAMESPACE) getGivenGroups(s string) error {
	kpods := n.getKafkaPods()
	if len(kpods) == 0 {
		return errors.New("No kafka pods in " + n.Name())
	}
	var wg sync.WaitGroup
	for i := range n.Groups {
		wg.Add(1)
		go func(i int) {
			command := "bin/kafka-consumer-groups.sh --bootstrap-server localhost:9092 --describe --group " + n.Groups[i].name + " --verbose " + s
			stdout, stderr, err := execToPod(command, "kafka", kpods[0].Name, n.Name(), nil)
			if len(stderr) != 0 {
				logErr(errors.New(stderr))
				wg.Done()
				return
			}
			if logErr(err) {
				wg.Done()
				return
			}
			lines := make([]string, 0)
			for _, line := range strings.Split(stdout, "\n") {
				if !strings.HasPrefix(line, "[") {
					lines = append(lines, line)
				}
			}
			switch s {
			case OPT_GROUP_DESCRIBE:
				n.Groups[i].describe = lines
			case OPT_GROUP_MEMBERS:
				n.Groups[i].members = lines
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	return nil
}

// Describe all groups of the given namespace
func (n *NAMESPACE) stateGivenGroups() error {
	kpods := n.getKafkaPods()
	if len(kpods) == 0 {
		return errors.New("No kafka pods in " + n.Name())
	}
	var wg sync.WaitGroup
	for i := range n.Groups {
		wg.Add(1)
		go func(i int) {
			command := "bin/kafka-consumer-groups.sh --bootstrap-server localhost:9092 --describe --group " + n.Groups[i].name + " --verbose --state"
			stdout, stderr, err := execToPod(command, "kafka", kpods[0].Name, n.Name(), nil)
			if len(stderr) != 0 {
				logErr(errors.New(stderr))
				wg.Done()
				return
			}
			if logErr(err) {
				wg.Done()
				return
			}
			lines := strings.Split(stdout, "\n")
			for _, line := range lines {
				if !strings.Contains(line, "#MEMBERS") && strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "[") {
					n.Groups[i].state = line
					break
				}
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	return nil
}

func stateGroupsAllNs() {
	var wg sync.WaitGroup
	for i := range Namespaces {
		wg.Add(1)
		go func(i int) {
			err := Namespaces[i].stateGroups()
			logErr(err)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

// Describe all groups of the given namespace
func (n *NAMESPACE) stateGroups() error {
	kpods := n.getKafkaPods()
	if len(kpods) == 0 {
		return errors.New("No kafka pods in " + n.Name())
	}
	const command = "bin/kafka-consumer-groups.sh --bootstrap-server localhost:9092 --describe --all-groups --verbose --state"
	stdout, stderr, err := execToPod(command, "kafka", kpods[0].Name, n.Name(), nil)
	if len(stderr) != 0 {
		return errors.New(stderr)
	}
	if err != nil {
		return err
	}
	lines := strings.Split(stdout, "\n")
	grps := make([]GROUP, 0)
	for _, line := range lines {
		if !strings.Contains(line, "#MEMBERS") && strings.TrimSpace(line) != "" {
			grps = append(grps, GROUP{state: line})
		}
	}
	n.Groups = grps
	return nil
}
