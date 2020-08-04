package main

import (
	"log"

	wasmgo "./wasm"
)

func main() {
	b, err := wasmgo.BridgeFromFile("test", "./ticket.wasm", nil)
	if err != nil {
		log.Fatal(err)
	}

	if err := b.Run(); err != nil {
		log.Fatal(err)
	}
}
