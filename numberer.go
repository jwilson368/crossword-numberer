package main

import (
	"bufio"
	"fmt"
	"io"
	"sort"
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

// Clue is one entry in a clue list: the number printed in the grid, and
// how many cells the word occupies.
type Clue struct {
	Number int
	Length int
}

// acrossLength counts the open cells starting at col and running right,
// stopping at the first black square or the edge of the row.
func acrossLength(row []bool, col int) int {
	n := 0
	for c := col; c < len(row) && row[c]; c++ {
		n++
	}
	return n
}

// ListClues reads a crossword grid the same way StreamNumber does, but
// instead of printing the numbered grid it returns the across and down
// clue lists (number and word length). An across word's length is known
// as soon as its row is read, but a down word's length isn't known until
// the row where it ends, so this tracks one length-in-progress per
// column rather than buffering the whole grid — memory stays proportional
// to the grid's width, not its area.
func ListClues(r io.Reader, block byte) (across, down []Clue, err error) {
	scanner := bufio.NewScanner(r)

	type downState struct {
		number int
		length int
	}
	active := map[int]*downState{}

	var prev, curr []bool
	haveCurr := false
	counter := 0
	lineNum := 0
	width := -1

	process := func(prevRow, currRow, nextRow []bool) {
		for col, open := range currRow {
			if !open {
				if ds, ok := active[col]; ok {
					down = append(down, Clue{ds.number, ds.length})
					delete(active, col)
				}
				continue
			}

			leftBlocked := col == 0 || !currRow[col-1]
			rightOpen := col+1 < len(currRow) && currRow[col+1]
			startsAcross := leftBlocked && rightOpen

			aboveBlocked := prevRow == nil || col >= len(prevRow) || !prevRow[col]
			belowOpen := nextRow != nil && col < len(nextRow) && nextRow[col]
			startsDown := aboveBlocked && belowOpen

			if startsAcross || startsDown {
				counter++
			}
			if startsAcross {
				across = append(across, Clue{counter, acrossLength(currRow, col)})
			}
			switch {
			case startsDown:
				active[col] = &downState{number: counter, length: 1}
			case active[col] != nil:
				active[col].length++
			}
		}
	}

	for scanner.Scan() {
		lineNum++
		next := parseRow(scanner.Text(), block)
		if width == -1 {
			width = len(next)
		} else if len(next) != width {
			return nil, nil, fmt.Errorf("line %d: row width %d does not match grid width %d", lineNum, len(next), width)
		}

		if haveCurr {
			process(prev, curr, next)
			prev = curr
		}
		curr = next
		haveCurr = true
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("reading grid: %w", err)
	}
	if haveCurr {
		process(prev, curr, nil)
	}

	// Any down word still open when the grid ends (no trailing black
	// square to close it) needs to be flushed. Walk columns in order so
	// the result is deterministic rather than following map order.
	for col := 0; col < width; col++ {
		if ds, ok := active[col]; ok {
			down = append(down, Clue{ds.number, ds.length})
		}
	}
	sort.Slice(down, func(i, j int) bool { return down[i].Number < down[j].Number })

	return across, down, nil
}

// WriteClueList prints across and down clue lists as "number. length"
// under headers, the format most crossword clue lists use before the
// clue text itself is filled in.
func WriteClueList(w io.Writer, across, down []Clue) {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	fmt.Fprintln(bw, "Across")
	for _, c := range across {
		fmt.Fprintf(bw, "%d. %d\n", c.Number, c.Length)
	}
	fmt.Fprintln(bw, "Down")
	for _, c := range down {
		fmt.Fprintf(bw, "%d. %d\n", c.Number, c.Length)
	}
}
