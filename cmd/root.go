package cmd

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile, clustername, brokername string

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
		fmt.Println(err)
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
	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
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
			fmt.Println(err)
			os.Exit(1)
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
}

// Convert a cluster name into a broker name
func bootstrap(clustername string) (string, error) {
	if strings.TrimSpace(clustername) == "" {
		return "", errors.New("bootstrap : cluster name is not defined")
	}
	re := regexp.MustCompile(`b[k|z].[0-9]{2}`)
	suffixe := ".os.amadeus.net:9092"
	if re.MatchString(clustername) {
		broker := clustername[:3] + "v" + clustername[3:]
		res := broker + "00" + suffixe
		for i := 1; i < 3; i++ {
			res += fmt.Sprintf(",%s%02d%s", broker, i, suffixe)
		}
		return res, nil
	}

	return "", errors.New("bootstrap [" + clustername + "] : bad format for cluster name")
}

// topic should be using comma as separator (e.g. topic1,topic2)
func extractTopics(topic string) []string {
	return strings.Split(topic, ",")
}

func splitClustername(clustername string) []string {
	return strings.Split(clustername, ",")
}
