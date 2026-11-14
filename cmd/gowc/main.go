package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

func spaceByte(inByte byte) bool {
	switch inByte {
	case ' ':
		return true
	case '\n':
		return true
	case '\t':
		return true
	case '\v':
		return true
	case '\f':
		return true
	case '\r':
		return true
	}
	return false
}

func processInput(r io.Reader) {
	readInterface := bufio.NewReader(r)
	var byteCounter int
	var wordCounter int
	var lineCounter int

	var isWord bool
	for {
		byte, err := readInterface.ReadByte()
		if err == io.EOF {
			break
		}

		if err != nil {
			fmt.Printf("Read error %v\n", err)
			return
		}
		byteCounter++

		if byte == '\n' {
			lineCounter++
		}

		if spaceByte(byte) {
			isWord = false
		} else if !spaceByte(byte) && !isWord {
			wordCounter++
			isWord = true
		} else if !spaceByte(byte) && isWord {
			continue
		}
	}

	fmt.Printf("Byte Count: %d\nWord Count: %d\nLine Count: %d\n", byteCounter, wordCounter, lineCounter)
}

func main() {
	//fmt.Println("Hello")
	processInput(os.Stdin)
}
