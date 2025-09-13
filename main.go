package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Broker struct {
	sockets map[*websocket.Conn]struct{}
	sync.Mutex
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  128,
	WriteBufferSize: 128,
}

var broker Broker

//go:embed index.html
var static embed.FS

func actionBroker(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to fulfill client request for playPauseBroker: %v", err)
		return
	}

	broker.Lock()
	broker.sockets[conn] = struct{}{}
	log.Printf("broker currently handles %d clients", len(broker.sockets))
	broker.Unlock()

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("failed to read message from client %p: %v", conn, err)
			log.Printf("will attempt to close connection %p", conn)
			if err := conn.Close(); err != nil {
				log.Printf("failed to close connection at %p", conn)
			}
			delete(broker.sockets, conn)
			return
		}

		for client := range broker.sockets {
			if client == conn {
				continue // prevent self echo
			}

			if err := client.WriteMessage(messageType, p); err != nil {
				log.Printf("while sending: %s to client %p: %v", p, client, err)
			}
			log.Printf("successfully sent %s", p)
		}
	}
}

func main() {
	listenPort := flag.Uint("port", 8000, "port to listen on")
	flag.Parse()
	streamFile := flag.Arg(0)
	if streamFile == "" {
		log.Fatal("stream file is undefined")
	}
	broker.sockets = make(map[*websocket.Conn]struct{})
	http.Handle("/stream/", http.StripPrefix("/stream/", http.FileServer(http.Dir(streamFile))))
	http.HandleFunc("/broker", actionBroker)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, static, "index.html")
	})

	panic(http.ListenAndServe(fmt.Sprintf(":%d", *listenPort), nil))
}
