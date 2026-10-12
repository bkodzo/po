// po-train builds the small student model that ships inside Po. It is a
// development tool and is not installed with Po.
//
//	po-train gen    write simulated work sessions
//	po-train label  have the large model label them
//	po-train fit    train the student on those labels and report how it does
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "gen":
		runGen(os.Args[2:])
	case "label":
		runLabel(os.Args[2:])
	case "fit":
		runFit(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: po-train gen|label|fit [flags]")
	os.Exit(2)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "po-train:", err)
	os.Exit(1)
}
