package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	log.SetFlags(0)

	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

// run starts a http.Server for the passed in address
// with all requests handled by chatServer.
func run() error {
	listener, err := net.Listen("tcp", ":8081")
	if err != nil {
		return err
	}
	log.Printf("listening on %v", listener.Addr())

	s := &http.Server{
		Handler: chatServer{
			logf: log.Printf,
		},
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}
	log.Printf("Server created")
	errc := make(chan error, 1)
	go func() {
		errc <- s.Serve(listener)
	}()
	// create a new handler
	handler := HttpHandler{}
	go func() {
		errc <- http.ListenAndServe(":80", handler)
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select {
	case err := <-errc:
		log.Printf("failed to serve: %v", err)
	case sig := <-sigs:
		log.Printf("terminating: %v", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	return s.Shutdown(ctx)
}

// create a handler struct
type HttpHandler struct{}

// implement `ServeHTTP` method on `HttpHandler` struct
func (h HttpHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	file, err := os.ReadFile("index.html")
	if err != nil {
		log.Println(err.Error())
		return
	}
	res.Write(file)
}
