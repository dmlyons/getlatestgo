package main

import (
	"log"
	"os"

	"github.com/dmlyons/getlatestgo/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		log.Fatal(err)
	}
}
