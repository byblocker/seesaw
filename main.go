package main

import (
	"log"
	"seesaw/cmd"
)

func main() {
	log.SetFlags(log.Ltime)
	cmd.Execute()
}
