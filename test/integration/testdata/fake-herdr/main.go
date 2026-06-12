package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "fake-herdr: unknown command %v\n", os.Args[1:])
		os.Exit(1)
	}
	sub1, sub2 := os.Args[1], os.Args[2]
	switch sub1 {
	case "workspace":
		if sub2 == "create" {
			fmt.Println(`{"result":{"workspace":{"workspace_id":"t1"},"root_pane":{"pane_id":"t1-1"}}}`)
			return
		}
	case "agent":
		switch sub2 {
		case "list":
			fmt.Println(`{"result":{"agents":[]}}`)
			return
		case "start":
			fmt.Println(`{"result":{"agent":{"pane_id":"t1-2"}}}`)
			return
		}
	case "wait":
		if sub2 == "output" {
			result := os.Getenv("FAKE_HERDR_WAIT_RESULT")
			if result == "" {
				result = `{"result":{"matched_line":"RALPH_DONE:0"}}`
			}
			fmt.Println(result)
			return
		}
	case "pane":
		if sub2 == "rename" || sub2 == "close" || sub2 == "run" {
			fmt.Println(`{}`)
			return
		}
	}
	fmt.Fprintf(os.Stderr, "fake-herdr: unknown command %s\n", strings.Join(os.Args[1:], " "))
	os.Exit(1)
}
