package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// parseRow converts a line of grid text into a slice of "is this cell
// open" flags. Any byte equal to block is a black square; everything
// else (a letter, a dot, whatever the caller uses for an empty cell)
// counts as open.
func parseRow(line string, block byte) []bool {
	row := make([]bool, len(line))
	for i := 0; i < len(line); i++ {
		row[i] = line[i] != block
	}
	return row
}

// needsNumber reports whether the cell at col in curr starts an across
// word, a down word, or both — exactly the condition standard crossword
// numbering uses to decide which cells get a number. prev and next are
// the rows above and below curr, and may be nil to mean "off the grid",
// which is treated the same as a black square.
func needsNumber(prev, curr, next []bool, col int) bool {
	if !curr[col] {
		return false
	}

	leftBlocked := col == 0 || !curr[col-1]
	rightOpen := col+1 < len(curr) && curr[col+1]
	startsAcross := leftBlocked && rightOpen

	aboveBlocked := prev == nil || col >= len(prev) || !prev[col]
	belowOpen := next != nil && col < len(next) && next[col]
	startsDown := aboveBlocked && belowOpen

	return startsAcross || startsDown
}

// StreamNumber reads a crossword grid one line at a time and writes the
// numbered grid to w. Deciding whether a cell starts a down word needs
// the row below it, so this holds at most three rows in memory at once
// (above, current, below) rather than the whole grid — a grid with a
// million rows costs the same handful of bytes as one with ten.
func StreamNumber(r io.Reader, w io.Writer, block byte) error {
	scanner := bufio.NewScanner(r)
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	var prev, curr []bool
	haveCurr := false
	counter := 0
	lineNum := 0
	width := -1

	emit := func(prevRow, currRow, nextRow []bool) {
		parts := make([]string, len(currRow))
		for col, open := range currRow {
			switch {
			case !open:
				parts[col] = "#"
			case needsNumber(prevRow, currRow, nextRow, col):
				counter++
				parts[col] = fmt.Sprintf("%d", counter)
			default:
				parts[col] = "."
			}
		}
		fmt.Fprintln(bw, strings.Join(parts, " "))
	}

	for scanner.Scan() {
		lineNum++
		next := parseRow(scanner.Text(), block)
		if width == -1 {
			width = len(next)
		} else if len(next) != width {
			return fmt.Errorf("line %d: row width %d does not match grid width %d", lineNum, len(next), width)
		}

		if haveCurr {
			emit(prev, curr, next)
			prev = curr
		}
		curr = next
		haveCurr = true
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading grid: %w", err)
	}
	if haveCurr {
		emit(prev, curr, nil)
	}
	return nil
}
