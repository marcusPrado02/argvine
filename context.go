package argvine

import (
	"fmt"
	"strings"
)

// Context is the result of a successful Parse.
//
// A fresh Context is produced by every call to Parse, so two parses over the
// same tree never contaminate each other. That is the whole reason parsed
// values live here instead of being written back into the Flag declarations.
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

// newContext returns an empty Context with both maps ready to write into.
//
// Unexported: a Context is only ever produced by Parse. Handing callers a
// constructor would invite them to build one by hand and expect it to behave
// like a parsed one.
func newContext() *Context {
	return &Context{
		flags: make(map[string]any),
		args:  make(map[string][]string),
	}
}

// PathString renders the full invocation path, e.g. "task remote add".
//
// Every diagnostic in the package goes through this rather than through
// Cmd.Name, so a message never names a subcommand without saying where it
// lives in the tree.
func (c *Context) PathString() string {
	names := make([]string, len(c.Path))
	for i, cmd := range c.Path {
		names[i] = cmd.Name
	}
	return strings.Join(names, " ")
}

// Bool returns the value of a bool flag.
//
// It panics when no such flag is visible on the parsed command, and when the
// flag is not a bool. Both are programming errors in the CLI that uses
// argvine, never usage errors by whoever ran it: the name is written by the
// same person who declared the flag, so a mismatch means the two drifted apart
// and should fail on the developer's first run rather than silently yield a
// zero value in production.
func (c *Context) Bool(name string) bool {
	v := c.lookupFlag(name)
	b, ok := v.(bool)
	if !ok {
		panic(fmt.Sprintf("argvine: flag --%s on %q is %T, not bool", name, c.PathString(), v))
	}
	return b
}

// Int returns the value of an int flag. See Bool for the panic contract.
func (c *Context) Int(name string) int {
	v := c.lookupFlag(name)
	n, ok := v.(int)
	if !ok {
		panic(fmt.Sprintf("argvine: flag --%s on %q is %T, not int", name, c.PathString(), v))
	}
	return n
}

// String returns the value of a string flag. See Bool for the panic contract.
func (c *Context) String(name string) string {
	v := c.lookupFlag(name)
	s, ok := v.(string)
	if !ok {
		panic(fmt.Sprintf("argvine: flag --%s on %q is %T, not string", name, c.PathString(), v))
	}
	return s
}

// Arg returns the first value of a positional argument.
//
// An argument declared with ZeroOrOne arity and absent from the command line
// yields "" — that is a legitimate outcome, not an error, which is why this
// does not panic on an empty slice. Only an undeclared name panics.
func (c *Context) Arg(name string) string {
	vs := c.lookupArg(name)
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

// ArgList returns every value of a positional argument. It is the accessor for
// an Arg declared with Many arity; on any other arity it yields at most one
// element.
func (c *Context) ArgList(name string) []string {
	return c.lookupArg(name)
}

// lookupFlag is the single place a missing flag turns into a panic, so the
// three typed accessors above stay short and cannot disagree about the message.
func (c *Context) lookupFlag(name string) any {
	v, ok := c.flags[name]
	if !ok {
		panic(fmt.Sprintf("argvine: no flag --%s visible on %q", name, c.PathString()))
	}
	return v
}

// lookupArg is the equivalent for positionals. The angle brackets match how
// the argument is rendered in the generated usage line, so a panic message and
// the help output name the same thing the same way.
func (c *Context) lookupArg(name string) []string {
	v, ok := c.args[name]
	if !ok {
		panic(fmt.Sprintf("argvine: no argument <%s> declared on %q", name, c.PathString()))
	}
	return v
}
