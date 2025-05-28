package main

import (
	"sync"

	vtt "github.com/luiskeys/simon_dictate/src/vtt"
)

func main() {
	var wg sync.WaitGroup
	vttsrv := vtt.Init()
	wg.Add(1)
	go func() {
		defer wg.Done()
		vttsrv.Run()
	}()

	wg.Wait()
}
