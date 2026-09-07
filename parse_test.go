package argvine

import "testing"


func TestConvert(t *testing.T ) {
	tests := []struct {
		name string 
		flag Flag
		raw string
		want any 
		wantErr bool
	} {
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



func TestFlagIndex(t * testing.T) {
	idx := newFlagIndex()
	idx.add([]Flag {
		{Name: "verbose", Short:"v", Type: Bool},
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