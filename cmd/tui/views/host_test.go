package views_test

import (
	"github.com/balinomad/prompt-backlog/cmd/tui/views/internal/viewtest"
)

var (
	newFakeApp       = viewtest.NewFakeApp
	runeKey          = viewtest.RuneKey
	escKey           = viewtest.EscKey
	assertHints      = viewtest.AssertHints
	confirmModalFrom = viewtest.ConfirmModalFrom
)
