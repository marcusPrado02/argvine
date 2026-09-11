package argvine

import (
	"errors"
	"strings"
	"testing"
)

// TestUsageErrorsCarryPathAndHint checks the contract every usage error shares,
// rather than the wording of any one of them.
//
// Three assertions, three different things:
//
//   - errors.As succeeds → the caller can branch on the specific failure, which
//     is the whole reason these are types instead of fmt.Errorf strings
//   - the message mentions --help → it tells the user what to do next
//   - the message has a newline → the two-part shape survived
//
// None of them pins a phrase. Wording will be rewritten; the shape is contract.
//
// Note errors.As needs a pointer to the target type, so target holds a
// **ErrUnknownFlag — new(*ErrUnknownFlag), not new(ErrUnknownFlag).
func TestUsageErrorsCarryPathAndHint(t *testing.T) {
	root := remoteTree()

	tests := []struct {
		name   string
		argv   []string
		target any
	}{
		{name: "unknown flag", argv: []string{"remote", "add", "--nope"}, target: new(*ErrUnknownFlag)},
		// --config is declared on the root and is the last token, so there is
		// nothing left to consume as its value.
		{name: "missing value", argv: []string{"--config"}, target: new(*ErrMissingValue)},
		// "maybe" is not a bool. The inline form is used so the failure is in
		// conversion and not in a missing value.
		{name: "bad type", argv: []string{"remote", "add", "--force=maybe", "a", "b"}, target: new(*ErrBadType)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(root, tt.argv)
			if err == nil {
				t.Fatalf("Parse(%v) = nil error, want one", tt.argv)
			}
			if !errors.As(err, tt.target) {
				t.Fatalf("Parse(%v) returned %T, want %T", tt.argv, err, tt.target)
			}

			msg := err.Error()
			if !strings.Contains(msg, "--help") {
				t.Errorf("message %q should point the user at --help", msg)
			}
			if strings.Count(msg, "\n") == 0 {
				t.Errorf("message %q should have a what-happened line and a what-to-do-next line", msg)
			}
		})
	}
}

// TestUnknownFlagNamesTheDeepestCommand is the one test here that reaches into
// a concrete type, and it earns that by checking the two fields a caller would
// actually want to branch on.
//
// "task remote add" rather than "add" is the point: the error was built from
// the walk's Path at the moment of failure, not from the leaf's name. An
// implementation that reported Cmd.Name would pass every other test in this
// file and still leave the user hunting for which "add" was meant.
func TestUnknownFlagNamesTheDeepestCommand(t *testing.T) {
	_, err := Parse(remoteTree(), []string{"remote", "add", "--nope"})

	var uf *ErrUnknownFlag
	if !errors.As(err, &uf) {
		t.Fatalf("got %T, want *ErrUnknownFlag", err)
	}
	if got := uf.PathString(); got != "task remote add" {
		t.Errorf("PathString() = %q, want \"task remote add\"", got)
	}
	if uf.Flag != "--nope" {
		t.Errorf("Flag = %q, want \"--nope\"", uf.Flag)
	}
}
