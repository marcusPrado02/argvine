package argvine

import (
	"testing"
)

// TestConvert covers the one place a raw command-line token becomes a typed
// value. Everything downstream trusts it, so the edge cases live here rather
// than being retested through Parse.
//
// "negative int parses" is the case worth staring at: -3 is a perfectly good
// value, which is why the parser must never decide "starts with a dash, so it
// cannot be a value".
func TestConvert(t *testing.T) {
	tests := []struct {
		name    string
		flag    Flag
		raw     string
		want    any
		wantErr bool
	}{
		{name: "string passes through", flag: Flag{Name: "host", Type: String}, raw: "localhost", want: "localhost"},
		{name: "int parses", flag: Flag{Name: "port", Type: Int}, raw: "8080", want: 8080},
		{name: "negative int parses", flag: Flag{Name: "offset", Type: Int}, raw: "-3", want: -3},
		{name: "int rejects letters", flag: Flag{Name: "port", Type: Int}, raw: "abc", wantErr: true},
		{name: "bool accepts true", flag: Flag{Name: "force", Type: Bool}, raw: "true", want: true},
		{name: "bool accepts 0", flag: Flag{Name: "force", Type: Bool}, raw: "0", want: false},
		{name: "bool rejects garbage", flag: Flag{Name: "force", Type: Bool}, raw: "maybe", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convert(tt.flag, tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("convert(%v, %q) = %v, want error", tt.flag.Type, tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("convert(%v, %q) returned %v", tt.flag.Type, tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("convert(%v, %q) = %v (%T), want %v (%T)", tt.flag.Type, tt.raw, got, got, tt.want, tt.want)
			}
		})
	}

}

// TestFlagIndex checks the lookup structure the parser consults for every
// token. The third assertion is the one that earns its place: a flag with no
// short form must not register an empty key, or the first such flag would
// claim "" and every later one would silently overwrite it.
func TestFlagIndex(t *testing.T) {
	idx := newFlagIndex()
	idx.add([]Flag{
		{Name: "verbose", Short: "v", Type: Bool},
		{Name: "output", Type: String},
	})

	if _, ok := idx.byName["verbose"]; !ok {
		t.Error("byName should contain \"verbose\"")
	}
	if _, ok := idx.byShort["v"]; !ok {
		t.Error("byShort should contain \"v\"")
	}
	if _, ok := idx.byShort[""]; ok {
		t.Error("a flag without a short form must not register an empty short key")
	}
}

// TestParseLongFlags drives Parse end to end over one flat command.
//
// One tree, many argv lines: this is the shape the whole design was chosen to
// allow. It only works because Parse writes nothing back into root, so the
// nine cases below share a tree without contaminating each other. Had flags
// been parsed into pre-allocated pointers, each case would need its own tree.
//
// Every case asserts all three flags, not just the one it exercises, so a
// change that leaks a value across flags cannot hide.
func TestParseLongFlags(t *testing.T) {
	root := &Command{
		Name: "app",
		Flags: []Flag{
			{Name: "force", Short: "f", Type: Bool, Default: false},
			{Name: "host", Type: String, Default: "localhost"},
			{Name: "port", Type: Int, Default: 8080},
		},
	}

	tests := []struct {
		name      string
		argv      []string
		wantForce bool
		wantHost  string
		wantPort  int
		wantErr   bool
	}{
		{name: "defaults when empty", argv: nil, wantForce: false, wantHost: "localhost", wantPort: 8080},
		{name: "bool sets true", argv: []string{"--force"}, wantForce: true, wantHost: "localhost", wantPort: 8080},
		{name: "space separated value", argv: []string{"--port", "9090"}, wantPort: 9090, wantHost: "localhost"},
		{name: "equals separated value", argv: []string{"--port=9090"}, wantPort: 9090, wantHost: "localhost"},
		{name: "explicit bool value", argv: []string{"--force=false"}, wantForce: false, wantHost: "localhost", wantPort: 8080},
		{name: "several flags", argv: []string{"--force", "--host", "example.com", "--port=1"}, wantForce: true, wantHost: "example.com", wantPort: 1},
		{name: "unknown flag", argv: []string{"--nope"}, wantErr: true},
		{name: "missing value", argv: []string{"--port"}, wantErr: true},
		{name: "bad type", argv: []string{"--port", "abc"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := Parse(root, tt.argv)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%v) = nil error, want error", tt.argv)
				}
				// The error case ends here: there is no ctx to inspect, and
				// falling through would hit the success assertions below.
				return
			}
			if err != nil {
				t.Fatalf("Parse(%v) returned %v", tt.argv, err)
			}
			if got := ctx.Bool("force"); got != tt.wantForce {
				t.Errorf("force = %v, want %v", got, tt.wantForce)
			}
			if got := ctx.String("host"); got != tt.wantHost {
				t.Errorf("host = %q, want %q", got, tt.wantHost)
			}
			if got := ctx.Int("port"); got != tt.wantPort {
				t.Errorf("port = %d, want %d", got, tt.wantPort)
			}
		})
	}
}

