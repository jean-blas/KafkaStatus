package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

type NAMESPACE struct {
	Ns           v1.Namespace
	Pods         v1.PodList
	TopicDetails []topicDetails
	Groups       []GROUP
	Mm2s         []MIRRORMAKER2
}

type MIRRORMAKER2 struct {
	ConnectCluster string
	Mirrors        [][]string
}

var Namespaces []NAMESPACE

// **************** CONNECTION *****************************
var clientset *kubernetes.Clientset
var config *restclient.Config
var dynset dynamic.Interface

func getConfigOrDie() {
	if strings.TrimSpace(kubeconfig) == "" {
		home, _ := homedir.Dir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	var err error = nil
	// use the current context in kubeconfig
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	log.Debug("Config : " + fmt.Sprintf("%v\n", config))
}

// Code working if connected through "oc login" because BearerToken is filled
func getClientsetOrDie() {
	var err error
	getConfigOrDie()
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("error getting Kubernetes clientset: %v\n", err)
		os.Exit(1)
	}
	log.Debug("Clientcmd : " + fmt.Sprintf("%v\n", *clientset))
}

func (n NAMESPACE) Name() string {
	return n.Ns.ObjectMeta.Name
}

// Get all pods for all namespaces
func getPodsAllNs() {
	for i := range Namespaces {
		Namespaces[i].getPods()
	}
}

// Get all the pods of a given namespace
func (n *NAMESPACE) getPods() {
	pods, err := clientset.CoreV1().Pods(n.Ns.ObjectMeta.Name).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error getting pods in %s: %v\n", n.Name(), err)
		os.Exit(1)
	}
	if log.GetLevel() == log.DebugLevel {
		for _, pod := range pods.Items {
			log.Debug("Pod name: " + pod.Name)
		}
	}
	n.Pods = *pods
}

// Populate the variable Namespaces with the ones from ans, or all kafka namespaces if ans is empty
func getKafkaNs(ans []string) {
	ns, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error getting namespaces: %v\n", err)
		os.Exit(1)
	}
	Namespaces = make([]NAMESPACE, 0)
	log.Debug("Namespaces:")
	for _, item := range ns.Items {
		name := item.ObjectMeta.Name
		if (strings.HasPrefix(name, "kafka") || strings.HasSuffix(name, "kafka")) && !strings.Contains(name, "-admin") {
			if ans == nil || inArray(ans, name) {
				Namespaces = append(Namespaces, NAMESPACE{Ns: item})
			}
			log.Debug("\t" + item.ObjectMeta.Name)
		}
	}
}

func execToPod(command, containerName, podName, _namespace string, stdin io.Reader) (string, string, error) {
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(_namespace).SubResource("exec")
	scheme := runtime.NewScheme()
	if err := v1.AddToScheme(scheme); err != nil {
		return "", "", fmt.Errorf("error adding to scheme: %v", err)
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&v1.PodExecOptions{
		Command:   strings.Fields(command),
		Container: containerName,
		Stdin:     stdin != nil,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	log.Debug(fmt.Sprintf("Request URL: %s", req.URL().String()))

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("error while creating Executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return "", "", fmt.Errorf("error in Stream: %v", err)
	}

	return stdout.String(), stderr.String(), nil
}

// Get the kafka pods only
func (n NAMESPACE) getKafkaPods() []v1.Pod {
	pods := make([]v1.Pod, 0)
	reKZ := regexp.MustCompile(`^(\S{4})(\d{2,3})(-kafka|-zookeeper)-(\d{1,2})$`)
	for _, p := range n.Pods.Items {
		if reKZ.MatchString(p.Name) {
			pods = append(pods, p)
		}
	}
	return pods
}

// Get custom resource dynamically

func getDynamicClientOrDie() {
	getConfigOrDie()
	dynset = dynamic.NewForConfigOrDie(config)
}

func GetResourcesDynamically(group, version, resource, namespace string) ([]unstructured.Unstructured, error) {
	resourceId := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
	list, err := dynset.Resource(resourceId).Namespace(namespace).List(context.Background(), metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	return list.Items, nil
}
