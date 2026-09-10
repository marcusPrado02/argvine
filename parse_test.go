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



func testParseShortFlags(t *testing.T) {
	root  := &Command {
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
		name string
		argv []string
		want map[string]any
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
		{name: "bad type in a group", argv: []string{"-pabc"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T){
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