package engine

// stopGraceSeconds holds the agent-wide graceful-stop grace, in
// seconds: -1 keeps each engine's built-in default (QEMU 10 s,
// Firecracker 30 s), 0 forces an immediate kill with no graceful
// attempt, and a positive value overrides the default. It exists for
// test fleets and operator drills; the default path is unchanged.
var stopGraceSeconds = -1

// SetStopGrace applies the agent-wide stop grace. Call once at service
// startup, before engines are constructed.
func SetStopGrace(seconds int) {
	if seconds < -1 {
		seconds = -1
	}
	stopGraceSeconds = seconds
}
