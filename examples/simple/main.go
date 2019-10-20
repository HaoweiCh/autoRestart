package main

import (
	"fmt"
	"os"

	"github.com/HaoweiCh/autoRestart"
)

func main() {
	go func() {
		err := autoRestart.Watch()
		if err != nil {
			panic(err) // Only returns initialisation errors.
		}
	}()

	fmt.Println(os.Args)
	fmt.Println(os.Environ())

	// block
	ch := make(chan bool)
	<-ch
}