// TestParseShortFlags covers the grouping rules, which are the fiddliest part
// of the parser and the easiest to get subtly wrong.
//
// The two cases that matter most are the last kind: "-p -2" must read -2 as a
// value, and "-pabc" must read "abc" as a value that then fails conversion.
// Both fall out of the same rule — the first non-bool ends the group — and a
// parser that instead looked at whether a token starts with a dash would get
// one of them wrong no matter which way it decided.
func TestParseShortFlags(t *testing.T) {
	root := &Command{
		Name: "app",
		Flags: []Flag{
			{Name: "all", Short: "a", Type: Bool, Default: false},
			{Name: "brief", Short: "b", Type: Bool, Default: false},
			{Name: "color", Short: "c", Type: Bool, Default: false},
			{Name: "priority", Short: "p", Type: Int, Default: 3},
			{Name: "output", Short: "o", Type: String, Default: ""},
		},
	}

	tests := []struct {
		name    string
		argv    []string
		want    map[string]any
		wantErr bool
	}{
		{
			name: "single bool",
			argv: []string{"-a"},
			want: map[string]any{"all": true, "brief": false, "priority": 3},
		},
		{
			name: "grouped bools",
			argv: []string{"-abc"},
			want: map[string]any{"all": true, "brief": true, "color": true},
		},
		{
			name: "group ending in a valued flag, value in the next token",
			argv: []string{"-ap", "1"},
			want: map[string]any{"all": true, "priority": 1},
		},
		{
			name: "group ending in a valued flag, value glued to it",
			argv: []string{"-ap1"},
			want: map[string]any{"all": true, "priority": 1},
		},
		{
			name: "valued flag with an equals sign",
			argv: []string{"-o=report.txt"},
			want: map[string]any{"output": "report.txt"},
		},
		{
			name: "negative value is not mistaken for a flag",
			argv: []string{"-p", "-2"},
			want: map[string]any{"priority": -2},
		},
		{name: "unknown short", argv: []string{"-z"}, wantErr: true},
		{name: "valued flag with nothing after it", argv: []string{"-p"}, wantErr: true},
		// "-pabc" is p taking "abc" as its glued value, not four flags: the
		// first non-bool ends the group and swallows the rest of the token.
		{name: "bad type in a group", argv: []string{"-pabc"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := Parse(root, tt.argv)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%v) = nil error, want error", tt.argv)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%v) returned %v", tt.argv, err)
			}
			for name, want := range tt.want {
				got := ctx.flags[name]
				if got != want {
					t.Errorf("flag %q = %v (%T), want %v (%T)", name, got, got, want, want)
				}
			}
		})
	}
}

