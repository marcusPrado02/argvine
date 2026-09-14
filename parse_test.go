package argvine

import (
	"strings"
	"testing"
)

// TestConvert covers the one place a raw command-line token becomes a typed
// value. Everything downstream trusts it, so the edge cases live here rather
// than being retested through Parse.
//
// EN — "negative int parses" is the case worth staring at: -3 is a perfectly
// good value, which is why the parser must never decide "starts with a dash, so
// it cannot be a value".
//
// PT — "negative int parses" é o caso que merece atenção: -3 é um valor
// perfeitamente bom, e é por isso que o parser nunca pode decidir "começa com
// hífen, logo não pode ser valor".
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
			got, ok := convert(tt.flag, tt.raw)
			if tt.wantErr {
				if ok {
					t.Fatalf("convert(%v, %q) = %v, want failure", tt.flag.Type, tt.raw, got)
				}
				return
			}
			if !ok {
				t.Fatalf("convert(%v, %q) failed", tt.flag.Type, tt.raw)
			}
			if got != tt.want {
				t.Errorf("convert(%v, %q) = %v (%T), want %v (%T)", tt.flag.Type, tt.raw, got, got, tt.want, tt.want)
			}
		})
	}

}

// TestFlagIndex checks the lookup structure the parser consults for every token.
//
// EN — The third assertion is the one that earns its place: a flag with no
// short form must not register an empty key, or the first such flag would claim
// "" and every later one would silently overwrite it.
//
// PT — A terceira asserção é a que justifica seu lugar: uma flag sem forma curta
// não pode registrar chave vazia, senão a primeira delas reivindicaria "" e todas
// as seguintes a sobrescreveriam em silêncio.
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
// EN — One tree, many argv lines: this is the shape the whole design was chosen
// to allow. It only works because Parse writes nothing back into root, so the
// nine cases below share a tree without contaminating each other. Had flags
// been parsed into pre-allocated pointers, each case would need its own tree.
//
// Every case asserts all three flags, not just the one it exercises, so a
// change that leaks a value across flags cannot hide.
//
// PT — Uma árvore, vários argv: esta é a forma que o design inteiro foi
// escolhido para permitir. Só funciona porque o Parse não escreve nada de volta
// em root, então os nove casos abaixo compartilham uma árvore sem se contaminar.
// Se as flags fossem parseadas em ponteiros pré-alocados, cada caso precisaria
// da própria árvore.
//
// Todo caso afirma as três flags, não só a que exercita, para que uma mudança
// que vaze valor entre flags não consiga se esconder.
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
				// EN: The error case ends here — there is no ctx to inspect,
				// and falling through would hit the success assertions below.
				// PT: O caso de erro termina aqui — não há ctx para inspecionar,
				// e cair adiante bateria nas asserções de sucesso abaixo.
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

// TestParseShortFlags covers the grouping rules, which are the fiddliest part of
// the parser and the easiest to get subtly wrong.
//
// EN — The two cases that matter most are the last kind: "-p -2" must read -2
// as a value, and "-pabc" must read "abc" as a value that then fails
// conversion. Both fall out of the same rule — the first non-bool ends the
// group — and a parser that instead looked at whether a token starts with a
// dash would get one of them wrong no matter which way it decided.
//
// PT — Os dois casos que mais importam são do último tipo: "-p -2" tem que ler
// -2 como valor, e "-pabc" tem que ler "abc" como valor que depois falha na
// conversão. Os dois caem da mesma regra — a primeira não-booleana encerra o
// grupo — e um parser que em vez disso olhasse se o token começa com hífen
// erraria um dos dois, decidisse o que decidisse.
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
		// EN: "-pabc" is p taking "abc" as its glued value, not four flags — the
		// first non-bool ends the group and swallows the rest of the token.
		// PT: "-pabc" é p pegando "abc" como valor colado, não quatro flags — a
		// primeira não-booleana encerra o grupo e engole o resto do token.
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

