package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Represent the group status command
var groupCmd = &cobra.Command{
	Use:   "group",
	Short: "Check group info of a cluster",
	Long: `By default, get the list of groups (option --short)
  or the list of groups along with their state (default, no option)
  of the given clusters (clusters are comma separated).
If a group is passed (or several groups with comma separator), then describe, members and state
  are retrieved for the given group(s).`,

	Run: func(cmd *cobra.Command, args []string) {
		servers, err := initServers()
		logFatal(err)
		if strings.TrimSpace(sGroup) == "" { // List all groups
			group_list(servers)
			if !short {
				group_state(servers)
			}
			printGroupsListAllServers(servers)
		} else { // Describe/members the given group
			for i := range servers {
				for _, g := range strings.Split(sGroup, ",") {
					servers[i].groups = append(servers[i].groups, GROUP{name: g})
				}
			}
			group_state(servers)
			group_members(servers)
			group_describe(servers)
			printGroupWithDetails(servers)
		}
	},
}

const (
	OPT_GROUP_DESCRIBE = "--describe"
	OPT_GROUP_STATE    = "--state"
	OPT_GROUP_MEMBERS  = "--members"
)

var sGroup string

func init() {
	rootCmd.AddCommand(groupCmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
	groupCmd.Flags().StringVarP(&sGroup, "group", "g", "", "Groups to describe (separator is comma for several groups)")
}

func group_describe(servers []SERVER) {
	var wg sync.WaitGroup
	for i, s := range servers {
		for j, g := range s.groups {
			wg.Add(1)
			go func(j int, g string, server *SERVER) {
				desc, err := group_info_cmd(server.bootstrap, g, "")
				if !logErr(err) {
					for _, gd := range strings.Split(desc, "\n") {
						server.groups[j].describe = append(server.groups[j].describe, gd)
					}
				}
				wg.Done()
			}(j, g.name, &servers[i])
		}
	}
	wg.Wait()
}

func group_members(servers []SERVER) {
	var wg sync.WaitGroup
	for i, s := range servers {
		for j, g := range s.groups {
			wg.Add(1)
			go func(j int, g string, server *SERVER) {
				members, err := group_info_cmd(server.bootstrap, g, OPT_GROUP_MEMBERS)
				if !logErr(err) {
					for _, gm := range strings.Split(members, "\n") {
						server.groups[j].members = append(server.groups[j].members, gm)
					}
				}
				wg.Done()
			}(j, g.name, &servers[i])
		}
	}
	wg.Wait()
}

func group_state(servers []SERVER) {
	var wg sync.WaitGroup
	for i, s := range servers {
		for j, g := range s.groups {
			wg.Add(1)
			go func(j int, g string, server *SERVER) {
				st, err := group_info_cmd(server.bootstrap, g, OPT_GROUP_STATE)
				if !logErr(err) {
					server.groups[j].state = strings.Split(st, "\n")[2]
				}
				wg.Done()
			}(j, g.name, &servers[i])
		}
	}
	wg.Wait()
}

func group_list(servers []SERVER) {
	var wg sync.WaitGroup
	for i := range servers {
		wg.Add(1)
		go func(s *SERVER) {
			groups, err := group_list_cmd(s.bootstrap)
			if !logErr(err) {
				for _, g := range strings.Split(groups, "\n") {
					s.groups = append(s.groups, GROUP{name: g})
				}
			}
			wg.Done()
		}(&servers[i])
	}
	wg.Wait()
}

func group_list_cmd(servers string) (string, error) {
	if err := check_conn(servers); err != nil {
		return "", errors.New("No connection to the VMs\n" + err.Error())
	}
	log.Debug("Run command : kafka-consumer-groups.sh --bootstrap-server" + servers + " --list")
	ecmd := exec.Command("kafka-consumer-groups.sh", "--bootstrap-server", servers, "--list")
	var out bytes.Buffer
	ecmd.Stdout = &out
	if err := ecmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func group_info_cmd(servers, group, option string) (string, error) {
	if err := check_conn(servers); err != nil {
		return "", errors.New("No connection to the VMs\n" + err.Error())
	}
	log.Debug("Run command : kafka-consumer-groups.sh --bootstrap-server"+servers+" --describe", "--group", group, "--verbose", option)
	ecmd := exec.Command("kafka-consumer-groups.sh", "--bootstrap-server", servers, "--describe", "--group", group, "--verbose", option)
	var out bytes.Buffer
	ecmd.Stdout = &out
	if err := ecmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func sortGroups(a *[]GROUP) {
	c := *a
	sort.Slice(*a, func(i, j int) bool {
		if c[i].name == c[j].name {
			return c[i].state < c[j].state
		}
		return c[i].name < c[j].name
	})
}

func printGroupsListAllServers(servers []SERVER) {
	for _, server := range servers {
		fmt.Println(server.cluster)
		printGroupList(server.groups)
	}
}

func printGroupList(groups []GROUP) {
	if short {
		sortGroups(&groups)
		for _, g := range groups {
			fmt.Println(g.name)
		}
	} else {
		m0, m1, m2 := groupStateMaxLen(getGroupState(groups))
		fmt.Printf("%-*s  %-*s  %s  %-*s  %s\n", m0, "GROUP", m1+5, "COORDINATOR (ID)", "ASSIGNMENT-STRATEGY", m2, "STATE", "#MEMBERS")
		sortGroups(&groups)
		re := regexp.MustCompile(`(\S*)\s*(\S*)\s\((\d*)\)\s*(\S*)\s*(\S*)\s*(\d*)`)
		for _, gs := range groups {
			if re.MatchString(gs.state) {
				ss := re.FindStringSubmatch(gs.state)
				s4, s5, s6 := ss[4], ss[5], ss[6] // Fix bug when displaying an empty group
				if ss[4] == "Empty" {
					s4, s5, s6 = "-", ss[4], ss[5]
				}
				fmt.Printf("%-*s  %-*s (%2s)  %-19s  %-*s  %s\n", m0, ss[1], m1, ss[2], ss[3], s4, m2, s5, s6)
			}
		}
	}
}

func getGroupState(groups []GROUP) []string {
	ss := make([]string, 0)
	for _, s := range groups {
		if s.state != "" {
			ss = append(ss, s.state)
		}
	}
	return ss
}

func groupStateMaxLen(as []string) (int, int, int) {
	re := regexp.MustCompile(`(\S*)\s*(\S*)\s\((\d*)\)\s*(\S*)\s*(\S*)\s*(\d*)`)
	max0, max1, max2 := -1, -1, -1
	for _, s := range as {
		if re.MatchString(s) {
			ss := re.FindStringSubmatch(s)
			l0, l1, l2 := len(ss[1]), len(ss[2]), len(ss[4])
			if l0 > max0 {
				max0 = l0
			}
			if l1 > max1 {
				max1 = l1
			}
			if l2 > max2 {
				max2 = l2
			}
		}
	}
	return max0, max1, max2
}

func printGroupWithDetails(servers []SERVER) {
	for _, server := range servers {
		fmt.Println(server.cluster)
		sortGroups(&server.groups)
		printGroupList(server.groups)
		for _, group := range server.groups {
			for _, m := range group.members {
				fmt.Println(m)
			}
			for _, d := range group.describe {
				fmt.Println(d)
			}
		}
	}
}
