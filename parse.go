package argvine

import (
	"fmt"
	"strconv"
	"strings"
)

// flagIndex is the set of flags visible at one point of the walk: the current
// command's own flags plus every persistent flag inherited from its ancestors.
//
// It is rebuilt from scratch on every descent rather than accumulated. That is
// what makes a non-persistent parent flag stop being visible in a child:
// nothing has to be removed, because only what should be visible is ever added.
type flagIndex struct {
	byName  map[string]Flag
	byShort map[string]Flag
}

func newFlagIndex() flagIndex {
	return flagIndex{
		byName:  make(map[string]Flag),
		byShort: make(map[string]Flag),
	}
}

// add registers flags under both their long and short names.
//
// The receiver is a value, not a pointer, and that is fine: the maps inside are
// reference types, so writes through a copy of the struct are visible to every
// other copy. Only reassigning a whole map field would need a pointer receiver.
func (x flagIndex) add(flags []Flag) {
	for _, f := range flags {
		x.byName[f.Name] = f
		// Guarding on the empty string matters: without it every flag without a
		// short form would overwrite the same "" key, and each would appear to
		// have registered a short alias.
		if f.Short != "" {
			x.byShort[f.Short] = f
		}
	}
}

// convert turns a raw command-line token into the flag's declared type.
//
// It takes the whole Flag rather than just its FlagType so it can name the flag
// in the error. The function that detects a problem is the one with the most
// context to describe it; making the caller rebuild that context would mean
// writing the same message in two places.
func convert(f Flag, raw string) (any, error) {
	switch f.Type {
	case String:
		return raw, nil

	case Int:
		n, err := strconv.Atoi(raw)
		if err != nil {
			// The wrapped strconv error is dropped on purpose: "parsing \"abc\":
			// invalid syntax" is about strconv, not about the user's command line.
			return nil, fmt.Errorf("invalid value %q for --%s: expected an integer", raw, f.Name)
		}
		return n, nil

	case Bool:
		// ParseBool is deliberately liberal — 1, t, T, TRUE, true, and their
		// false counterparts — which matches what people expect from --flag=1.
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q for --%s: expected true or false", raw, f.Name)
		}
		return b, nil

	default:
		// Unreachable for any tree that passed Validate. Reaching it means the
		// tree was never validated, which is a programming error, so it panics
		// rather than returning an error a user could see.
		panic(fmt.Sprintf("argvine: flag --%s has unknown type %d", f.Name, f.Type))
	}
}

// parser holds the mutable state of one walk over the tree.
//
// It exists so the step helpers do not have to thread half a dozen arguments
// between themselves. Note that all of this state is per-call: it lives here,
// never in the Command tree, which is what keeps Parse a pure function of
// (tree, argv).
type parser struct {
	ctx *Context
	// cur is the deepest command reached so far.
	cur *Command

	// visible is the flag index for cur, rebuilt on every descent.
	visible flagIndex
	// persistent accumulates every Persistent flag seen along the path, root
	// first. A descent seeds the new index from it.
	persistent []Flag
	// declared accumulates every flag declared along the walked path. It is the
	// set that gets default values seeded at the end, and it deliberately
	// excludes flags belonging to branches the walk never entered.
	declared []Flag
	// seen records which flags actually appeared in argv, so seedDefaults can
	// tell "absent" from "explicitly set to the zero value".
	seen map[string]bool

	// positional collects tokens that are neither flags nor subcommands.
	positional []string

	argv []string
	// i is the index of the next token to read. Helpers advance it by however
	// much they consumed, which is how a valued flag skips its own value.
	i int

	// afterTerminator latches on at the first "--" and never resets: from there
	// to the end of argv, every token is a positional regardless of its shape.
	afterTerminator bool
}

