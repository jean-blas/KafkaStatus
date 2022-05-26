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

//TODO check the connection (when the bkpv0000/1/2 are dead, can not connect)
