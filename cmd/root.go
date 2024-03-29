package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var cfgFile, clustername, brokername, topics, groups string
var logLevel string
var gitRepo, gitBranch, login, passwd string
var short bool
var timeout, httpTimeout int
var invFile string
var kubeconfig, namespace string

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
	rootCmd.PersistentFlags().StringVarP(&invFile, "inv", "", "", "Input ansible-like inventory file ")
	rootCmd.PersistentFlags().StringVarP(&brokername, "broker", "b", "", "Broker full name (e.g. bkuv1000.os.amadeus.net:9092)")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log", "l", "warn", "log level (e.g. trace, debug, info, warn, error, fatal)")
	rootCmd.PersistentFlags().StringVarP(&gitRepo, "git-repo", "", ansible_config, "git repository to clone")
	rootCmd.PersistentFlags().StringVarP(&gitBranch, "git-branch", "", "", "git branch to checkout (e.g. ERDING_TL1)")
	rootCmd.PersistentFlags().StringVarP(&login, "login", "u", "", "login")
	rootCmd.PersistentFlags().StringVarP(&passwd, "passwd", "w", "", "password")
	rootCmd.PersistentFlags().BoolVarP(&short, "short", "s", false, "When available, display only a short version of the results")
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "", 500, "Timeout used when checking the connection (milliseconds)")
	rootCmd.PersistentFlags().IntVarP(&httpTimeout, "http-timeout", "", 2000, "Timeout used when sending a request (milliseconds)")

	rootCmd.PersistentFlags().StringVarP(&kubeconfig, "kconfig", "", "", "Absolute path to the kubeconfig file")
	rootCmd.PersistentFlags().StringVarP(&namespace, "ns", "", "", "Namespace names using comma as separator (e.g. namespace1,namespace2)")
	rootCmd.PersistentFlags().StringVarP(&topics, "topic", "t", "", "Topic names using comma as separator (e.g. topic1,topic2)")
	rootCmd.PersistentFlags().StringVarP(&groups, "group", "g", "", "Groups to describe (separator is comma for several groups)")

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
