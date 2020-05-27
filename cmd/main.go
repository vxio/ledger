package main

import (
	"log"

	"proglog/internal/server"
)

func main() {
	s := server.NewHTTPServer(":8080")
	log.Fatal(s.ListenAndServe())

}
