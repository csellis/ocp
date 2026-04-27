package voice

import "math/rand/v2"

// ObliquePack is a curated subset of Brian Eno's Oblique Strategies
// cards. OCP appends a card to each observation as a quiet nudge
// toward a different angle on the work. Add cards in the same
// imperative-or-aphoristic register.
var ObliquePack = []string{
	"Honor thy error as a hidden intention.",
	"Use an old idea.",
	"What would your closest friend do?",
	"Be less critical more often.",
	"Give the game away.",
	"Discard an axiom.",
	"Take away the elements in order of apparent non-importance.",
	"Look at the order in which you do things.",
	"Question the heroic.",
	"Don't be afraid of things because they're easy to do.",
	"The most important thing is the thing most easily forgotten.",
	"Distorting time.",
	"What mistakes did you make last time?",
	"Listen to the quiet voice.",
	"Disconnect from desire.",
	"Don't break the silence.",
	"Repetition is a form of change.",
	"What would make this really successful?",
	"Trust in the you of now.",
	"Once the search is in progress, something will be found.",
}

// PickCard returns one card from ObliquePack at random. math/rand/v2
// auto-seeds; no setup needed. Pure-random by design — observations
// look different across runs.
func PickCard() string {
	return ObliquePack[rand.IntN(len(ObliquePack))]
}
