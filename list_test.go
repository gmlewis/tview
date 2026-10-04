package tview

import (
	"testing"

	"github.com/gmlewis/tcell/v2"
)

// TestListSelectedHighlightsSecondaryLine pins two-line item highlighting: with
// ShowSecondaryText(true) each item occupies two rows (main + secondary), and
// when an item is selected BOTH rows must carry the selected style — foreground
// plus background, and the HighlightFullLine fill across the full width on each
// row. nomadnet (urwid) renders each list entry as a single Text whose
// focus_style colors name and time lines together, so a selected entry shows
// the highlight on both rows; the old List.Draw styled only the main line and
// left the secondary row at the unfocused secondary style and no background.
func TestListSelectedHighlightsSecondaryLine(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim.Init: %v", err)
	}
	t.Cleanup(sim.Fini)
	sim.SetSize(30, 6)

	bg := tcell.NewRGBColor(170, 170, 170)
	fg := tcell.NewRGBColor(17, 17, 17)

	list := NewList()
	list.ShowSecondaryText(true)
	list.SetHighlightFullLine(true)
	list.SetSelectedStyle(tcell.StyleDefault.Foreground(fg).Background(bg)).
		AddItem("conv zero", "  2w ago", 0, nil).
		AddItem("conv one", "  3w ago", 0, nil)
	list.SetRect(0, 0, 30, 6)
	list.Draw(sim)
	sim.Sync()

	// tcell v2.8 Style has no getters — Decompose() is the reader.
	bgOf := func(s tcell.Style) tcell.Color { _, b, _ := s.Decompose(); return b }
	fgOf := func(s tcell.Style) tcell.Color { f, _, _ := s.Decompose(); return f }

	cells, w, _ := sim.GetContents()
	cellAt := func(x, y int) (tcell.Style, rune) {
		c := cells[y*w+x]
		return c.Style, c.Runes[0]
	}

	// Row 0: the selected main line — must carry fg+bg.
	style, _ := cellAt(6, 0)
	if bgOf(style) != bg {
		t.Errorf("selected main line cell 6,0 bg = %v, want %v", bgOf(style), bg)
	}
	if fgOf(style) != fg {
		t.Errorf("selected main line cell 6,0 fg = %v, want %v", fgOf(style), fg)
	}
	// Full-line fill on the main line: a cell past the text end.
	if style, _ = cellAt(25, 0); bgOf(style) != bg {
		t.Errorf("selected main line fill cell 25,0 bg = %v, want %v", bgOf(style), bg)
	}

	// Row 1: the selected item's SECONDARY line — must carry the same fg+bg
	// and the full-line fill (this is the regression: it used to render with
	// the unfocused secondary style and no background).
	style, _ = cellAt(6, 1)
	if bgOf(style) != bg {
		t.Errorf("selected secondary line cell 6,1 bg = %v, want %v", bgOf(style), bg)
	}
	if fgOf(style) != fg {
		t.Errorf("selected secondary line cell 6,1 fg = %v, want %v", fgOf(style), fg)
	}
	if style, _ = cellAt(25, 1); bgOf(style) != bg {
		t.Errorf("selected secondary line fill cell 25,1 bg = %v, want %v", bgOf(style), bg)
	}

	// The unselected item's main line (row 2) and secondary line (row 3) must
	// NOT carry the selected background.
	style, _ = cellAt(6, 2)
	if bgOf(style) == bg {
		t.Error("unselected main line carries selected background")
	}
	style, _ = cellAt(6, 3)
	if bgOf(style) == bg {
		t.Error("unselected secondary line carries selected background")
	}

	// GetShowSecondaryText reflects the flag the draw path keys off.
	if !list.GetShowSecondaryText() {
		t.Error("GetShowSecondaryText() = false, want true")
	}
}

// TestListItemEntryStyle pins per-entry styling: SetItemStyle assigns an
// optional style to one list entry, drawn on BOTH the main and the secondary
// line while the item is unselected — mirroring urwid's AttrMap(attr,
// focus_attr), which styles the whole entry widget (nomadnet renders each
// conversation entry as one Text whose name and time lines share the
// list_normal / msg_notice_unread attr). The SELECTED item keeps selectedStyle
// (urwid focus_attr semantics): its entry style, if any, must be ignored.
func TestListItemEntryStyle(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim.Init: %v", err)
	}
	t.Cleanup(sim.Fini)
	sim.SetSize(30, 6)

	entryFG := tcell.NewHexColor(0xbbaa44) // "list_warning"-style entry attr
	selBG := tcell.NewRGBColor(170, 170, 170)

	list := NewList()
	list.ShowSecondaryText(true)
	list.SetSelectedStyle(tcell.StyleDefault.Foreground(tcell.NewRGBColor(17, 17, 17)).Background(selBG)).
		AddItem("sel entry", "  1d ago", 0, nil).
		AddItem("other entry", "  2w ago", 0, nil)
	list.SetCurrentItem(0)
	list.SetItemStyle(0, tcell.StyleDefault.Foreground(entryFG)) // must be ignored (selected)
	list.SetItemStyle(1, tcell.StyleDefault.Foreground(entryFG))
	list.SetRect(0, 0, 30, 6)
	list.Draw(sim)
	sim.Sync()

	cells, w, _ := sim.GetContents()
	bgOf := func(s tcell.Style) tcell.Color { _, b, _ := s.Decompose(); return b }
	fgOf := func(s tcell.Style) tcell.Color { f, _, _ := s.Decompose(); return f }
	styleAt := func(x, y int) tcell.Style { return cells[y*w+x].Style }

	// Selected entry (item 0): selectedStyle wins on both lines — entry style
	// ignored, and the selected background covers the text cells.
	if s := styleAt(4, 0); fgOf(s) != tcell.NewRGBColor(17, 17, 17) {
		t.Errorf("selected main line fg = %v, want selected fg #111", fgOf(s))
	}
	if s := styleAt(4, 1); bgOf(s) != selBG {
		t.Errorf("selected secondary line bg = %v, want selected bg", bgOf(s))
	}

	// Unselected entry (item 1): the entry style colors BOTH lines, with the
	// default background untouched.
	if s := styleAt(4, 2); fgOf(s) != entryFG {
		t.Errorf("unselected main line fg = %v, want entry style fg", fgOf(s))
	}
	if s := styleAt(4, 3); fgOf(s) != entryFG {
		t.Errorf("unselected secondary line fg = %v, want entry style fg", fgOf(s))
	}
	if s := styleAt(20, 3); bgOf(s) == selBG {
		t.Errorf("unselected secondary line bg = %v, want no selected background", bgOf(s))
	}
}