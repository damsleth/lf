package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v3"
)

type sixelScreen struct {
	lastFile   string
	lastWin    win
	forceClear bool
}

func (sxs *sixelScreen) clearSixel(win *win, screen tcell.Screen, filePath string) {
	if sxs.lastFile != "" && (filePath != sxs.lastFile || *win != sxs.lastWin || sxs.forceClear) {
		screen.LockRegion(sxs.lastWin.x, sxs.lastWin.y, sxs.lastWin.w, sxs.lastWin.h, false)
	}
}

func (sxs *sixelScreen) printSixel(win *win, screen tcell.Screen, reg *reg) {
	if reg.path == sxs.lastFile && *win == sxs.lastWin && !sxs.forceClear {
		return
	}

	cw, ch := -1, -1
	y := win.y
	var b strings.Builder

	for _, line := range reg.lines {
		if !strings.HasPrefix(line, "\033P") {
			line, h := renderRawLine(line, win.w)
			if y+h > win.y+win.h {
				break
			}

			// Blank all rows first: in kitty, writing over any cell of a
			// multicell (OSC 66) run erases the whole run.
			screen.LockRegion(win.x, y, win.w, h, true)
			for r := range h {
				fmt.Fprintf(&b, "\033[0m\033[%d;%dH%s", y+r+1, win.x+1, strings.Repeat(" ", win.w))
			}
			fmt.Fprintf(&b, "\033[%d;%dH%s\033[0m\033]8;;\033\\", y+1, win.x+1, line)
			y += h
			continue
		}

		if cw < 0 {
			var err error
			if cw, ch, err = cellSize(screen); err != nil {
				log.Printf("sixel: %s", err)
				continue
			}
		}

		matches := reSixelSize.FindStringSubmatch(line)
		if matches == nil {
			log.Print("sixel: failed to get image size")
			continue
		}

		iw, _ := strconv.Atoi(matches[1])
		ih, _ := strconv.Atoi(matches[2])

		sw := (iw + cw - 1) / cw
		sh := (ih + ch - 1) / ch

		if y+sh-1 >= win.y+win.h {
			break
		}

		screen.LockRegion(win.x, y, sw, sh, true)
		fmt.Fprintf(&b, "\033[%d;%dH", y+1, win.x+1)
		b.WriteString(line)
		y += sh
	}

	fmt.Fprint(os.Stderr, "\033[?2026h") // Begin synchronized update
	fmt.Fprint(os.Stderr, "\0337")       // Save cursor position
	fmt.Fprint(os.Stderr, b.String())    // Write data
	fmt.Fprint(os.Stderr, "\0338")       // Restore cursor position
	fmt.Fprint(os.Stderr, "\033[?2026l") // End synchronized update

	sxs.lastFile = reg.path
	sxs.lastWin = *win
	sxs.forceClear = false
}

func cellSize(screen tcell.Screen) (int, int, error) {
	tty, ok := screen.Tty()
	if !ok {
		return -1, -1, fmt.Errorf("failed to get tty")
	}

	ws, err := tty.WindowSize()
	if err != nil {
		return -1, -1, fmt.Errorf("failed to get window size: %w", err)
	}

	cw, ch := ws.CellDimensions()
	if cw <= 0 || ch <= 0 {
		// fallback for Windows Terminal
		return 10, 20, nil
	}

	return cw, ch, nil
}
