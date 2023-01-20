package main

import "sync"

type Fan[I any, O any] struct {
	InputWorker   func(chan<- I)
	ProcessWorker func(<-chan I, chan<- O)
	OutputWorker  func(<-chan O)
	Workers       int
	InputBuffer   int
	OutputBuffer  int
}

func (f Fan[I, O]) Run() {
	var wg sync.WaitGroup
	wg.Add(1 + f.Workers)

	inputChan := make(chan I, f.InputBuffer)
	outputChan := make(chan O, f.OutputBuffer)

	go func() {
		f.InputWorker(inputChan)
		wg.Done()
	}()
	for i := 0; i < f.Workers; i++ {
		go func() {
			f.ProcessWorker(inputChan, outputChan)
			wg.Done()
		}()
	}
	var dbWg sync.WaitGroup
	dbWg.Add(1)
	go func() {
		f.OutputWorker(outputChan)
		dbWg.Done()
	}()
	wg.Wait()
	close(outputChan)
	dbWg.Wait()
}
