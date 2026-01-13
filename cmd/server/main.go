package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ballbattle/internal/server"
)

func main() {
	var listen string
	var hz int
	var foodCount int
	var arenaSize float64
	flag.StringVar(&listen, "listen", ":30000", "UDP listen addr")
	flag.IntVar(&hz, "hz", 60, "tick rate")
	flag.IntVar(&foodCount, "foods", 120, "number of food pellets")
	flag.Float64Var(&arenaSize, "size", 100, "arena half-size (square from -size..size)")
	flag.Parse()

	srv, err := server.New(listen, hz, foodCount, float32(arenaSize))
	if err != nil {
		log.Fatalf("create server: %v", err)
	}

	// 使用新版 netcore.Start 一次性启动所有循环
	srv.Start()
	defer srv.Close()

	log.Printf("ballbattle server started on %s (hz=%d, foods=%d, size=%.1f)", listen, hz, foodCount, arenaSize)

	// graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
	log.Println("server exiting")
}
