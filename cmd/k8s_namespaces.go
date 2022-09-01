package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var kNamespaceCmd = &cobra.Command{
	Use:   "namespace",
	Short: "[PaaS] Display namespace info",

	Run: func(cmd *cobra.Command, args []string) {
		getClientsetOrDie()
		if namespace == "" {
			getKafkaNs(nil)
		} else {
			getKafkaNs(strings.Split(namespace, ","))
		}
		if short {
			for _, item := range Namespaces {
				fmt.Println(item.Ns.ObjectMeta.Name)
			}
			return
		}
		getPodsAllNs()
		printNsShort()
	},
}

func init() {
	rootCmd.AddCommand(kNamespaceCmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
}

func printNsShort() {
	reKZ := regexp.MustCompile(`^(\S{4})(\d{2,3})(-kafka|-zookeeper)-(\d{1,2})$`)
	reKZother := regexp.MustCompile(`^(\S*)(-kafka|-zookeeper)-(\d{1,2})$`)
	reMM2 := regexp.MustCompile(`^(\S*)(-mirrormaker2)(.*)-(\S{5})$`)
	for i := range Namespaces {
		fmt.Println(Namespaces[i].Ns.ObjectMeta.Name)
		maxLen := 0
		m := make(map[string][]string)
		for _, pod := range Namespaces[i].Pods.Items {
			if reKZ.MatchString(pod.Name) {
				as := reKZ.FindStringSubmatch(pod.Name)
				name := as[1] + as[2] + as[3]
				maxLen = max(maxLen, len(name))
				ai := m[name]
				ai = append(ai, as[4])
				m[name] = ai
			} else if reMM2.MatchString(pod.Name) {
				as := reMM2.FindStringSubmatch(pod.Name)
				name := as[1]
				maxLen = max(maxLen, len(name))
				ai := m[name]
				ai = append(ai, as[4])
				m[name] = ai
			} else if reKZother.MatchString(pod.Name) {
				as := reKZother.FindStringSubmatch(pod.Name)
				name := as[1] + as[2]
				maxLen = max(maxLen, len(name))
				ai := m[name]
				ai = append(ai, as[3])
				m[name] = ai
			}
		}
		keys := sortMapKeys(m)
		for _, k := range keys {
			fmt.Printf("\t%-*s  %s \n", maxLen, k, m[k])
		}
	}
}
