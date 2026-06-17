package main

import (
	"fmt"
	"log"
	"os"

	"github-release-notifier/internal/infrastructure/certgen"
)

func main() {
	dir := "certs"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	if err := certgen.Write(dir); err != nil {
		log.Fatalf("gencerts: %v", err)
	}

	fmt.Printf("wrote ca.crt and subscription/scanner .crt/.key to %s\n", dir)
}
