package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: vlx [status|prune]")
		return
	}

	command := os.Args[1]

	switch command {
	case "status":
		resp, err := http.Get("http://localhost:8080/health")
		if err != nil {
			fmt.Printf("Error connecting to Velarix server: %v\n", err)
			return
		}
		defer resp.Body.Close()
		fmt.Println("Velarix server is running.")
	case "prune":
		fmt.Println("Pruning stale facts... (Implementation stub)")
	default:
		fmt.Printf("Unknown command: %s\n", command)
	}
}
