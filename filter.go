package main

import (
	"github.com/rs/zerolog/log"
)

type Filter[I, O any] interface {
	UnprocessedInstances(db *Database) ([]I, error)
	ProcessInstance(*I) ([]O, error)
	SetProcessed(*Database, *I) error
	SaveResult(*Database, []O) error
}

type FilterResult[I, O any] struct {
	Input  I
	Output []O
	Error  error
}

func FilterInputWorker[I, O any](filter Filter[I, O], instanceChan chan<- I) {
	db, err := NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not create database")
	}
	potentialInstances, err := filter.UnprocessedInstances(&db)
	db.Close()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get unprocessed instances")
	}
	log.Info().Int("count", len(potentialInstances)).Msg("fetched unprocessed instances")
	for _, instance := range potentialInstances {
		instanceChan <- instance
	}
	close(instanceChan)
}

func FilterProcessWorker[I, O any](filter Filter[I, O], instanceChan <-chan I, resultChan chan<- FilterResult[I, O]) {
	for {
		instance, ok := <-instanceChan
		if !ok {
			return
		}
		result, err := filter.ProcessInstance(&instance)
		resultChan <- FilterResult[I, O]{
			Input:  instance,
			Output: result,
			Error:  err,
		}
	}
}

func FilterOutputWorker[I, O any](filter Filter[I, O], resultsChan <-chan FilterResult[I, O]) {
	db, err := NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not create database")
	}
	for {
		result, ok := <-resultsChan
		if !ok {
			return
		}
		if result.Error == nil && len(result.Output) > 0 {
			err = filter.SaveResult(&db, result.Output)
			if err != nil {
				log.Fatal().Err(err).Msg("could not save result")
			}
		}
		err = filter.SetProcessed(&db, &result.Input)
		if err != nil {
			log.Fatal().Err(err).Msg("could not set instance processed")
		}
	}

}

func RunFilter[I, O any](filter Filter[I, O], workers int) {
	Fan[I, FilterResult[I, O]]{
		InputWorker: func(inputChan chan<- I) {
			FilterInputWorker(filter, inputChan)
		},
		ProcessWorker: func(instanceChan <-chan I, resultChan chan<- FilterResult[I, O]) {
			FilterProcessWorker(filter, instanceChan, resultChan)
		},
		OutputWorker: func(resultsChan <-chan FilterResult[I, O]) {
			FilterOutputWorker(filter, resultsChan)
		},
		Workers:      workers,
		InputBuffer:  100,
		OutputBuffer: 100,
	}.Run()
}
