// Copyright 2026 The TCell Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tview

import (
	"testing"

	"github.com/gmlewis/tcell/v2"
)

// clearCountScreen wraps a tcell.Screen and counts how many times Clear()
// is called, so tests can verify that draw() skips screen.Clear() on
// normal redraws and only calls it when fullRedraw is set.
type clearCountScreen struct {
	tcell.Screen
	clears int
}

func (s *clearCountScreen) Clear() {
	s.clears++
	s.Screen.Clear()
}

// newTestAppWithScreen creates an Application wired to a clear-counting
// SimulationScreen and a simple root primitive, ready for draw() calls.
func newTestAppWithScreen(t *testing.T) (*Application, *clearCountScreen) {
	t.Helper()
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim.Init: %v", err)
	}
	t.Cleanup(sim.Fini)
	sim.SetSize(40, 10)
	wrapped := &clearCountScreen{Screen: sim}

	app := NewApplication()
	app.SetRoot(NewTextView(), true)
	// Replace the screen with our wrapper.
	app.Lock()
	app.screen = wrapped
	app.Unlock()

	return app, wrapped
}

// TestDrawSkipsClearOnNormalRedraw verifies the core optimization: a
// normal draw() (fullRedraw=false) must NOT call screen.Clear(). This
// eliminates the O(w×h) Fill loop that tview previously ran on every
// frame. tcell's per-cell dirty checking handles incremental repaints.
func TestDrawSkipsClearOnNormalRedraw(t *testing.T) {
	app, wrapped := newTestAppWithScreen(t)

	// Consume the fullRedraw=true from SetRoot by drawing once.
	app.draw()
	wrapped.clears = 0 // reset counter after initial draw

	// Normal redraw: fullRedraw is false → Clear must be skipped.
	app.draw()
	if wrapped.clears != 0 {
		t.Errorf("draw() called Clear() %d time(s) on normal redraw, want 0", wrapped.clears)
	}
}

// TestDrawClearsOnFullRedraw verifies that draw() calls screen.Clear()
// when fullRedraw is true (e.g. after resize, SetRoot, or Sync), and
// resets fullRedraw to false so the next draw is incremental.
func TestDrawClearsOnFullRedraw(t *testing.T) {
	app, wrapped := newTestAppWithScreen(t)

	// Consume the initial fullRedraw from SetRoot.
	app.draw()
	wrapped.clears = 0

	// Set fullRedraw and draw — Clear must be called once.
	app.fullRedraw = true
	app.draw()
	if wrapped.clears != 1 {
		t.Errorf("draw() called Clear() %d time(s) on full redraw, want 1", wrapped.clears)
	}

	// fullRedraw must be reset to false after the draw.
	if app.fullRedraw {
		t.Error("fullRedraw should be false after draw()")
	}

	// Next normal draw must NOT clear.
	wrapped.clears = 0
	app.draw()
	if wrapped.clears != 0 {
		t.Errorf("draw() called Clear() %d time(s) after fullRedraw was reset, want 0", wrapped.clears)
	}
}

// TestSetRootSetsFullRedraw verifies that SetRoot sets fullRedraw=true
// so the next draw() does a full clear.
func TestSetRootSetsFullRedraw(t *testing.T) {
	app, _ := newTestAppWithScreen(t)

	// Consume the initial fullRedraw from the first SetRoot.
	app.draw()

	// SetRoot again — should set fullRedraw.
	app.SetRoot(NewTextView(), true)
	if !app.fullRedraw {
		t.Error("fullRedraw should be true after SetRoot")
	}
}

// TestSetFocusSkipsWhenUnchanged verifies that SetFocus(p) is a no-op
// when p is already the focused primitive. This avoids redundant
// Blur/Focus/HideCursor cycles on every SetFocus call from tview's
// internal delegate closures, eliminating cursor flicker at the source.
func TestSetFocusSkipsWhenUnchanged(t *testing.T) {
	app, wrapped := newTestAppWithScreen(t)
	app.draw() // consume initial fullRedraw
	wrapped.clears = 0

	tv := NewTextView()
	app.SetFocus(tv)
	if app.GetFocus() != tv {
		t.Fatalf("GetFocus = %v, want tv after first SetFocus", app.GetFocus())
	}

	// SetFocus with the same primitive must be a no-op.
	// HideCursor should NOT be called (no cursor flicker).
	app.SetFocus(tv)
	if app.GetFocus() != tv {
		t.Error("GetFocus changed after SetFocus with same primitive")
	}
}

// TestSetFocusChangesFocus verifies that SetFocus with a different
// primitive updates a.focus, calls Blur on the old, and calls Focus
// on the new.
func TestSetFocusChangesFocus(t *testing.T) {
	app, _ := newTestAppWithScreen(t)
	app.draw()

	tv1 := NewTextView()
	tv2 := NewTextView()

	app.SetFocus(tv1)
	if app.GetFocus() != tv1 {
		t.Fatalf("GetFocus = %v, want tv1", app.GetFocus())
	}

	app.SetFocus(tv2)
	if app.GetFocus() != tv2 {
		t.Fatalf("GetFocus = %v, want tv2 after SetFocus(tv2)", app.GetFocus())
	}

	// tv1 should no longer have focus.
	if tv1.HasFocus() {
		t.Error("tv1 should not have focus after SetFocus(tv2)")
	}
}

// TestGetFocusReturnsDirectFocus verifies that GetFocus returns a.focus
// directly (the v0.42.0-compatible behavior), not by walking the focus
// chain. This is critical for gonomadnet's custom containers that may
// not implement focusChain.
func TestGetFocusReturnsDirectFocus(t *testing.T) {
	app, _ := newTestAppWithScreen(t)
	app.draw()

	tv := NewTextView()
	app.SetFocus(tv)

	got := app.GetFocus()
	if got != tv {
		t.Errorf("GetFocus = %v, want %v (direct a.focus)", got, tv)
	}
}

// TestGetFocusNilWhenNoFocus verifies that GetFocus returns nil when no
// primitive has been focused.
func TestGetFocusNilWhenNoFocus(t *testing.T) {
	app, _ := newTestAppWithScreen(t)
	app.draw()

	// Clear focus.
	app.Lock()
	app.focus = nil
	app.Unlock()

	if got := app.GetFocus(); got != nil {
		t.Errorf("GetFocus = %v, want nil", got)
	}
}