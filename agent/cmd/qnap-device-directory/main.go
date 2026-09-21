package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"qnap-ai-control-suite/agent/internal/devicedirs"
)

func main() {
	root := flag.String("root", "", "fixed device inbox root")
	flag.Parse()
	if flag.NArg() != 0 {
		os.Exit(2)
	}
	var request devicedirs.Request
	decoder := json.NewDecoder(io.LimitReader(os.Stdin, 4097))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		fmt.Fprintln(os.Stderr, "invalid_request")
		os.Exit(2)
	}
	var trailing interface{}
	if decoder.Decode(&trailing) != io.EOF {
		fmt.Fprintln(os.Stderr, "invalid_request")
		os.Exit(2)
	}
	receipt, err := devicedirs.Ensure(*root, request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "directory_unverified")
		os.Exit(1)
	}
	if json.NewEncoder(os.Stdout).Encode(receipt) != nil {
		os.Exit(1)
	}
}
