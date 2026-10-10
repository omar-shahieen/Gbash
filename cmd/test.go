package main

import (
	"fmt"
	"sync"
	"time"
)

func worker(id int, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Printf("Worker %d started\n", id)
	time.Sleep(time.Second)
	fmt.Printf("Worker %d finished\n", id)
}

func main() {
	var wg sync.WaitGroup

	// Tell the WaitGroup to wait for 3 tasks.
	wg.Add(3)

	go worker(1, &wg)
	go worker(2, &wg)
	go worker(3, &wg)

	// Block until all 3 tasks finish.
	wg.Wait()

	fmt.Println("All workers finished")
}
