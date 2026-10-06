package ui

import tea "charm.land/bubbletea/v2"

// Direction represents possible moving directions.
type Direction int

const (
	DirectionLeft Direction = iota
	DirectionRight
	DirectionDown
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

// ScrollMsg shows that user wants to scroll content of the selected component.
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

// Moves reports whether msg moves the cursor between items.
func Moves(msg tea.Msg) bool {
	switch msg.(type) {
	case MoveMsg, JumpMsg:
		return true
	}

	return false
}
