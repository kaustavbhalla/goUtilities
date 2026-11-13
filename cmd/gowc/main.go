package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
)

func processInput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() == true {
		fmt.Printf("Line: %v\n", scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error while reading file %v", err)
	}

}

func main() {
	fmt.Println("Hello")
	processInput(os.Stdin)
}
