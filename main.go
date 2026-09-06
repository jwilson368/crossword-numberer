// Command xwnum reads a crossword grid as plain text and prints the
// standard across/down clue numbering for every cell.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	blockFlag := flag.String("block", "#", "character used for a black square")
	inPath := flag.String("in", "", "grid file to read (default: stdin)")
	flag.Parse()

	if len(*blockFlag) != 1 {
		fmt.Fprintln(os.Stderr, "xwnum: -block must be exactly one character")
		os.Exit(2)
	}

	in := os.Stdin
	if *inPath != "" {
		f, err := os.Open(*inPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "xwnum: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		in = f
	}

	if err := StreamNumber(in, os.Stdout, (*blockFlag)[0]); err != nil {
		fmt.Fprintf(os.Stderr, "xwnum: %v\n", err)
		os.Exit(1)
	}
}
