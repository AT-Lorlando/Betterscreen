package ui

type focusZone int

const (
	focusSessions focusZone = iota
	focusWindows
)

type uiMode int

const (
	modeNormal uiMode = iota
	modeConfirmKill
	modeNewSession
)
