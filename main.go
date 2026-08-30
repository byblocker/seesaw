package main

import (
	"log"
	"seesaw/cmd"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	cmd.Execute()
}