// remoteTree is the tree the spec's acceptance test is written against.
//
// EN — It is deliberately shaped to exercise the hard cases in one structure:
//
//   - two levels of nesting (task remote add), so routing has to recurse
//   - a persistent flag on the root (-v) that must reach the deepest leaf
//   - a NON-persistent flag on the root (-c) that must NOT reach it
//   - two required positionals, so binding has something to split
//
// A tree that only exercised one of these would let the other three regress
// silently.
//
// PT — Tem forma deliberada para exercitar os casos difíceis numa estrutura só:
//
//   - dois níveis de aninhamento (task remote add), para o roteamento recursar
//   - uma flag persistente na raiz (-v) que precisa alcançar a folha mais funda
//   - uma flag NÃO-persistente na raiz (-c) que NÃO pode alcançá-la
//   - dois posicionais obrigatórios, para a ligação ter o que dividir
//
// Uma árvore que exercitasse só um desses deixaria os outros três regredirem em
// silêncio.
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
// EN — Path matters as much as Cmd: an error message that says "add" without
// saying "task remote add" leaves the user with no idea where that command
// lives.
//
// PT — Path importa tanto quanto Cmd: uma mensagem de erro que diz "add" sem
// dizer "task remote add" deixa o usuário sem ideia de onde esse comando mora.
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
// EN — This one proves inheritance happens: -v is declared only on the root,
// typed two levels down, and still resolves.
//
// PT — Este prova que a herança acontece: -v é declarado só na raiz, digitado
// dois níveis abaixo, e ainda assim resolve.
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
// EN — A descend that merely added the child's flags to the existing index
// would pass the test above and fail this one: --config would stay visible
// forever. Rebuilding the index from persistent-plus-own is what makes it
// disappear.
//
// PT — Um descend que apenas somasse as flags do filho ao índice existente
// passaria no teste acima e falharia neste: --config ficaria visível para
// sempre. Reconstruir o índice a partir de persistentes-mais-próprias é o que o
// faz sumir.
func TestParseNonPersistFlagIsNotInherited(t *testing.T) {
	_, err := Parse(remoteTree(), []string{"remote", "add", "--config", "x"})
	if err == nil {
		t.Fatal("--config is declared on the root without Persistent, so it must not be visible on \"remote add\"")
	}
}

// TestParseTerminator pins the POSIX "--" convention: everything after it is a
// positional, even when it looks exactly like a flag.
//
// EN — Without it there is no way to pass a value that starts with a dash — a
// task titled "--not-a-flag" would be unrepresentable. The two tokens here
// would otherwise be an unknown long flag and an unknown short flag.
//
// PT — Sem ele não há como passar um valor que comece com hífen — uma tarefa
// chamada "--not-a-flag" seria impossível de representar. Os dois tokens aqui
// seriam, de outro modo, uma flag longa desconhecida e uma curta desconhecida.
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
// EN — Routing stops for good at the first positional. Without that rule, a
// task titled "remote" would silently route into the remote subcommand instead
// of being stored, and the user would have no way to express the title at all.
//
// PT — O roteamento para de vez no primeiro posicional. Sem essa regra, uma
// tarefa chamada "remote" rotearia calada para o subcomando remote em vez de ser
// guardada, e o usuário não teria como expressar esse título de jeito nenhum.
func TestParseSubCommandAfterPositionalIsPositional(t *testing.T) {
	ctx, err := Parse(remoteTree(), []string{"origin", "remote"})
	if err != nil {
		t.Fatalf("Parse returned %v", err)
	}
	if ctx.Cmd.Name != "task" {
		t.Errorf("Cmd.Name = %q, want \"task\": routing must not resume after a positional", ctx.Cmd.Name)
	}
}

