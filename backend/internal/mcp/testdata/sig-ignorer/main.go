package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		for range c {
		}
	}() // swallow
	time.Sleep(60 * time.Second)
}
