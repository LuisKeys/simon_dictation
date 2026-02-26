package main

import (
	"log"
	"sync"

	"github.com/joho/godotenv"
	vtt "github.com/luiskeys/simon_dictate/src/vtt"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	var wg sync.WaitGroup
	vttsrv := vtt.Init()
	wg.Add(1)
	go func() {
		defer wg.Done()
		vttsrv.Run()
	}()

	wg.Wait()
}