// TestAcceptance1 is the spec's starred acceptance test, and the gate for this
// milestone.
//
// EN — The three orders are the point. They are the same six pieces of
// information arranged three ways, and all three must produce an identical
// Context:
//
//	remote add --force -v origin https://x     flags first
//	remote add origin --force https://x -v     interleaved
//	remote add -vf origin https://x            flags grouped into one token
//
// That commutativity is not something the parser was told to do; it falls out
// of classifying each token independently of what came before. Only two pieces
// of state break the symmetry — the "--" terminator and "no positional seen
// yet" — and neither is exercised here.
//
// Validate is called first on purpose: an acceptance test written against a
// malformed tree would be testing the wrong thing, and would fail in a way that
// looks like a parser bug.
//
// PT — As três ordens são o ponto. São as mesmas seis informações arranjadas de
// três jeitos, e as três precisam produzir um Context idêntico:
//
//	remote add --force -v origin https://x     flags primeiro
//	remote add origin --force https://x -v     intercalado
//	remote add -vf origin https://x            flags agrupadas num token só
//
// Essa comutatividade não é algo que o parser foi mandado fazer; ela cai de
// classificar cada token independentemente do que veio antes. Só duas memórias
// quebram a simetria — o terminador "--" e "nenhum posicional visto ainda" — e
// nenhuma das duas é exercitada aqui.
//
// O Validate é chamado primeiro de propósito: um teste de aceite escrito contra
// árvore malformada estaria testando a coisa errada, e falharia de um jeito que
// parece bug do parser.
func TestAcceptance1(t *testing.T) {
	root := remoteTree()
	if err := root.Validate(); err != nil {
		t.Fatalf("the acceptance tree must be valid: %v", err)
	}

	orders := [][]string{
		{"remote", "add", "--force", "-v", "origin", "https://x"},
		{"remote", "add", "origin", "--force", "https://x", "-v"},
		{"remote", "add", "-vf", "origin", "https://x"},
	}

	for _, argv := range orders {
		t.Run(strings.Join(argv, " "), func(t *testing.T) {
			ctx, err := Parse(root, argv)
			if err != nil {
				t.Fatalf("Parse returned %v", err)
			}
			if got := ctx.PathString(); got != "task remote add" {
				t.Errorf("PathString() = %q, want \"task remote add\"", got)
			}
			if !ctx.Bool("force") {
				t.Error("force = false, want true")
			}
			if !ctx.Bool("verbose") {
				t.Error("verbose = false, want true (persistent, inherited from root)")
			}
			if got := ctx.Arg("name"); got != "origin" {
				t.Errorf("Arg(\"name\") = %q, want \"origin\"", got)
			}
			if got := ctx.Arg("url"); got != "https://x" {
				t.Errorf("Arg(\"url\") = %q, want \"https://x\"", got)
			}
		})
	}
}

// TestAcceptance1UnknownFlagMentionsTheCommand is the other half of the spec's
// first criterion: a rejected flag must say WHERE it was rejected.
//
// EN — Asserting on the message text is normally a bad idea — wording changes
// are not behavior changes — but the substring checked here is not wording. It
// is the full command path, and it can only appear if the error was built with
// the walk's Path rather than with the leaf's name. "unknown flag --nope in
// add" would pass a naive test and leave the user hunting for which "add".
//
// PT — Afirmar sobre o texto da mensagem normalmente é má ideia — mudança de
// redação não é mudança de comportamento — mas o trecho conferido aqui não é
// redação. É o caminho completo do comando, e ele só pode aparecer se o erro foi
// construído com o Path da travessia em vez do nome da folha. "unknown flag
// --nope in add" passaria num teste ingênuo e deixaria o usuário caçando qual
// "add".
func TestAcceptance1UnknownFlagMentionsTheCommand(t *testing.T) {
	_, err := Parse(remoteTree(), []string{"remote", "add", "--nope"})
	if err == nil {
		t.Fatal("Parse should reject --nope")
	}
	if !strings.Contains(err.Error(), "task remote add") {
		t.Errorf("error %q should name the full command path", err)
	}
}