// Parse walks the tree consuming argv left to right and returns a fresh
// Context.
//
// It never prints and never terminates the process: on bad input it returns an
// error and lets the caller decide what to show and which exit code to use.
// A new Context per call is what allows two parses over the same tree to be
// independent, and what makes the table-driven tests possible.
func Parse(root *Command, argv []string) (*Context, error) {
	p := &parser{
		ctx:     newContext(),
		cur:     root,
		visible: newFlagIndex(),
		seen:    make(map[string]bool),
		argv:    argv,
	}

	p.ctx.Path = []*Command{root}
	p.visible.add(root.Flags)
	p.declared = append(p.declared, root.Flags...)
	p.collectPersistent(root)

	// One token at a time. step decides what the token is and how far to
	// advance, so the loop itself has no idea what a flag looks like.
	for p.i < len(p.argv) {
		if err := p.step(); err != nil {
			return nil, err
		}
	}

	p.ctx.Cmd = p.cur
	p.ctx.rawPositional = p.positional
	// Defaults are seeded last, so an explicit value on the command line is
	// never overwritten by the declaration it came from.
	p.seedDefaults()
	return p.ctx, nil
}

// step classifies the token at p.i and dispatches to the right handler.
//
// The order of the cases is the whole specification of the syntax, and it is
// not arbitrary: the terminator check must precede every pattern it disables,
// and "--" must be tested before the "--" prefix or it would parse as a long
// flag with an empty name.
//
// Only two pieces of state make position meaningful — afterTerminator and
// "no positional seen yet". Everything else is classified independently of what
// came before, which is exactly why flags and positionals can be interleaved
// freely and still produce the same result.
func (p *parser) step() error {
	tok := p.argv[p.i]

	switch {
	case p.afterTerminator:
		p.positional = append(p.positional, tok)
		p.i++
		return nil

	case tok == "--":
		p.afterTerminator = true
		p.i++
		return nil

	case strings.HasPrefix(tok, "--"):
		return p.longFlag()

	// A lone "-" is the conventional name for stdin, not a flag, so the length
	// guard keeps it out of shortGroup and lets it fall through as a positional.
	case len(tok) > 1 && strings.HasPrefix(tok, "-"):
		return p.shortGroup()

	default:
		// A subcommand only wins while no positional has been seen. Once the
		// user has started supplying arguments, a token that happens to match a
		// child's name is just another argument — "task add remote" adds a task
		// titled "remote".
		if sub := p.cur.findSub(tok); sub != nil && len(p.positional) == 0 {
			p.descend(sub)
			return nil
		}
		p.positional = append(p.positional, tok)
		p.i++
		return nil
	}
}

// longFlag parses a token beginning with "--", in either the "--name value" or
// the "--name=value" form.
func (p *parser) longFlag() error {
	tok := p.argv[p.i]
	// Cut splits on the first "=" only, so --message=a=b keeps "a=b" as the value.
	name, inline, hasInline := strings.Cut(tok[2:], "=")

	f, ok := p.visible.byName[name]
	if !ok {
		return fmt.Errorf("unknown flag --%s in %q", name, p.ctx.PathString())
	}

	// The declared type, never the next token, decides consumption. A parser
	// that guessed by looking ahead ("starts with a dash, so not a value") would
	// break on --offset -3 and on --message -v2-fix. The information needed is
	// not in the token; it is in the declaration, and it was known before
	// parsing began.
	if f.Type == Bool && !hasInline {
		p.set(f, true)
		p.i++
		return nil
	}

	raw, consumed := inline, 1
	if !hasInline {
		if p.i+1 >= len(p.argv) {
			return fmt.Errorf("flag --%s needs a value", f.Name)
		}
		// Two tokens consumed: the flag and its value.
		raw, consumed = p.argv[p.i+1], 2
	}

	v, err := convert(f, raw)
	if err != nil {
		return err
	}
	p.set(f, v)
	p.i += consumed
	return nil
}

// set records a parsed value and marks the flag as having appeared.
//
// Both writes happen here so they can never drift apart: a value recorded
// without its seen entry would be silently overwritten by seedDefaults.
func (p *parser) set(f Flag, v any) {
	p.ctx.flags[f.Name] = v
	p.seen[f.Name] = true
}

