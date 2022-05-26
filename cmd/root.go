package cmd

import (
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var cfgFile, clustername, brokername string
var logLevel string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kstat",
	Short: "A simple CLI to check Kafka clusters",
	Long:  `When you want some info on Kafka servers, just call this API`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.task.yaml)")
	rootCmd.PersistentFlags().StringVarP(&clustername, "cluster", "c", "", "Cluster name (e.g. bku10)")
	rootCmd.PersistentFlags().StringVarP(&brokername, "broker", "b", "", "Broker full name (e.g. bkuv1000.os.amadeus.net:9092)")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log", "l", "warn", "log level (trace, debug, info, warn, error, fatal)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}

func logSetLevel() {
	switch logLevel {
	case "trace":
		log.SetLevel(log.TraceLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			log.Fatal(err)
		}

		// Search config in home directory with name ".task" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".kstat")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}

	logSetLevel()
}

// Convert a cluster name into a broker name (e.g. bku10 => bkuv1000.os.amadeus.net:9092,bku1001.os.amadeus.net:9092,bku1002.os.amadeus.net:9092)
func bootstrap(clustername string) (string, error) {
	if strings.TrimSpace(clustername) == "" {
		return "", errors.New("bootstrap : cluster name is not defined")
	}
	re := regexp.MustCompile(`b[k|z].[0-9]{2}`)
	suffixe := ".os.amadeus.net:9092"
	if re.MatchString(clustername) {
		log.Debug("Computing 3 brokers for cluster " + clustername + " on port 9092")
		broker := clustername[:3] + "v" + clustername[3:]
		res := broker + "00" + suffixe
		for i := 1; i < 3; i++ {
			res += fmt.Sprintf(",%s%02d%s", broker, i, suffixe)
		}
		return res, nil
	}

	return "", errors.New("bootstrap [" + clustername + "] : bad format for cluster name")
}

func toCluster(brokers string) (string, error) {
	broker := strings.Split(brokers, ",")
	re := regexp.MustCompile(`b[k|z][p|t|g|c|u|x]v[0-9]{4}\\.os\\.amadeus\\.net:[0-9]{4}`)
	if re.MatchString(broker[0]) {
		b := broker[0]
		return b[:3] + b[4:5], nil
	}
	return "", errors.New("Bad format for bootstrap servers; should be of the form fqdn:port (e.g. bkuv1000.os.amadeus.net:9092)")
}

// topic should be using comma as separator (e.g. topic1,topic2)
func extractTopics(topic string) []string {
	return strings.Split(topic, ",")
}

// clusters should be using comma separator (e.g. bku10,bku11,bku12)
func splitClustername(clustername string) []string {
	return strings.Split(clustername, ",")
}

// Kind of telnet to the host:port
func raw_connect(host, port string) (bool, error) {
	log.Debug("Trying to connect to " + host + ":" + port)
	timeout := 500 * time.Millisecond
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
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
