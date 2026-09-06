package argvine

// Context is the result of a successful Parse.
//
// A fresh Context is produced by every call to Parse, so two parses over the
// same tree never contaminate each other. That is the whole reason parsed
// values live here instead of being written back into the Flag declarations.
//
// This is the skeleton the tree needs in order to compile; the typed accessors
// that read flags and args are added next.
type Context struct {
	// Cmd is the command that won the routing: the deepest node reached.
	Cmd *Command
	// Path is the chain from the root down to Cmd. Help and error messages use
	// it to say "task remote add" instead of a bare "add".
	Path []*Command

	// flags maps a long flag name to its already-converted value. Unexported so
	// every read goes through the typed accessors, which are the surface the
	// CLI author is meant to use.
	flags map[string]any
	// args maps a positional Arg name to its values. A slice even for single
	// values, so Many needs no separate storage.
	args map[string][]string
}
