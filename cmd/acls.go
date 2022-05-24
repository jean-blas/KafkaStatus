package cmd

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// Represent the acls status command
var aclsCmd = &cobra.Command{
	Use:   "acl",
	Short: "Display acls of all or subset topics of a cluster",

	Run: func(cmd *cobra.Command, args []string) {
		var err error
		broker := brokername
		if broker == "" {
			broker, err = bootstrap(clustername)
			logFatal(err)
		}
		fmt.Println("Display acls of ", clustername)
		if strings.TrimSpace(acls_topic) != "" {
			topics := extractTopics(acls_topic)
			var wg sync.WaitGroup
			for _, topic := range topics {
				wg.Add(1)
				go func(t string) {
					result, err := acls_cmdWithTopic(broker, t)
					if err != nil {
						log.Print("ERROR: ", err)
					}
					acls := extractAcls(result)
					fmt.Println(acls)
					wg.Done()
				}(topic)
			}
			wg.Wait()
		} else {
			result, err := acls_cmd(broker)
			logFatal(err)
			acls := extractAcls(result)
			fmt.Println(acls_toString(acls))
		}
	},
}

var acls_topic string

func init() {
	rootCmd.AddCommand(aclsCmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
	aclsCmd.Flags().StringVarP(&acls_topic, "topic", "t", "", "Topic names using comma as separator (e.g. topic1,topic2)")
}

func acls_cmdWithTopic(broker, topic string) (string, error) {
	ecmd := exec.Command("kafka-acls.sh", "--bootstrap-server", broker, "--list", "--topic", topic)
	var out bytes.Buffer
	ecmd.Stdout = &out
	if err := ecmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func acls_cmd(broker string) (string, error) {
	ecmd := exec.Command("kafka-acls.sh", "--bootstrap-server", broker, "--list")
	var out bytes.Buffer
	ecmd.Stdout = &out
	if err := ecmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

type ACL struct {
	topic, rtype, ptype string
	perms               []PERM
}

type PERM struct {
	user, host, perm string
	r, w, d          bool
}

func (a ACL) String() string {
	s := a.rtype + " " + a.topic + " " + "(" + a.ptype + ")\n"
	for _, p := range a.perms {
		s += "\t" + p.String() + "\n"
	}
	return s
}

func (a *ACL) updateAcl(user, host, oper, perm string) {
	for i := range a.perms {
		p := &a.perms[i]
		if p.user == user && p.host == host {
			p.updatePerm(oper)
			return
		}
	}
	p := PERM{user: user, host: host, perm: perm, r: false, w: false, d: false}
	p.updatePerm(oper)
	a.perms = append(a.perms, p)
}

func (p PERM) String() string {
	return p.perm + " " + rwd(p.r, p.w, p.d) + " " + p.host + " " + p.user
}

func rwd(r, w, d bool) string {
	var R, W, D string
	if r {
		R = "R"
	}
	if w {
		W = "W"
	}
	if d {
		D = "D"
	}
	return fmt.Sprintf("%2s %2s %2s", R, W, D)
}

func (p *PERM) updatePerm(oper string) {
	switch oper {
	case "READ":
		p.r = true
	case "WRITE":
		p.w = true
	case "DESCRIBE":
		p.d = true
	}
}

func sortAcls(a *[]ACL) {
	c := *a
	sort.Slice(*a, func(i, j int) bool {
		if c[i].rtype == c[j].rtype {
			return c[i].topic < c[j].topic
		}
		return c[i].rtype < c[j].rtype
	})
}

func acls_toString(acls []ACL) string {
	if len(acls) > 1 {
		sortAcls(&acls)
	}
	s := ""
	for _, a := range acls {
		s += fmt.Sprintln(a)
	}
	return s
}

func extractAcls(out string) []ACL {
	lines := strings.Split(out, "\n")
	reUser := regexp.MustCompile(`^\(principal=User:(.*),\shost=(.*),\soperation=(.*),\spermissionType=(.*)\)$`)
	reName := regexp.MustCompile(`.*resourceType=(.*),\sname=(.*),\s*patternType=(.*)\).*`)
	acls := make([]ACL, 0)
	var acl ACL
	for _, l := range lines {
		line := strings.TrimSpace(l)
		if reUser.MatchString(line) {
			as := reUser.FindStringSubmatch(line)
			if len(as) == 5 {
				acl.updateAcl(as[1], as[2], as[3], as[4])
			}
		} else if reName.MatchString(line) {
			as := reName.FindStringSubmatch(line)
			if len(as) != 4 {
				continue
			}
			if acl.topic != "" {
				acls = append(acls, acl)
			}
			acl = ACL{topic: as[2], rtype: as[1], ptype: as[3], perms: make([]PERM, 0)}
		}
	}
	if acl.topic != "" {
		acls = append(acls, acl)
	}
	return acls
}
