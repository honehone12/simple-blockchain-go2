package main

import (
	"log"
	"simple-blockchain-go2/cli"
)

func main() {
	err := cli.Run()
	if err != nil {
		log.Panic(err)
	}
}
