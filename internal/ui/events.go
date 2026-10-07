package ui

import tea "charm.land/bubbletea/v2"

// Direction represents possible moving directions.
type Direction int

const (
	DirectionDown Direction = iota
	DirectionUp
)

// MoveMsg shows that user wants to move to a different component in some direction.
type MoveMsg struct {
	Direction Direction
}

// JumpMsg shows that user wants to move to the first item with DirectionUp, or to the last one with DirectionDown.
type JumpMsg struct {
	Direction Direction
}

// ScrollMsg shows that user wants to scroll the selected item while it is clipped, or else the list by half a page.
type ScrollMsg struct {
	Direction Direction
}

// InteractMsg shows that user wants to interact with some component.
type InteractMsg struct{}

// FocusMsg shows that user wants to focus on some component to interact with it.
type FocusMsg struct{}

// UnfocusMsg shows that user no longer wants to focus on current component.
type UnfocusMsg struct{}

// InsertMsg asks navigation to start typing into the selected component.
type InsertMsg struct{}

// NormalMsg asks navigation to stop typing.
type NormalMsg struct{}

// SelectMsg shows that user wants to select some component to focus on it later.
type SelectMsg struct{}

// UnselectMsg shows that user no longer wants current component to be selected.
type UnselectMsg struct{}

// WheelMsg shows that user turned the mouse wheel to scroll the list in some direction.
type WheelMsg struct {
	Direction Direction
}

// ClickMsg is a left click at cell X, Y, counted from the top left corner of the component that receives it.
type ClickMsg struct {
	X, Y int
}

// ClickedMsg reports that a click landed on a List item. Again is true when the item was already selected.
type ClickedMsg struct {
	Again bool
}

// Moves reports whether msg moves the cursor between items.
func Moves(msg tea.Msg) bool {
	switch msg.(type) {
	case MoveMsg, JumpMsg, ScrollMsg, ClickMsg, WheelMsg:
		return true
	}

	return false
}
