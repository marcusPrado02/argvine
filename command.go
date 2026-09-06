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

// Flag describes a named option.
//
// A Flag is immutable description: parsing never writes into it, and every
// parsed value lands in a Context instead. That separation between description
// and result is what makes Parse a pure function, and it is why Command holds
// []Flag by value rather than []*Flag.
type Flag struct {
	// Name is the long form, written without dashes: "force" renders as --force.
	Name string
	// Short is the single-character form, without the dash. Empty means the
	// flag has no short form.
	Short string
	// Type decides whether this flag consumes the token that follows it.
	Type FlagType
	// Default is the value used when the flag is absent. Its dynamic type must
	// match Type; Validate rejects the mismatch at startup.
	Default any
	// Usage is the one-line description rendered in generated help.
	Usage string
	// Required makes the flag mandatory on the command that declares it.
	Required bool
	// Choices restricts the accepted values. It is validated only when non-nil,
	// and is meaningless on a Bool flag, which Validate rejects.
	Choices []string
	// Persistent makes the flag visible to every descendant command, so a root
	// --verbose can be written after a deeply nested subcommand.
	Persistent bool
}

// Arg describes a positional argument.
type Arg struct {
	// Name is both the lookup key for Context.Arg and the placeholder rendered
	// as <name> in the usage line.
	Name string
	// Arity decides how many values this argument consumes.
	Arity Arity
	// Usage is the one-line description rendered in generated help.
	Usage string
	// Choices restricts the accepted values, the same way it does on a Flag.
	// Not in the v1 design; wire it into validation or drop it.
	Choices []string
}

// Command is a node in the command tree.
//
// The whole CLI is meant to be written as a single composite literal: a node
// holds its children directly, and no node points back at its parent. Keeping
// the tree acyclic is what lets Validate walk it without cycle detection and
// what keeps the literal declarative.
type Command struct {
	// Name is the word that selects this command on the command line.
	Name string
	// Short is the one-line description shown when the parent lists its children.
	Short string
	// Long is the paragraph shown in this command's own help. Help falls back
	// to Short when Long is empty.
	Long string
	// Flags are the options declared directly on this command.
	Flags []Flag
	// Args are the positional arguments this command accepts.
	Args []Arg
	// Sub are the child commands.
	Sub []*Command
	// Run is the handler. A node with children and no Run is a namespace: the
	// caller is expected to print help for it rather than execute anything.
	Run func(*Context) error
}

// findSub returns the direct child named name, or nil when there is none.
//
// A linear scan is deliberate: command trees have a handful of children per
// node, and a map would have to be built and kept in sync with Sub, which is
// the kind of duplicated state this design exists to avoid.
func (c *Command) findSub(name string) *Command {
	for _, s := range c.Sub {
		if s.Name == name {
			return s
		}
	}
	return nil
}
