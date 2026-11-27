package main

import (
	"fmt"
	"image"
	"strings"

	"golang.org/x/image/font"
)

// todoItem is one line of todo.txt on the PC. Line is its 0-based line number in the file,
// sent back with the text when the item is tapped so the PC toggles the right task.
type todoItem struct {
	Line int    `json:"line"`
	Text string `json:"text"`
	Done bool   `json:"done"`
}

type todoList struct {
	Items []todoItem `json:"items"` // open tasks first, at most todoRows
	Done  int        `json:"done"`
	Total int        `json:"total"`
	Error string     `json:"error"`
}

// Pixel layout of the TO-DO panel in the 600 × 800 backdrop.
const (
	todoRows                  = 8
	todoTop, todoRowHeight    = 126, 40
	todoLeft, todoRight       = 400, 581 // area cleared for each row
	todoBoxX, todoBoxSize     = 402, 16
	todoTextX, todoTextWidth  = 427, 151
	todoPanelX0, todoPanelX1  = 395, 585 // tappable width
	todoCountRight, todoCount = 577, 111 // right edge and ink bottom of the "2/6" counter
)

func todoRowAt(point image.Point) (int, bool) {
	if point.X < todoPanelX0 || point.X >= todoPanelX1 || point.Y < todoTop || point.Y >= todoTop+todoRows*todoRowHeight {
		return 0, false
	}
	return (point.Y - todoTop) / todoRowHeight, true
}

// wrapTwoLines splits text into at most two lines that fit width, ending in "..." if it is cut.
func (a *artwork) wrapTwoLines(face font.Face, text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	first := ""
	rest := words
	for len(rest) > 0 {
		next := strings.TrimSpace(first + " " + rest[0])
		if a.measure(face, next) > width {
			break
		}
		first, rest = next, rest[1:]
	}
	if first == "" {
		// A single word wider than the row: cut it.
		return []string{a.fitText(face, text, width)}
	}
	if len(rest) == 0 {
		return []string{first}
	}
	return []string{first, a.fitText(face, strings.Join(rest, " "), width)}
}

func drawCheckbox(img *canvas, t theme, x, y int, done bool) {
	if done {
		rectangle(img, x, y, x+todoBoxSize, y+todoBoxSize, t.frame)
		fx, fy := float64(x), float64(y)
		img.line(fx+3, fy+7.5, fx+6.5, fy+11, 2, t.panel)
		img.line(fx+6.5, fy+11, fx+12, fy+3.5, 2, t.panel)
		return
	}
	outline(img, image.Rect(x, y, x+todoBoxSize, y+todoBoxSize), t.frame)
	outline(img, image.Rect(x+1, y+1, x+todoBoxSize-1, y+todoBoxSize-1), t.frame)
}

// centredMessage clears the whole list (row dividers included) and shows a short note.
func (a *artwork) centredMessage(img *canvas, t theme, lines ...string) {
	rectangle(img, 396, 124, 584, 451, t.panel)
	baseline := 270 - len(lines)*9
	for _, line := range lines {
		centerText(img, a.week, 490, baseline, line, t.ink)
		baseline += 20
	}
}

// drawTodo fills the TO-DO panel. online is false when the PC companion cannot be reached.
func (a *artwork) drawTodo(img *canvas, t theme, list *todoList, online bool) {
	rectangle(img, 530, 96, 584, 119, t.panel)
	for row := 0; row < todoRows; row++ {
		top := todoTop + row*todoRowHeight
		rectangle(img, todoLeft, top, todoRight, top+todoRowHeight-1, t.panel)
	}
	switch {
	case !online:
		a.centredMessage(img, t, "PC OFFLINE", "", "Your tasks are in", "todo.txt on the PC.")
		return
	case list == nil:
		a.centredMessage(img, t, "Update the PC", "companion to see", "your to-do list.")
		return
	case list.Error != "":
		a.centredMessage(img, t, "Could not read", "todo.txt on the PC.")
		return
	case len(list.Items) == 0:
		a.centredMessage(img, t, "Nothing to do!", "", "Add tasks to", "todo.txt on the PC.")
	}
	inkText(img, a.count, fmt.Sprintf("%d/%d", list.Done, list.Total), alignRight, todoCountRight, todoCount, t.ink, 1)
	for row, item := range list.Items {
		if row == todoRows {
			break
		}
		top := todoTop + row*todoRowHeight
		drawCheckbox(img, t, todoBoxX, top+12, item.Done)
		lines := a.wrapTwoLines(a.week, item.Text, todoTextWidth)
		baseline := top + 24
		if len(lines) == 2 {
			baseline = top + 16
		}
		ink := t.ink
		if item.Done {
			ink = t.muted
		}
		for _, line := range lines {
			textAt(img, a.week, todoTextX, baseline, line, ink)
			if item.Done {
				rectangle(img, todoTextX, baseline-5, todoTextX+a.measure(a.week, line), baseline-4, ink)
			}
			baseline += 16
		}
	}
}
