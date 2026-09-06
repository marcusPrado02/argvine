package argvine

import "testing"

// TestContextTypedAccessors builds a Context by hand and reads it back.
//
// Reaching into the unexported maps is deliberate: Parse does not exist yet,
// and the accessors have to be correct before anything fills them. An
// in-package test is the only thing that can do this, which is one reason the
// test file shares the package instead of being argvine_test.
func TestContextTypedAccessors(t *testing.T) {
	root := &Command{Name: "task"}
	add := &Command{Name: "add"}

	ctx := newContext()
	ctx.Cmd = add
	ctx.Path = []*Command{root, add}
	ctx.flags["verbose"] = true
	ctx.flags["priority"] = 2
	ctx.flags["status"] = "done"
	ctx.args["title"] = []string{"buy bread"}
	ctx.args["ids"] = []string{"1", "2", "3"}

	if got := ctx.Bool("verbose"); got != true {
		t.Errorf("Bool(\"verbose\") = %v, want true", got)
	}
	if got := ctx.Int("priority"); got != 2 {
		t.Errorf("Int(\"priority\") = %v, want 2", got)
	}
	if got := ctx.String("status"); got != "done" {
		t.Errorf("String(\"status\") = %q, want \"done\"", got)
	}
	if got := ctx.Arg("title"); got != "buy bread" {
		t.Errorf("Arg(\"title\") = %q, want \"buy bread\"", got)
	}
	// ArgList is checked element by element rather than with reflect.DeepEqual
	// so a failure names which position went wrong.
	if got := ctx.ArgList("ids"); len(got) != 3 || got[0] != "1" || got[1] != "2" || got[2] != "3" {
		t.Errorf("ArgList(\"ids\") = %v, want [\"1\", \"2\", \"3\"]", got)
	}
	if got := ctx.PathString(); got != "task add" {
		t.Errorf("PathString() = %q, want \"task add\"", got)
	}
}

// TestContextUnknownFlagPanics pins the decision that a misspelled flag name
// is a programming error, not a usage error.
//
// Returning the zero value instead would hide the bug: the CLI would run, the
// wrong value would flow through, and the mistake would surface far from its
// cause. Failing loudly on the developer's first run is the point.
func TestContextUnknownFlagPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Int on unknown flag did not panic; it is a programming error, not a usage error")
		}
	}()

	add := &Command{Name: "add"}
	ctx := newContext()
	ctx.Cmd = add
	// Path is set so the panic message names the command; without it the
	// message would read `on ""`, which helps nobody.
	ctx.Path = []*Command{{Name: "task"}, add}

	_ = ctx.Int("prot")
}

// TestContextWrongTypePanics covers the second half of the same contract: the
// flag exists, but the accessor asks for the wrong type.
//
// This is what a typo in the tree costs — declaring a flag as String and
// reading it with Int — and it must fail as loudly as an unknown name.
func TestContextWrongTypePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Int on string flag did not panic; it is a programming error, not a usage error")
		}
	}()

	add := &Command{Name: "add"}
	ctx := newContext()
	ctx.Cmd = add
	ctx.Path = []*Command{{Name: "task"}, add}
	ctx.flags["status"] = "done"

	_ = ctx.Int("status")
}
