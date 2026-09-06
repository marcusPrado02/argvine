package argvine

import "testing"

// TestFlagString pins the strings that reach the user.
//
// These values are not internal labels: they are printed in generated help
// ("--priority int") and in type errors ("expected int"). Changing one changes
// the CLI's output, so the table is the contract, not a convenience.
func TestFlagString(t *testing.T) {
	tests := []struct {
		typ  FlagType
		want string
	}{
		{Bool, "bool"},
		{String, "string"},
		{Int, "int"},
	}

	for _, tt := range tests {
		// %d prints the numeric value even though FlagType is a Stringer:
		// numeric verbs ignore String(), which is what we want when the thing
		// under test is String() itself.
		if got := tt.typ.String(); got != tt.want {
			t.Errorf("FlagType(%d).String() = %q, want %q", tt.typ, got, tt.want)
		}
	}
}

// TestTreeIsInspectable checks the property the whole design rests on: a CLI
// is one composite literal, and everything about it can be read back out of
// that literal without running anything.
//
// It looks trivial, and that is the point. If this test ever needs a
// constructor call, a registration step or a second phase to wire parents to
// children, the model has stopped being declarative and help and completion
// can no longer be pure readers of the tree.
func TestTreeIsInspectable(t *testing.T) {
	root := &Command{
		// Lowercase on purpose in a real CLI: the name is the word typed at the
		// shell, and argv is case-sensitive.
		Name:  "task",
		Short: "local task manager",
		Flags: []Flag{
			{Name: "verbose", Short: "v", Type: Bool, Default: false,
				Usage: "verbose output", Persistent: true},
		},
		Sub: []*Command{
			{
				Name:  "add",
				Short: "create a task",
				Flags: []Flag{
					{Name: "priority", Short: "p", Type: Int, Default: 3,
						Usage: "priority from 1 (high) to 5 (low)."},
				},
				Args: []Arg{
					{Name: "title", Arity: One, Usage: "task title"},
				},
				Run: func(ctx *Context) error { return nil },
			},
		},
	}

	add := root.findSub("add")
	if add == nil {
		// Fatal, not Error: every assertion below dereferences add.
		t.Fatal("findSub(\"add\") returned nil, want the add command")
	}
	if got := add.Flags[0].Name; got != "priority" {
		t.Errorf("add.Flags[0].Name = %q, want \"priority\"", got)
	}
	// Arity has no String method, so %d is the honest verb here: the failure
	// message shows the raw constant value rather than a name that does not exist.
	if got := add.Args[0].Arity; got != One {
		t.Errorf("add.Args[0].Arity = %d, want One", got)
	}
	// The negative case matters as much as the positive one: a findSub that
	// returned the first child regardless of name would pass every check above.
	if root.findSub("nope") != nil {
		t.Error("findSub(\"nope\") should be nil")
	}
}

// TestValidateRejectBadFlags enumerates every way a flag declaration can be
// wrong. Each case is a tree a developer could plausibly write by accident.
//
// The assertion is only "an error came back", not which message: the messages
// are for humans and will be reworded, while the set of rejected shapes is the
// actual contract. Pinning the text here would make every wording improvement
// look like a behavior change.
func TestValidateRejectBadFlags(t *testing.T) {
	tests := []struct {
		name string
		root *Command
	}{
		{
			name: "duplicate long name",
			root: &Command{Name: "app", Flags: []Flag{
				{Name: "force", Type: Bool},
				{Name: "force", Type: Bool},
			}},
		},
		{
			name: "short flag longer than one character",
			root: &Command{Name: "app", Flags: []Flag{
				{Name: "force", Short: "fo", Type: Bool},
			}},
		},
		{
			name: "duplicate short name",
			root: &Command{Name: "app", Flags: []Flag{
				{Name: "force", Short: "f", Type: Bool},
				{Name: "fast", Short: "f", Type: Bool},
			}},
		},
		{
			name: "choices on a bool flag",
			root: &Command{Name: "app", Flags: []Flag{
				{Name: "force", Type: Bool, Choices: []string{"yes", "no"}},
			}},
		},
		{
			name: "default type does not match flag type",
			root: &Command{Name: "app", Flags: []Flag{
				{Name: "priority", Type: Int, Default: "high"},
			}},
		},
		{
			name: "empty flag name",
			root: &Command{Name: "app", Flags: []Flag{
				{Name: "", Type: Bool},
			}},
		},
	}

	for _, tt := range tests {
		// A subtest per case so a failure names the shape that slipped through,
		// instead of reporting "one of six trees was accepted".
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.root.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want an error for %q", tt.name)
			}
		})
	}
}

