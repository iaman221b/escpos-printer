// Command list enumerates every printer this machine can see.
//
//	go run ./examples/list
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	escposprinter "github.com/iaman221b/escpos-printer"
	"github.com/iaman221b/escpos-printer/transport/network"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reg := escposprinter.New(
		// Network discovery is off unless asked for: sweeping a network
		// unprompted is rude and can look like a port scan.
		escposprinter.WithNetwork(network.Config{SweepSubnet: false}),
	)

	fmt.Printf("discovery sources: %v\n\n", reg.Sources())

	devices, err := reg.Discover(ctx)
	if err != nil {
		// Partial failure: some sources answered, some did not. The devices
		// that were found are still usable.
		fmt.Fprintf(os.Stderr, "some sources failed: %v\n\n", err)
	}

	if len(devices) == 0 {
		fmt.Println("no printers found")
		return
	}

	active := reg.ActiveID()
	for _, d := range devices {
		marker := " "
		if d.ID == active {
			marker = "*"
		}
		receipt := ""
		if d.Receipt {
			receipt = "  [receipt printer]"
		}
		fmt.Printf("%s %-40s %s\n", marker, d.ID, d.Name)
		fmt.Printf("    %s via %s%s\n", d.Connection, d.Transport, receipt)
		if d.Detail != "" {
			fmt.Printf("    %s\n", d.Detail)
		}
		fmt.Println()
	}

	if active == "" {
		fmt.Println("nothing selected — more than one candidate, so pick one explicitly")
	}
}
