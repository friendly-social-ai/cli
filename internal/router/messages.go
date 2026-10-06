package router

import (
	tea "charm.land/bubbletea/v2"
	"github.com/friendly-social/cli/internal/screen"
)

// BroadcastMsg tells router to broadcast inner message to all screens.
type BroadcastMsg struct {
	Inner tea.Msg
}

// TargetMsg tells router to route inner message to one specific screen.
type TargetMsg struct {
	Type  screen.Type
	Inner tea.Msg
}
