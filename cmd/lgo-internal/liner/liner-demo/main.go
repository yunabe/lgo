package main

import (
	"io"
	"log"

	"github.com/yunabe/lgo/cmd/lgo-internal/liner"
)

func main() {
	liner := liner.NewLiner()
	for {
		content, err := liner.Next()
		if err == io.EOF {
			break
		}
		log.Printf("Content == %q (err == %v)", content, err)
	}
}
