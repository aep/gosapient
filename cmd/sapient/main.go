package main

import (
	"fmt"
	"os"
)

var commands = map[string]func([]string){
	"server": serverCmd,
	"peer":   peerCmd,
	"sensor": sensorCmd,
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd, ok := commands[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}

	cmd(os.Args[2:])
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: sapient <command> [args]\n\nCommands:\n")
	fmt.Fprintf(os.Stderr, "  server   Listen for SAPIENT connections and print messages\n")
	fmt.Fprintf(os.Stderr, "  peer     Connect to middleware as Peer and print messages\n")
	fmt.Fprintf(os.Stderr, "  sensor   Simulate a sensor edge node sending detections\n")
}
