package argvine

import (
	"fmt"
	"strings"
)

// This file holds every error a *user* of a CLI can cause. Errors caused by
// the *author* of a CLI — a malformed tree — stay in command.go as plain
// fmt.Errorf values, because nothing needs to inspect or re-render them.
//
// The split is visible in the code: parse.go returns only types from this file,
// command.go returns only plain errors.

// usageError is embedded in every usage error below.
//
// Eight error types need the same field and the same three methods. Repeating
// them eight times is where the ninth one forgets a case; an interface would
// describe the behaviour without providing it. Embedding gives both at once —
// each type gets the field and the methods for free, and any interface they
// need to satisfy later comes along without them knowing.
//
// It is unexported on purpose: it is an implementation detail, not API. A CLI
// author sees *ErrUnknownFlag and can call PathString on it, and never has to
// learn where that method came from.
type usageError struct {
	// Path is the chain of commands the walk had reached when the error
	// happened, which is what lets a message say "task remote add" rather than
	// a bare "add" the user cannot locate.
	Path []*Command
}

// PathString renders the full invocation path, e.g. "task remote add".
func (u usageError) PathString() string {
	names := make([]string, len(u.Path))
	for i, c := range u.Path {
		names[i] = c.Name
	}
	return strings.Join(names, " ")
}

// hint is the second half every usage message ends with.
//
// Every error in this file has the same two-part shape, and the split is not
// cosmetic: the first line says what happened, the second says what to do next.
// An error that only diagnoses leaves the user stuck.
//
//	unknown flag --nope in "task remote add"
//
//	run "task remote add --help" for usage
func (u usageError) hint() string {
	return fmt.Sprintf("\n\nrun %q for usage", u.PathString()+" --help")
}

// ErrUnknownFlag reports a flag that is not visible on the parsed command.
//
// Flag is the token exactly as typed, dashes included, so the message can echo
// "--nope" or "-z" back without the parser having to remember which form the
// user wrote.
type ErrUnknownFlag struct {
	usageError
	Flag string
}

func (e *ErrUnknownFlag) Error() string {
	return fmt.Sprintf("unknown flag %s in %q%s", e.Flag, e.PathString(), e.hint())
}

// ErrMissingValue reports a non-bool flag that ended the command line with
// nothing after it.
//
// It carries the whole Flag rather than just its name so the message can name
// the type that was expected — "needs a int value" tells the user what to type
// next, where "needs a value" only tells them something is missing.
type ErrMissingValue struct {
	usageError
	Flag Flag
}

func (e *ErrMissingValue) Error() string {
	return fmt.Sprintf("flag --%s needs a %s value%s", e.Flag.Name, e.Flag.Type, e.hint())
}

// ErrBadType reports a value that does not convert to the flag's declared type.
//
// This is the error convert cannot build itself: convert knows the value is
// wrong but not where on the command tree it appeared, which is why it returns
// a bool and lets the caller — who has the path — construct this.
type ErrBadType struct {
	usageError
	Flag Flag
	Got  string
}

func (e *ErrBadType) Error() string {
	return fmt.Sprintf("invalid value %q for --%s: expected %s%s", e.Got, e.Flag.Name, e.Flag.Type, e.hint())
}

// ErrMissingRequired reports a required flag that never appeared.
//
// It can only be raised after the whole command line has been read: until the
// last token, "this flag was never given" is not yet a true statement.
type ErrMissingRequired struct {
	usageError
	Flag Flag
}

func (e *ErrMissingRequired) Error() string {
	return fmt.Sprintf("required flag --%s is missing%s", e.Flag.Name, e.hint())
}

// ErrBadChoice reports a value outside the flag's Choices set.
//
// The message lists the accepted values rather than just rejecting: a user who
// typed "closed" instead of "done" should not have to go find --help to learn
// the three words that were allowed.
type ErrBadChoice struct {
	usageError
	Flag Flag
	Got  string
}

func (e *ErrBadChoice) Error() string {
	return fmt.Sprintf("invalid value %q for --%s: expected one of %s%s",
		e.Got, e.Flag.Name, strings.Join(e.Flag.Choices, ", "), e.hint())
}

// ErrMissingArg reports a positional argument whose arity was not satisfied.
//
// The angle brackets match how the argument is rendered in the generated usage
// line, so the error and the help name the same thing the same way.
type ErrMissingArg struct {
	usageError
	Arg Arg
}

func (e *ErrMissingArg) Error() string {
	return fmt.Sprintf("missing required argument <%s>%s", e.Arg.Name, e.hint())
}

// ErrUnknownCommand reports a leftover token on a command that has
// subcommands, where the token was most likely meant to be one of them.
//
// This and ErrUnexpectedArg describe the same raw situation — a positional
// nobody claimed — and are separate types because the user's next move differs.
// Here it is "I got the subcommand name wrong, show me the list"; in
// ErrUnexpectedArg it is "the command was right, I passed one argument too
// many". A single error would say "unexpected token" for both and leave the
// user to work out which problem they have.
type ErrUnknownCommand struct {
	usageError
	Got string
}

func (e *ErrUnknownCommand) Error() string {
	return fmt.Sprintf("unknown command %q for %q%s", e.Got, e.PathString(), e.hint())
}

// ErrUnexpectedArg reports a leftover token on a command that declares no
// further positionals and has no subcommands.
type ErrUnexpectedArg struct {
	usageError
	Got string
}

func (e *ErrUnexpectedArg) Error() string {
	return fmt.Sprintf("unexpected argument %q for %q%s", e.Got, e.PathString(), e.hint())
}
