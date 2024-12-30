package main

import "log"

func main() {
	server, err := InitServer()
	if err != nil {
		log.Fatal(err)
	}
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
