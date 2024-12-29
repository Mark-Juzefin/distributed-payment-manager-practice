package main

import "log"

func main() {
	server := InitServer()
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
