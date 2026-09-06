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
