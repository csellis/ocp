// Package names holds the Banks-style ship-name pack and the selector
// that gives one OCP instance its identity.
//
// In v0.1 every instance uses the dogfood name. Future slices will read
// .ocp/config.toml and either return the persisted ship-name or pick
// one deterministically from Pack on first run.
package names

// Pack is the canonical set of ship-names OCP instances may take. New
// names go here in the same register: a short statement of character
// or wry bureaucratic title. The first entry is the v0.1 default.
var Pack = []string{
	"Drone Honor Thy Error As A Hidden Intention",
	"GCU Conscientious Objector",
	"Mind Quietly Tending The Names",
	"Drone Steady Drift",
	"GSV Sufficient Unto The Day",
	"Eccentric Ambient Patience",
	"Mind Of Quiet Citation",
	"ROU Charitably Misread",
}

// Default returns the v0.1 ship-name. Hardcoded to the dogfood instance
// (Pack[0]) so README and architecture stay accurate. A future slice
// adds per-repo persistence and seed-based selection.
func Default() string {
	return Pack[0]
}