// collectPersistent appends a command's persistent flags to the running set
// that descendants will inherit.
func (p *parser) collectPersistent(c *Command) {
	for _, f := range c.Flags {
		if f.Persistent {
			p.persistent = append(p.persistent, f)
		}
	}
}

// seedDefaults fills in every flag declared along the walked path that did not
// appear in argv.
//
// This is why the typed accessors can panic on a missing key without being
// hostile: after Parse, every declared flag has an entry, so a missing key can
// only mean a name that was never declared — a typo, not an absent flag.
//
// A nil Default is not an error; it means the flag falls back to its type's
// zero value, which keeps trivial declarations short.
func (p *parser) seedDefaults() {
	for _, f := range p.declared {
		if p.seen[f.Name] {
			continue
		}
		if f.Default != nil {
			p.ctx.flags[f.Name] = f.Default
			continue
		}
		switch f.Type {
		case Bool:
			p.ctx.flags[f.Name] = false
		case String:
			p.ctx.flags[f.Name] = ""
		case Int:
			p.ctx.flags[f.Name] = 0
		}
	}
}

// shortGroup parses a token like "-abc", where every character is a short flag.
//
// The rule that makes the group unambiguous: bool flags chain freely, and the
// first non-bool flag ENDS the group and takes its value — from whatever is
// left in the token, or from the next token when nothing is left. This is the
// same convention as "tar -czf archive.tar.gz", where c and z are bools and f
// is valued.
//
// The user never has to know any flag's type. The order they type resolves it,
// and the only way to write something ambiguous is to write something that was
// already an error.
func (p *parser) shortGroup() error {
	chars := p.argv[p.i][1:]

	for j := 0; j < len(chars); j++ {
		ch := string(chars[j])

		f, ok := p.visible.byShort[ch]
		if !ok {
			return fmt.Errorf("unknown flag -%s in %q", ch, p.ctx.PathString())
		}

		// Bools do not end the group: keep walking the characters.
		if f.Type == Bool {
			p.set(f, true)
			continue
		}

		// Value glued to the flag: "-p1" or "-p=1". Note chars[j+1:] is safe at
		// the last index — it yields "", and TrimPrefix leaves it "".
		if rest := strings.TrimPrefix(chars[j+1:], "="); rest != "" {
			v, err := convert(f, rest)
			if err != nil {
				return err
			}
			p.set(f, v)
			p.i++
			return nil
		}

		// Nothing left in the token, so the value is the next one. Reaching here
		// with "-p -2" consumes "-2" as the value without ever classifying it,
		// which is why a negative number is never mistaken for a flag.
		if p.i+1 >= len(p.argv) {
			return fmt.Errorf("flag -%s needs a value", ch)
		}
		v, err := convert(f, p.argv[p.i+1])
		if err != nil {
			return err
		}
		p.set(f, v)
		p.i += 2
		return nil
	}

	// Only reached when every character was a bool: the group consumed exactly
	// one token.
	p.i++
	return nil
}

// descend moves the walk into a child command.
//
// The visible flag index is thrown away and rebuilt from scratch: inherited
// persistent flags first, then the child's own. Rebuilding rather than adding
// is what makes a non-persistent parent flag stop being visible here — nothing
// has to be removed, because only what should be visible is ever put in.
//
// In tree code, rebuilding is usually easier to reason about than removing:
// you do not have to know what to take out, only what to put in.
func (p *parser) descend(sub *Command) {
	p.cur = sub
	p.ctx.Path = append(p.ctx.Path, sub)

	p.visible = newFlagIndex()
	p.visible.add(p.persistent)
	p.visible.add(sub.Flags)

	// declared grows along the walked path only, so seedDefaults never seeds a
	// flag from a branch this argv did not enter.
	p.declared = append(p.declared, sub.Flags...)
	p.collectPersistent(sub)
	p.i++
}
