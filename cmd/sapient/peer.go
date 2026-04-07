package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"sapient/pkg/sapient"

	"google.golang.org/protobuf/encoding/protojson"
)

func peerCmd(args []string) {
	fs := flag.NewFlagSet("peer", flag.ExitOnError)
	addr := fs.String("addr", "localhost:5001", "middleware peer address")
	json := fs.Bool("json", false, "print messages as JSON")
	fs.Parse(args)

	conn, err := sapient.Dial(*addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	log.Printf("connected to %s as peer", *addr)

	for {
		msg, err := conn.Recv()
		if err != nil {
			if sapient.IsConnectionClosed(err) {
				log.Printf("disconnected")
			} else {
				log.Printf("recv error: %v", err)
			}
			return
		}

		ct := sapient.ContentType(msg)
		from := msg.GetNodeId()

		if *json {
			out, _ := protojson.MarshalOptions{
				Multiline: true,
				Indent:    "  ",
			}.Marshal(msg)
			fmt.Printf("%s from %s:\n%s\n", ct, from, out)
		} else {
			fmt.Printf("%s from=%s %s\n", ct, from, summarize(msg))
		}
	}
}
