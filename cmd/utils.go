package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
)

// **************** CLUSTER / BROKERS / TOPICS *****************************

type GROUP struct {
	name, state       string
	describe, members []string
}

type SERVER struct {
	cluster, bootstrap, topics string
	groups                     []GROUP
}

// Construct the struct of bootstrap servers for each cluster
func buildServers() ([]SERVER, error) {
	tpcs := make([]SERVER, 0)
	fqdn := brokername
	if fqdn == "" {
		clusternames := strings.Split(clustername, ",")
		for _, c := range clusternames {
			server, err := clusterToBootstrap(c)
			if err != nil {
				return nil, err
			}
			tpcs = append(tpcs, SERVER{cluster: c, bootstrap: server})
		}
	} else {
		cluster, err := toCluster(fqdn)
		if err != nil {
			return nil, err
		}
		tpcs = append(tpcs, SERVER{cluster: cluster, bootstrap: fqdn})
	}
	return tpcs, nil
}

// Convert a cluster name into a broker name (e.g. bku10 => bkuv1000.os.amadeus.net:9092,bku1001.os.amadeus.net:9092,bku1002.os.amadeus.net:9092)
func clusterToBootstrap(clustername string) (string, error) {
	if strings.TrimSpace(clustername) == "" {
		return "", errors.New("bootstrap : cluster name is not defined")
	}
	re := regexp.MustCompile(`b[k|z].[0-9]{2}`)
	suffixe := ".os.amadeus.net:9092"
	if re.MatchString(clustername) {
		log.Debug("Computing 3 brokers for cluster " + clustername + " on port 9092")
		broker := clustername[:3] + "v" + clustername[3:]
		res := broker + "00" + suffixe
		for i := 1; i < 5; i++ {
			res += fmt.Sprintf(",%s%02d%s", broker, i, suffixe)
		}
		return res, nil
	}

	return "", errors.New("bootstrap [" + clustername + "] : bad format for cluster name")
}

func toCluster(brokers string) (string, error) {
	broker := strings.Split(brokers, ",")
	re := regexp.MustCompile(`b[k|z][p|t|g|c|u|x]v[0-9]{4}\.os\.amadeus\.net:[0-9]{4}`)
	if re.MatchString(broker[0]) {
		b := broker[0]
		return b[:3] + b[4:6], nil
	}
	return "", errors.New("Bad format for bootstrap servers; should be of the form fqdn:port (e.g. bkuv1000.os.amadeus.net:9092)")
}

// ********** CONNECTION CHECKING *************************************

// Kind of telnet to the host:port
func raw_connect(host, port string) (bool, error) {
	log.Debug(fmt.Sprintf("Trying to connect to %s : %s in %d millis", host, port, timeout))
	_timeout := time.Duration(timeout) * time.Millisecond
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), _timeout)
	if err != nil {
		return false, err
	} else {
		defer conn.Close()
		log.Debug("Opened ", net.JoinHostPort(host, port))
		return true, nil
	}
}

// Try to connect to each broker of the given (boostrap)servers to check the connection and port listening
func check_conn(servers string) error {
	log.Debug(fmt.Sprintf("Checking connection of %s", servers))
	vms := strings.Split(servers, ",")
	con := false
	var err error
	for _, vm := range vms {
		hp := strings.Split(vm, ":")
		if len(hp) != 2 {
			return errors.New("Bad format : broker should be of the form fqdn:port (e.g. bkuv1000.os.amadeus.net:9092)")
		}
		con, err = raw_connect(hp[0], hp[1])
		if con {
			break
		}
	}
	if !con {
		return err
	}
	return nil
}

// **************** LOGGING AND ERRORS *****************************************

func logFatal(err ...error) {
	for _, e := range err {
		if e != nil {
			log.Fatal(e)
		}
	}
}

func logErr(err ...error) bool {
	ok := false
	for _, e := range err {
		if e != nil {
			log.Error(e)
			ok = true
		}
	}
	return ok
}

// ****************** GIT ******************************************************
const (
	ansible_config = "https://rndwww.nce.amadeus.net/git/scm/kafka/ansible-configs.git"
)

var branchs = [...]string{"ERDING_DEV", "ERDING_PRD", "ERDING_TL1", "ERDING_TL2", "ERDING_DES", "ERDING_STG"}

func askCredentials() {
	if strings.TrimSpace(gitLogin) == "" {
		fmt.Println("Enter you git login:")
		fmt.Scanln(&gitLogin)
	}
	if strings.TrimSpace(gitPasswd) == "" {
		fmt.Println("Enter you git password:")
		bytepw, err := term.ReadPassword(int(syscall.Stdin))
		logFatal(err)
		gitPasswd = string(bytepw)
	}
}

func cloneInMemory(branch string) (billy.Filesystem, error) {
	askCredentials()
	fs := memfs.New()
	log.Debug("Cloning " + gitRepo + " : " + branch)
	_, err := git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		Auth: &http.BasicAuth{
			Username: gitLogin,
			Password: gitPasswd,
		},
		URL:           gitRepo,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
	})
	if err != nil {
		return nil, err
	}
	return fs, nil
}

func printRepo(fs billy.Filesystem) {
	arr, err := fs.ReadDir("/")
	logFatal(err)
	for i, a := range arr {
		fmt.Println(i, ":", a.Name(), ",", a.Size(), ",", a.Mode(), ":", a.IsDir())
		if !a.IsDir() {
			file, err := fs.Open(a.Name())
			logFatal(err)
			lines, err := load(file)
			logFatal(err)
			fmt.Println(lines)
		}
	}
}

// Build the inventory of all Kafka clusters
func buildInventory(fs billy.Filesystem) ([]string, error) {
	arr, err := fs.ReadDir("/")
	if err != nil {
		return nil, err
	}
	inv := make([]string, 0)
	for _, a := range arr {
		re := regexp.MustCompile(`b[k][p|t|g|c|u|x][0-9]{2}`)
		if re.MatchString(a.Name()) {
			inv = append(inv, a.Name())
		}
	}
	log.Debug("Inventory : " + strings.Join(inv, ","))
	return inv, nil
}

//Load a file into a slice of strings
//Suppress empty lines
func load(filename billy.File) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(filename)
	var line string
	for scanner.Scan() {
		line = scanner.Text()
		if line == "" { // suppress empty lines
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
