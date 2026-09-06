// Package argvine is a dependency-free command-line framework for Go.
//
// A CLI is described as an immutable tree of Command nodes. Parsing, help
// rendering and shell completion are three independent readers of that same
// tree, which is what keeps them from ever disagreeing with each other: a flag
// added to the tree shows up in all three without anyone editing three places.
package argvine

import (
	"fmt"
)

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

// Validate walks the tree and reports the first structural problem it finds.
//
// Call it once at startup, before any Parse. Every error it returns is a
// programming error in the CLI definition, never a usage error by whoever ran
// the CLI — which is exactly why it lives outside Parse. Run inside Parse, a
// malformed tree would come back as an ordinary error, indistinguishable from
// a mistyped flag, and the user would be shown a message only the developer
// could act on.
//
// It is also the safety net for the one weak spot of the Context design:
// flag names are strings, so ctx.Int("prot") only fails at runtime. Validate
// catches the declaration-side half of that class of mistake at boot.
func (c *Command) Validate() error {
	return c.validate(nil)
}

// validate is the recursive half of Validate. inherited carries the persistent
// flags accumulated from every ancestor, so a child can be checked against
// names it did not declare but will nonetheless see.
//
// It returns the first problem rather than collecting all of them: this runs at
// startup and stops the program, so the developer fixes one and re-runs. A list
// of errors would cost complexity for no gain in that loop.
func (c *Command) validate(inherited []Flag) error {
	// Seed the seen-sets with what this command inherits, so a child that
	// redeclares an ancestor's persistent flag is caught as a duplicate.
	seenName := make(map[string]bool, len(inherited)+len(c.Flags))
	seenShort := make(map[string]bool, len(inherited)+len(c.Flags))
	for _, f := range inherited {
		seenName[f.Name] = true
		if f.Short != "" {
			seenShort[f.Short] = true
		}
	}

	for _, f := range c.Flags {
		if f.Name == "" {
			return fmt.Errorf("argvine: command %q declares a flag with an empty name", c.Name)
		}
		// One message covers both duplicate and shadowing because the fix is
		// the same in either case: rename one of the two.
		if seenName[f.Name] {
			return fmt.Errorf("argvine: command %q declares --%s twice, or shadows an inherited flag of the same name", c.Name, f.Name)
		}
		seenName[f.Name] = true

		if f.Short != "" {
			// A multi-character Short would be indistinguishable from a group
			// of single-character flags once parsing starts: "-fo" has to mean
			// -f -o, so it can never also mean a flag named "fo".
			if len(f.Short) != 1 {
				return fmt.Errorf("argvine: command %q flag --%s has short %q; short flags are exactly one character", c.Name, f.Name, f.Short)
			}
			if seenShort[f.Short] {
				return fmt.Errorf("argvine: command %q reuses short flag -%s", c.Name, f.Short)
			}
			seenShort[f.Short] = true
		}

		// A bool flag has exactly two possible values, so restricting them is
		// either a no-op or a contradiction. Reaching for Choices here almost
		// always means the Type field was meant to be String.
		if f.Type == Bool && f.Choices != nil {
			return fmt.Errorf("argvine: command %q flag --%s is bool but declares Choices", c.Name, f.Name)
		}
		if err := checkDefault(c.Name, f); err != nil {
			return err
		}
	}

	for i, a := range c.Args {
		if a.Name == "" {
			return fmt.Errorf("argvine: command %q declares a positional with an empty name", c.Name)
		}
		// Many absorbs every remaining token, so anything declared after it
		// could never be filled. And there is no rule that would make the shape
		// unambiguous anyway: with {tags: Many}, {name: One} and three tokens,
		// both "tags=[a] name=b" and "tags=[a b] name=c" are defensible.
		// Rejecting the declaration is cheaper than inventing a tie-breaker.
		if a.Arity == Many && i != len(c.Args)-1 {
			return fmt.Errorf("argvine: command %q declares <%s> with Many arity but it is not the last positional", c.Name, a.Name)
		}
	}

	seenSub := make(map[string]bool, len(c.Sub))
	for _, s := range c.Sub {
		if s.Name == "" {
			return fmt.Errorf("argvine: command %q declares a subcommand with an empty name", c.Name)
		}
		// Duplicates would make routing depend on declaration order, which is
		// invisible to the person typing the command.
		if seenSub[s.Name] {
			return fmt.Errorf("argvine: command %q declares subcommand %q twice", c.Name, s.Name)
		}
		seenSub[s.Name] = true
	}

	// Build what the children inherit: whatever this node already inherited,
	// plus its own persistent flags.
	//
	// Copy rather than `next := inherited`. The naive version appends into the
	// caller's backing array whenever it has spare capacity, which makes this
	// function's correctness depend on how a caller several frames up built its
	// slice. It happens not to produce a wrong read in this particular walk —
	// each child receives its own length and never looks past it — but owning
	// the array is what keeps the reasoning local, and this runs once at startup.
	next := make([]Flag, len(inherited), len(inherited)+len(c.Flags))
	copy(next, inherited)
	for _, f := range c.Flags {
		if f.Persistent {
			next = append(next, f)
		}
	}

	// Recursing last means a node's own problems are reported before its
	// children's, so the first error points at the outermost mistake.
	for _, s := range c.Sub {
		if err := s.validate(next); err != nil {
			return err
		}
	}
	return nil
}

// checkDefault verifies that a flag's Default matches its declared Type.
//
// Default is `any`, so nothing stops {Type: Int, Default: "high"} from
// compiling. Left unchecked it would surface much later as a panic inside
// Context.Int, at a call site that has nothing to do with the declaration that
// caused it. Catching it here turns a confusing runtime panic into a startup
// message that names the command and the flag.
//
// A nil Default is legal: it means "no default", and Parse seeds the type's
// zero value instead.
func checkDefault(cmdName string, f Flag) error {
	if f.Default == nil {
		return nil
	}

	// The comma-ok form of the type assertion is the whole check: the value is
	// discarded, only the assertion's success matters.
	var ok bool
	switch f.Type {
	case Bool:
		_, ok = f.Default.(bool)
	case String:
		_, ok = f.Default.(string)
	case Int:
		_, ok = f.Default.(int)
	}
	if !ok {
		// %s prints the declared type via FlagType.String, %T the actual one:
		// "is int but its Default is string" reads as the fix itself.
		return fmt.Errorf("argvine: command %q flag --%s is %s but its Default is %T", cmdName, f.Name, f.Type, f.Default)
	}
	return nil
}
