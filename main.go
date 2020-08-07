package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"labirong3d.com/server/network"
)

var addr = flag.String("addr", ":8080", "http service address")

func main() {
	flag.Parse()
	hub := network.NewHub()
	go hub.Run()

	_, isPortSet := os.LookupEnv("PORT")
	if isPortSet {
		*addr = ":" + os.Getenv("PORT")
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		network.ServeWs(hub, w, r)
	})

	fmt.Println("Server running at: http://localhost" + *addr)
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