// TestValidateAcceptWellFormedTree is the counterweight to the table above.
//
// Without it, a Validate that returned an error unconditionally would pass
// every negative case. Any suite built from rejections needs at least one
// acceptance, or it is only testing that the function is pessimistic.
func TestValidateAcceptWellFormedTree(t *testing.T) {
	root := &Command{
		Name: "app",
		Flags: []Flag{
			{Name: "verbose", Short: "v", Type: Bool, Default: false},
		},
		Sub: []*Command{
			{Name: "serve", Flags: []Flag{
				{Name: "port", Short: "p", Type: Int, Default: 8080},
			}},
		},
	}

	if err := root.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

// TestValidateRejectsBadArgsAndSubs covers the rules about shape rather than
// about flags: positional arity, subcommand names, and the fact that validate
// now walks the whole tree instead of only the root.
//
// The last two cases carry the most weight. "invalid flag inside a nested
// subcommand" fails only if the recursion reaches depth two, and "subcommand
// short flag collides with an inherited persistent flag" fails only if
// persistent flags are actually threaded down. Both would pass silently
// against the non-recursive validate written in the previous task, which is
// what makes them a real test of this one.
func TestValidateRejectsBadArgsAndSubs(t *testing.T) {
	tests := []struct {
		name string
		root *Command
	}{
		{
			name: "Many arity not on the last arg",
			root: &Command{Name: "app", Args: []Arg{
				{Name: "ids", Arity: Many},
				{Name: "target", Arity: One},
			}},
		},
		{
			name: "empty arg name",
			root: &Command{Name: "app", Args: []Arg{
				{Name: "", Arity: One},
			}},
		},
		{
			name: "duplicate subcommand name",
			root: &Command{Name: "app", Sub: []*Command{
				{Name: "serve"},
				{Name: "serve"},
			}},
		},
		{
			name: "invalid flag inside a nested subcommand",
			root: &Command{Name: "app", Sub: []*Command{
				{Name: "remote", Sub: []*Command{
					{Name: "add", Flags: []Flag{{Name: "force", Short: "fo", Type: Bool}}},
				}},
			}},
		},
		{
			name: "subcommand short flag collides with an inherited persistent flag",
			root: &Command{
				Name:  "app",
				Flags: []Flag{{Name: "verbose", Short: "v", Type: Bool, Persistent: true}},
				Sub: []*Command{
					{Name: "serve", Flags: []Flag{{Name: "version", Short: "v", Type: Bool}}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.root.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want an error for %s", tt.name)
			}
		})
	}
}

// TestValidateSiblingsDoNotShareInheritance pins the direction of inheritance:
// flags flow down from a parent, never sideways between siblings.
//
// build declares -o as persistent and serve declares its own -o. They are only
// a collision if serve can somehow see build's flags — which it must not, since
// nobody typing "app serve" has any way to reach a flag that belongs to
// "app build".
//
// This is a regression guard, not a bug reproduction. It passes against the
// naive `next := inherited` too, because each child receives its own slice
// length and never reads past it. It is here to fail loudly if someone later
// "optimizes" the inheritance by sharing a single accumulator across the walk.
func TestValidateSiblingsDoNotShareInheritance(t *testing.T) {
	root := &Command{
		Name:  "app",
		Flags: []Flag{{Name: "verbose", Short: "v", Type: Bool, Persistent: true}},
		Sub: []*Command{
			{Name: "build", Flags: []Flag{{Name: "output", Short: "o", Type: String, Persistent: true}}},
			{Name: "serve", Flags: []Flag{{Name: "open", Short: "o", Type: Bool}}},
		},
	}

	if err := root.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil: -o on serve must not collide with -o on its sibling build", err)
	}
}
