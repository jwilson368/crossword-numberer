# crossword-numberer

Given a crossword grid, work out which cells get a clue number.

The rule is the usual one: a cell gets a number if it's not a black
square and either (a) there's no open cell to its left but there is one
to its right (it starts an across word), or (b) there's no open cell
above it but there is one below (it starts a down word). Doing this by
hand for anything bigger than a Sunday-size grid is tedious and easy to
get wrong near the edges.

`xwnum` reads a grid as plain text, one row per line, and prints the
same grid back with each cell replaced by its clue number (or `.` for
an open cell with no number, `#` for a black square).

## Input format

Each line is one row. By default `#` marks a black square and any other
character marks an open cell — the actual letter, if there is one,
doesn't matter, so a solved grid, a blank grid, or a grid of dots all
number the same way. Use `-block` to pick a different black-square
character.

## Usage

```sh
$ cat grid.txt
.....
.###.
.....
.###.
.....

$ xwnum -in grid.txt
1 . . . 2
. # # # .
3 . . . .
. # # # .
4 . . . .
```

It also reads from stdin, so it fits in a pipeline:

```sh
$ xwnum < grid.txt
```

Pass `-clues` to get a clue list instead of the numbered grid — each
entry is the clue number and the word's length, grouped by direction:

```sh
$ xwnum -clues -in grid.txt
Across
1. 5
3. 5
4. 5
Down
1. 5
2. 5
```

## Multiple grids

A blank line separates one grid from the next, so a batch of puzzles
can be numbered in a single pass:

```sh
$ xwnum <<'EOF'
...
.#.
...

....
EOF
1 . 2
. # .
3 . .

1 . . .
```

Each grid's numbering starts back at 1. This only applies to the
default numbered-grid output; `-clues` currently expects a single grid
per run.

## Why streaming matters here

Numbering a cell needs to know about the row above it (to check for a
down word) and the row below it (same reason). `xwnum` never buffers
more than those two neighboring rows plus the row it's currently
finalizing — it doesn't read the whole grid into memory first. That
means it can number a grid of arbitrary size, or a stream of many grids
back to back, using memory proportional to the grid's width, not its
area.

## Building

```sh
go build -o xwnum .
```

No dependencies outside the standard library.
