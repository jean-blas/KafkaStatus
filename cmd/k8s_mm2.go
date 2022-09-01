package cmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/itchyny/gojq"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var kMm2Cmd = &cobra.Command{
	Use:   "kmm2",
	Short: "[PaaS] Display MirrorMaker2 info inside a PaaS",

	Run: func(cmd *cobra.Command, args []string) {
		getClientsetOrDie()
		if namespace == "" {
			getKafkaNs(nil)
		} else {
			getKafkaNs(strings.Split(namespace, ","))
		}
		getDynamicClientOrDie()
		fillMM2Namespaces()
		for _, namespace := range Namespaces {
			fmt.Printf("%s:\n", namespace.Name())
			for _, mm2 := range namespace.Mm2s {
				printMirrors(mm2)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(kMm2Cmd)
	// Cobra supports local flags which will only run when this command is called directly, e.g.:
}

var keys = [...]string{".sourceCluster", ".topicsPattern", ".topicsBlacklistPattern"}

func printMirrors(m MIRRORMAKER2) {
	fmt.Printf("  %s <- ", m.ConnectCluster)
	l := len(keys)
	for _, m := range m.Mirrors {
		fmt.Printf("%s [ ", m[0])
		for i := 1; i < l; i++ {
			if m[i] != "" && keys[i] == ".topicsBlacklistPattern" {
				fmt.Printf("BLACKLIST=%s ", m[i])
			} else {
				fmt.Printf("%s ", m[i])
			}
		}
		fmt.Printf("]    ")
	}
	fmt.Println(" ")
}

func fillMM2Namespaces() {
	var wg sync.WaitGroup
	for i := range Namespaces {
		wg.Add(1)
		go func(i int) {
			items, err := GetResourcesDynamically("kafka.strimzi.io", "v1beta2", "kafkamirrormaker2s", Namespaces[i].Name())
			if logErr(err) {
				wg.Done()
				return
			}
			if log.GetLevel() == log.DebugLevel || log.GetLevel() == log.TraceLevel {
				for _, item := range items {
					fmt.Printf("%+v\n", item)
				}
			}
			mm2, err := queryMM2(items)
			if logErr(err) {
				wg.Done()
				return
			}
			Namespaces[i].Mm2s = mm2
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func queryMM2(items []unstructured.Unstructured) ([]MIRRORMAKER2, error) {
	mm2 := make([]MIRRORMAKER2, 0)
	qConnectCluster := parseOrDie(".spec.connectCluster")
	for _, item := range items {
		// Convert object to raw JSON
		var rawJson interface{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &rawJson)
		if err != nil {
			return nil, err
		}
		// Evaluate jq against JSON
		connectCluster, err := getJqString(qConnectCluster, rawJson)
		if err != nil {
			return nil, err
		}
		mirrors, err := getJqMirrors(rawJson)
		if err != nil {
			return nil, err
			// printMirrors(connectCluster, mirrors)
		}
		mm2 = append(mm2, MIRRORMAKER2{ConnectCluster: connectCluster, Mirrors: mirrors})
	}
	return mm2, nil
}

func parseOrDie(query string) *gojq.Query {
	q, err := gojq.Parse(query)
	logFatal(err)
	return q
}

func getJqString(query *gojq.Query, rawJson interface{}) (string, error) {
	iter := query.Run(rawJson)
	for {
		result, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := result.(error); ok {
			if err != nil {
				return "", err
			}
		} else if str, ok := result.(string); ok {
			return str, nil
		}
	}
	return "", nil
}

func getJqMirrors(rawJson interface{}) ([][]string, error) {
	res := make([][]string, 0)
	mkeys := make(map[string]*gojq.Query)
	for _, k := range keys {
		mkeys[k] = parseOrDie(k)
	}
	qMirrors := parseOrDie(".spec.mirrors[]")
	mirrors := qMirrors.Run(rawJson)
	for {
		mirror, ok := mirrors.Next()
		if !ok {
			break
		}
		if err, ok := mirror.(error); ok {
			if err != nil {
				return nil, err
			}
		} else {
			m := make([]string, 3)
			for i, key := range keys {
				s, err := getJqString(mkeys[key], mirror)
				if err != nil {
					return nil, err
				}
				m[i] = s
			}
			res = append(res, m)
		}
	}
	return res, nil
}
