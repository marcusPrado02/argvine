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
