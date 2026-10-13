package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Create a channel to receive signals.
	sigChan := make(chan os.Signal, 1)

	// Register the signals we want to listen for.
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	fmt.Println("Waiting for signals...")

	// Block until a signal arrives.
	sig := <-sigChan

	switch sig {
	case os.Interrupt:
		fmt.Println("Received Ctrl+C")
	case syscall.SIGTERM:
		fmt.Println("Received termination request")
	}

}
