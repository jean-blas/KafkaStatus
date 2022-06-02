package main

import (
	"flag"
	"fmt"
	"kafkaStatus/cmd"
	"os"
)

var Usage = func() {
	fmt.Fprintf(flag.CommandLine.Output(), "%s: utility program used to check some kafka cluster health\n\n", os.Args[0])
	fmt.Fprintln(flag.CommandLine.Output(), "kstat [options] clustername")
	fmt.Fprintf(flag.CommandLine.Output(), "\nUsage of %s:\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = Usage

	cmd.Execute()
}

// TODO : check brokers load in terms of balance of partitions, etc.
// TODO : check zookeeper
// TODO : check the partition size, etc. kafka-log-dirs.sh --bootstrap-server bktv0900.os.amadeus.net:9092 --describe --broker-list 0
// TODO : get the equivalent of kafka-cluster-status
