// Package argvine is a dependency-free command-line framework for Go.
//
// A CLI is described as an immutable tree of Command nodes. Parsing, help
// rendering and shell completion are three independent readers of that same
// tree, which is what keeps them from ever disagreeing with each other: a flag
// added to the tree shows up in all three without anyone editing three places.
package argvine

// FlagType enumerates the value types a Flag can hold.
//
// The type is what decides whether a flag consumes the token that follows it
// on the command line, so it must be known before parsing begins. That is why
// it lives in the declaration rather than being inferred while parsing.
type FlagType int

const (
	// Bool flags never consume the following token: "--verbose x" leaves x as
	// a positional argument. An explicit value still works as "--verbose=false".
	Bool FlagType = iota
	// String flags consume the following token verbatim.
	String
	// Int flags consume the following token and parse it as a base-10 integer.
	Int
)

// String renders the type as it appears in generated output.
//
// It is not a debugging aid: generated help prints "--priority int", and type
// errors report the type they expected, so this is part of the public surface.
func (t FlagType) String() string {
	switch t {
	case Bool:
		return "bool"
	case String:
		return "string"
	case Int:
		return "int"
	default:
		return "unknown"
	}
}

// Arity describes how many values a positional Arg accepts.
type Arity int

const (
	// One requires exactly one value; its absence is a usage error.
	One Arity = iota
	// ZeroOrOne accepts an optional single value.
	ZeroOrOne
	// Many accepts one or more values and absorbs every remaining positional,
	// which is why it is only valid on the last Arg of a command.
	Many
)