// remoteTree is the tree the spec's acceptance test is written against, and it
// is deliberately shaped to exercise the hard cases in one structure:
//
//   - two levels of nesting (task remote add), so routing has to recurse
//   - a persistent flag on the root (-v) that must reach the deepest leaf
//   - a NON-persistent flag on the root (-c) that must NOT reach it
//   - two required positionals, so binding has something to split
//
// A tree that only exercised one of these would let the other three regress
// silently.
func remoteTree() *Command {
	return &Command{
		Name:  "task",
		Short: "local task manager",
		Flags: []Flag{
			{Name: "verbose", Short: "v", Type: Bool, Default: false,
				Usage: "verbose output", Persistent: true},
			{Name: "config", Short: "c", Type: String, Default: "",
				Usage: "config path, not inherited"},
		},
		Sub: []*Command{
			{
				Name:  "remote",
				Short: "manage remotes",
				Sub: []*Command{
					{
						Name:  "add",
						Short: "add a remote",
						Flags: []Flag{
							{Name: "force", Short: "f", Type: Bool, Default: false, Usage: "overwrite"},
						},
						Args: []Arg{
							{Name: "name", Arity: One, Usage: "remote name"},
							{Name: "url", Arity: One, Usage: "remote url"},
						},
						Run: func(ctx *Context) error { return nil },
					},
				},
			},
		},
	}
}

// TestParseRouting checks that a token matching a subcommand descends into it
// rather than being collected as a positional, and that the walk records where
// it ended up.
//
// Path matters as much as Cmd: an error message that says "add" without saying
// "task remote add" leaves the user with no idea where that command lives.
func TestParseRouting(t *testing.T) {
	root := remoteTree()

	ctx, err := Parse(root, []string{"remote", "add", "origin", "https://x"})
	if err != nil {
		t.Fatalf("Parse returned %v", err)
	}
	if got := ctx.PathString(); got != "task remote add" {
		t.Errorf("PathString() = %q, want \"task remote add\"", got)
	}
	if ctx.Cmd.Name != "add" {
		t.Errorf("Cmd.Name = %q, want \"add\"", ctx.Cmd.Name)
	}
}

// TestParsePersistentFlagIsInherited and its counterpart below are a pair, and
// neither means much alone.
//
// This one proves inheritance happens: -v is declared only on the root, typed
// two levels down, and still resolves.
func TestParsePersistentFlagIsInherited(t *testing.T) {
	ctx, err := Parse(remoteTree(), []string{"remote", "add", "-v", "origin", "https://x"})
	if err != nil {
		t.Fatalf("Parse returned %v", err)
	}
	if !ctx.Bool("verbose") {
		t.Error("verbose should be true: it is persistent on the root")
	}
}

// TestParseNonPersistFlagIsNotInherited proves inheritance is SELECTIVE, which
// is the half that actually constrains the implementation.
//
// A descend that merely added the child's flags to the existing index would
// pass the test above and fail this one: --config would stay visible forever.
// Rebuilding the index from persistent-plus-own is what makes it disappear.
func TestParseNonPersistFlagIsNotInherited(t *testing.T) {
	_, err := Parse(remoteTree(), []string{"remote", "add", "--config", "x"})
	if err == nil {
		t.Fatal("--config is declared on the root without Persistent, so it must not be visible on \"remote add\"")
	}
}

// TestParseTerminator pins the POSIX "--" convention: everything after it is a
// positional, even when it looks exactly like a flag.
//
// Without it there is no way to pass a value that starts with a dash — a task
// titled "--not-a-flag" would be unrepresentable. The two tokens here would
// otherwise be an unknown long flag and an unknown short flag.
func TestParseTerminator(t *testing.T) {
	ctx, err := Parse(remoteTree(), []string{"remote", "add", "--", "--not-a-flag", "-x"})
	if err != nil {
		t.Fatalf("Parse returned %v", err)
	}
	want := []string{"--not-a-flag", "-x"}
	if len(ctx.positionalsForTest()) != len(want) {
		t.Fatalf("positionals = %v, want %v", ctx.positionalsForTest(), want)
	}
}

// TestParseSubCommandAfterPositionalIsPositional pins the second — and last —
// place where token order carries meaning.
//
// Routing stops for good at the first positional. Without that rule, a task
// titled "remote" would silently route into the remote subcommand instead of
// being stored, and the user would have no way to express the title at all.
func TestParseSubCommandAfterPositionalIsPositional(t *testing.T) {
	ctx, err := Parse(remoteTree(), []string{"origin", "remote"})
	if err != nil {
		t.Fatalf("Parse returned %v", err)
	}
	if ctx.Cmd.Name != "task" {
		t.Errorf("Cmd.Name = %q, want \"task\": routing must not resume after a positional", ctx.Cmd.Name)
	}
}
