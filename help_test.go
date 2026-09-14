package argvine

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helpTree is the fixture the whole help milestone is rendered against.
//
// EN — Every node exists to exercise a different rendering branch, and none is
// decoration:
//
//	root     Long + Short + a persistent flag, and children → "<command>"
//	add      two flags and a required arg    → "[flags] <title>"
//	list     flags only, with Choices        → "[flags]"
//	done     no flags, Many arity            → "<ids>..."
//	remote   a node with children but no flags of its own
//	remote/add  optional arg                 → "[url]"
//
// It is deliberately richer than remoteTree in parse_test.go: parsing cares
// about token shapes, rendering cares about declaration shapes, so the two
// milestones need different fixtures rather than one stretched to cover both.
//
// PT — Cada nó existe para exercitar um ramo de renderização diferente, e
// nenhum é decoração:
//
//	root     Long + Short + uma flag persistente, e filhos → "<command>"
//	add      duas flags e um arg obrigatório  → "[flags] <title>"
//	list     só flags, com Choices            → "[flags]"
//	done     sem flags, aridade Many          → "<ids>..."
//	remote   um nó com filhos mas sem flags próprias
//	remote/add  arg opcional                  → "[url]"
//
// É deliberadamente mais rica que a remoteTree do parse_test.go: o parsing se
// importa com formas de token, a renderização com formas de declaração, então os
// dois marcos precisam de fixtures diferentes em vez de uma esticada para cobrir
// os dois.
func helpTree() *Command {
	return &Command{
		Name:  "task",
		Short: "local task manager",
		Long:  "task keeps a list to do in a local JSON file.",
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
						Usage: "priority from 1 (high) to 5 (low)"},
					{Name: "due", Type: String, Default: "", Usage: "due date, YYYY-MM-DD"},
				},
				Args: []Arg{{Name: "title", Arity: One, Usage: "task title"}},
			},
			{
				Name:  "list",
				Short: "list tasks",
				Flags: []Flag{
					{Name: "status", Type: String, Default: "open",
						Choices: []string{"open", "done", "all"}, Usage: "filter by status"},
				},
			},
			{
				Name:  "done",
				Short: "close tasks",
				Args:  []Arg{{Name: "ids", Arity: Many, Usage: "task ids"}},
			},
			{
				Name:  "remote",
				Short: "manage remotes",
				Sub: []*Command{
					{
						Name:  "add",
						Short: "add a remote",
						Args: []Arg{
							{Name: "name", Arity: One},
							{Name: "url", Arity: ZeroOrOne},
						},
					},
				},
			},
		},
	}
}

// TestUsageLine is a table of tree shape → rendered line.
//
// EN — Six cases, six different combinations of the four segments the line can
// have. Read as a group they double as the syntax's documentation:
//
//	task <command> [flags]                   a node with children
//	task add [flags] <title>                 a leaf with flags and a required arg
//	task list [flags]                        a leaf with flags only
//	task done [flags] <ids>...               Many renders as an ellipsis
//	task remote <command> [flags]            nesting keeps the full prefix
//	task remote add [flags] <name> [url]     required then optional
//
// Two of them are quietly load-bearing. "task done [flags]" shows "[flags]"
// even though done declares none of its own — it inherits -v, and a line that
// hid that would be lying. "task remote add ..." shows the full prefix, which
// only works because usageLine takes the path rather than the node.
//
// PT — Seis casos, seis combinações diferentes dos quatro segmentos que a linha
// pode ter. Lidos em conjunto, servem também de documentação da sintaxe:
//
//	task <command> [flags]                   um nó com filhos
//	task add [flags] <title>                 folha com flags e arg obrigatório
//	task list [flags]                        folha só com flags
//	task done [flags] <ids>...               Many vira reticências
//	task remote <command> [flags]            aninhar mantém o prefixo completo
//	task remote add [flags] <name> [url]     obrigatório e depois opcional
//
// Dois deles sustentam mais do que parece. "task done [flags]" mostra "[flags]"
// mesmo done não declarando nenhuma própria — ele herda -v, e uma linha que
// escondesse isso estaria mentindo. "task remote add ..." mostra o prefixo
// completo, o que só funciona porque usageLine recebe o caminho, não o nó.
func TestUsageLine(t *testing.T) {
	root := helpTree()
	add := root.findSub("add")
	list := root.findSub("list")
	done := root.findSub("done")
	remote := root.findSub("remote")
	remoteAdd := remote.findSub("add")

	tests := []struct {
		name string
		path []*Command
		want string
	}{
		{name: "root", path: []*Command{root}, want: "task <command> [flags]"},
		{name: "leaf with one arg", path: []*Command{root, add}, want: "task add [flags] <title>"},
		{name: "leaf with only flags", path: []*Command{root, list}, want: "task list [flags]"},
		{name: "many arity", path: []*Command{root, done}, want: "task done [flags] <ids>..."},
		{name: "nested node", path: []*Command{root, remote}, want: "task remote <command> [flags]"},
		{name: "nested leaf with optional arg", path: []*Command{root, remote, remoteAdd},
			want: "task remote add [flags] <name> [url]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usageLine(tt.path); got != tt.want {
				t.Errorf("usageLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSplitFlags checks the division that "Flags" and "Global Flags" are built
// from.
//
// EN — Three things are asserted, and each one would let a different bug
// through if it were missing:
//
//   - own has TWO flags → the node's own set is complete
//   - own is sorted [due priority], not declaration order [priority due] → the
//     output is stable, which golden files depend on
//   - inherited is exactly [verbose] → only PERSISTENT ancestor flags cross
//     over, and the node's own flags do not leak into the inherited list
//
// The root case at the end is the boundary: a path of length one has no
// ancestors at all, and the slicing path[:len(path)-1] has to yield an empty
// range rather than panic.
//
// PT — Três coisas são afirmadas, e cada uma deixaria passar um bug diferente se
// faltasse:
//
//   - own tem DUAS flags → o conjunto próprio do nó está completo
//   - own está ordenado [due priority], não na ordem de declaração
//     [priority due] → a saída é estável, do que os golden files dependem
//   - inherited é exatamente [verbose] → só as PERSISTENTES dos ancestrais
//     atravessam, e as próprias do nó não vazam para a lista de herdadas
//
// O caso da raiz no fim é a fronteira: um caminho de comprimento um não tem
// ancestral nenhum, e o fatiamento path[:len(path)-1] precisa render um intervalo
// vazio em vez de panicar.
func TestSplitFlags(t *testing.T) {
	root := helpTree()
	add := root.findSub("add")

	own, inherited := splitFlags([]*Command{root, add})

	if len(own) != 2 {
		// EN: Fatal, not Error — the assertions below index into own.
		// PT: Fatal, não Error — as asserções abaixo indexam own.
		t.Fatalf("own = %d flags, want 2 (due, priority)", len(own))
	}

	// EN: "add" declares priority first and due second. Seeing them the other
	// way round here is the proof that sorting happened.
	// PT: "add" declara priority primeiro e due depois. Vê-las na ordem
	// inversa aqui é a prova de que a ordenação aconteceu.
	if own[0].Name != "due" || own[1].Name != "priority" {
		t.Errorf("own = [%s %s], want sorted [due priority]", own[0].Name, own[1].Name)
	}

	// EN: The root declares only --verbose, and only because it is Persistent
	// does it reach "add" at all.
	// PT: A raiz declara só --verbose, e é só por ser Persistent que ela chega
	// ao "add".
	if len(inherited) != 1 || inherited[0].Name != "verbose" {
		t.Errorf("inherited = %v, want [verbose]", inherited)
	}

	// EN: The root has no ancestors, so inherited must be empty — and its own
	// --verbose must appear under own, not under inherited. A flag is never
	// "global" to the command that declares it.
	// PT: A raiz não tem ancestrais, então inherited tem que estar vazio — e o
	// --verbose dela tem que aparecer em own, não em inherited. Uma flag nunca é
	// "global" para o comando que a declara.
	ownRoot, inheritedRoot := splitFlags([]*Command{root})
	if len(ownRoot) != 1 || len(inheritedRoot) != 0 {
		t.Errorf("root: own = %d, inherited = %d; want 1 and 0", len(ownRoot), len(inheritedRoot))
	}
}

// updateGolden rewrites the files in testdata instead of comparing against
// them: go test ./... -update
//
// EN — The flag is what keeps golden files cheap to maintain. The discipline it
// requires is reading the resulting diff before committing it — an -update that
// is run reflexively turns a regression test into a rubber stamp.
//
// PT — A flag é o que mantém golden files baratos de manter. A disciplina que
// ela exige é ler o diff resultante antes de commitar — um -update rodado por
// reflexo transforma um teste de regressão em carimbo.
var updateGolden = flag.Bool("update", false, "rewrite golden files in testdata")

// TestHelpGolden compares rendered help against files checked into testdata.
//
// EN — Why golden files instead of strings.Contains assertions: Contains tests
// that something IS there, and help is about HOW it is there. Broken column
// alignment, unstable ordering, a blank line too many, "Global Flags" glued to
// the section above — none of that fails a Contains, and all of it is exactly
// what makes a help screen bad.
//
// The golden also buys regression coverage for free: touching renderFlags and
// accidentally breaking another command's padding shows up in the diff
// immediately.
//
// The files MUST be committed. Ignoring testdata would make this test fail on a
// fresh clone with "reading golden: no such file", and delete the protection
// entirely.
//
// PT — Por que golden file em vez de asserções com strings.Contains: o Contains
// testa que algo ESTÁ lá, e help é sobre COMO está lá. Alinhamento de coluna
// quebrado, ordenação instável, uma linha em branco a mais, "Global Flags"
// colada na seção acima — nada disso falha um Contains, e é tudo exatamente o
// que torna uma tela de help ruim.
//
// O golden também dá cobertura de regressão de graça: mexer no renderFlags e
// quebrar sem querer o padding de outro comando aparece no diff na hora.
//
// Os arquivos TÊM que ser versionados. Ignorar testdata faria este teste falhar
// num clone limpo com "reading golden: no such file", e apagaria a proteção.
func TestHelpGolden(t *testing.T) {
	root := helpTree()

	tests := []struct {
		golden string
		path   []*Command
	}{
		// EN: Three shapes, three different section combinations:
		//   root   → Commands + Flags, no Global Flags (nothing above it)
		//   add    → Flags + Global Flags, no Commands (it is a leaf)
		//   remote → Commands + Global Flags, no Flags (declares none of its own)
		// Together they prove every section is skipped when empty rather than
		// printed as a bare heading.
		// PT: Três formas, três combinações de seção diferentes:
		//   root   → Commands + Flags, sem Global Flags (nada acima dele)
		//   add    → Flags + Global Flags, sem Commands (é folha)
		//   remote → Commands + Global Flags, sem Flags (não declara nenhuma)
		// Juntos provam que toda seção é pulada quando vazia em vez de impressa
		// como título sozinho.
		{golden: "help_root.txt", path: []*Command{root}},
		{golden: "help_add.txt", path: []*Command{root, root.findSub("add")}},
		{golden: "help_remote.txt", path: []*Command{root, root.findSub("remote")}},
	}

	for _, tt := range tests {
		t.Run(tt.golden, func(t *testing.T) {
			got := Help(tt.path)
			file := filepath.Join("testdata", tt.golden)

			if *updateGolden {
				if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
					t.Fatalf("writing golden: %v", err)
				}
				return
			}

			want, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("reading golden (run: go test ./... -update): %v", err)
			}
			// EN: Both sides are printed whole. A "help did not match" message
			// would send you to the file; printing got and want side by side
			// makes the difference visible in the test output itself.
			// PT: Os dois lados são impressos inteiros. Uma mensagem "help não
			// bateu" mandaria você ao arquivo; imprimir got e want lado a lado
			// deixa a diferença visível na própria saída do teste.
			if got != string(want) {
				t.Errorf("Help() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

// TestHelpIsStable renders the same tree many times and demands byte-identical
// output.
//
// EN — This exists because of one specific Go behaviour: map iteration order is
// randomised on purpose, and it varies between runs of the same binary. Any
// rendering path that iterated a map instead of a sorted slice would produce
// help that reshuffles itself — and would pass a single-run golden test roughly
// half the time.
//
// Twenty iterations is not a proof, it is a trap. A flaky test that fails once
// in twenty runs is worse than useless; this one fails almost immediately if
// non-determinism is ever introduced, which is what makes it worth having.
//
// PT — Isto existe por um comportamento específico de Go: a ordem de iteração de
// mapa é aleatorizada de propósito, e varia entre execuções do mesmo binário.
// Qualquer caminho de renderização que iterasse um mapa em vez de uma slice
// ordenada produziria help que se embaralha sozinho — e passaria num golden de
// execução única mais ou menos metade das vezes.
//
// Vinte iterações não é prova, é armadilha. Um teste instável que falha uma vez
// em vinte é pior que inútil; este falha quase de imediato se
// não-determinismo for introduzido, e é isso que o faz valer a pena.
func TestHelpIsStable(t *testing.T) {
	root := helpTree()
	for i := 0; i < 20; i++ {
		if Help([]*Command{root}) != Help([]*Command{root}) {
			t.Fatal("Help() is not deterministic")
		}
	}
}

// TestHelpHasNoHardcodedPerCommandText is the test that guards the design
// decision the whole milestone exists for.
//
// EN — It adds a flag to the tree at runtime — something no CLI author would
// ever do — and demands it appear in the help with nothing else edited. That is
// only possible if help is DERIVED from the tree rather than written alongside
// it.
//
// It is the executable form of the rule in the plan: if someone ever adds a
// HelpText field to Command, or a per-command template, this test breaks. And
// it should, because that field starts correct and rots on the first flag
// anyone adds afterwards.
//
// Note the mutation is safe here: helpTree() builds a fresh tree per call, so
// appending to add.Flags cannot leak into another test.
//
// PT — Ele acrescenta uma flag à árvore em tempo de execução — algo que nenhum
// autor de CLI faria — e exige que ela apareça no help sem mais nada editado.
// Isso só é possível se o help for DERIVADO da árvore em vez de escrito ao lado
// dela.
//
// É a forma executável da regra do plano: se alguém um dia acrescentar um campo
// HelpText ao Command, ou um template por comando, este teste quebra. E deve
// quebrar, porque esse campo nasce correto e apodrece na primeira flag que
// alguém adicionar depois.
//
// Note que a mutação é segura aqui: helpTree() constrói uma árvore nova a cada
// chamada, então dar append em add.Flags não vaza para outro teste.
func TestHelpHasNoHardcodedPerCommandText(t *testing.T) {
	root := helpTree()
	add := root.findSub("add")
	add.Flags = append(add.Flags, Flag{
		Name: "tag", Short: "t", Type: String, Default: "", Usage: "attach a tag",
	})

	out := Help([]*Command{root, add})

	// EN: Both the label and the description are checked. A renderer that
	// listed flag names but dropped Usage would pass on "--tag" alone.
	// PT: Tanto o rótulo quanto a descrição são conferidos. Um renderizador que
	// listasse nomes de flag mas perdesse o Usage passaria só com "--tag".
	if !strings.Contains(out, "--tag") || !strings.Contains(out, "attach a tag") {
		t.Errorf("a newly declared flag must appear in help automatically; got:\n%s", out)
	}
}

// TestHelpRequested checks that --help and -h stop the parse at the right node
// and render that node's help.
//
// EN — Three things are asserted per case, and each one guards a different
// mistake:
//
//   - errors.As finds *ErrHelpRequested → the sentinel travels through the
//     error channel and main can tell it apart from a real failure
//   - PathString is the node where --help appeared → not the root, not the leaf
//     the walk might have reached later
//   - Help() renders "Usage: <that path>" → the sentinel carries enough to
//     produce the right text, not just the fact that help was asked for
//
// "before a required arg is given" duplicates an earlier argv on purpose. The
// name is the assertion: "task add" declares <title> as required, and asking
// for help must NOT complain about it. If the interception ever moved out of
// the token loop and below bindArgs, only this case would catch it.
//
// PT — Três coisas são afirmadas por caso, e cada uma protege de um engano
// diferente:
//
//   - errors.As encontra *ErrHelpRequested → o sentinela viaja pelo canal de
//     erro e a main consegue distingui-lo de uma falha de verdade
//   - PathString é o nó onde o --help apareceu → não a raiz, nem a folha que a
//     varredura poderia ter alcançado depois
//   - Help() renderiza "Usage: <aquele caminho>" → o sentinela carrega o
//     suficiente para produzir o texto certo, não só o fato de terem pedido ajuda
//
// "before a required arg is given" duplica um argv anterior de propósito. O nome
// é a asserção: "task add" declara <title> como obrigatório, e pedir ajuda NÃO
// pode reclamar disso. Se a interceptação um dia saísse do laço de tokens para
// abaixo do bindArgs, só este caso pegaria.
func TestHelpRequested(t *testing.T) {
	root := helpTree()

	tests := []struct {
		name     string
		argv     []string
		wantPath string
	}{
		{name: "long form at root", argv: []string{"--help"}, wantPath: "task"},
		{name: "short form at root", argv: []string{"-h"}, wantPath: "task"},
		{name: "on a subcommand", argv: []string{"add", "--help"}, wantPath: "task add"},
		{name: "on a nested subcommand", argv: []string{"remote", "add", "--help"}, wantPath: "task remote add"},
		{name: "before a required arg is given", argv: []string{"add", "--help"}, wantPath: "task add"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(root, tt.argv)

			var help *ErrHelpRequested
			if !errors.As(err, &help) {
				t.Fatalf("Parse(%v) = %v (%T), want *ErrHelpRequested", tt.argv, err, err)
			}
			if got := help.PathString(); got != tt.wantPath {
				t.Errorf("PathString() = %q, want %q", got, tt.wantPath)
			}
			if !strings.Contains(help.Help(), "Usage: "+tt.wantPath) {
				t.Errorf("Help() should render the usage line for %q; got:\n%s", tt.wantPath, help.Help())
			}
		})
	}
}

// TestUsageErrorsExposeHelp proves the embedding pays off on the failure path
// too, not just for the help sentinel.
//
// EN — ErrUnknownFlag never declares a Help method. It gets one from the
// embedded usageError, and so does every other error in the package. That is
// what lets a main print the right usage without a switch over eight types:
// catch any error, ask it for Help(), print.
//
// The assertion is "Usage: task add", not the root's — an error must render the
// help of the command it happened on, or the user gets a page about the wrong
// thing.
//
// PT — ErrUnknownFlag nunca declara um método Help. Ele ganha um da usageError
// embutida, e todo outro erro do pacote também. É isso que permite a uma main
// imprimir o uso certo sem um switch sobre oito tipos: pegue qualquer erro, peça
// o Help() dele, imprima.
//
// A asserção é "Usage: task add", não a da raiz — um erro precisa renderizar o
// help do comando em que aconteceu, ou o usuário recebe uma página sobre outra
// coisa.
func TestUsageErrorsExposeHelp(t *testing.T) {
	_, err := Parse(helpTree(), []string{"add", "--nope"})

	var uf *ErrUnknownFlag
	if !errors.As(err, &uf) {
		t.Fatalf("got %T, want *ErrUnknownFlag", err)
	}
	if !strings.Contains(uf.Help(), "Usage: task add") {
		t.Errorf("a usage error must be able to render the help of its own command; got:\n%s", uf.Help())
	}
}

// TestDeclaredHelpFlagWins is the escape hatch: interception applies only to a
// --help the CLI did NOT declare.
//
// EN — The condition in longFlag reads "the lookup failed AND the name is
// help", and the order is the whole point. A framework that grabbed --help
// unconditionally would make it impossible to build "app --help networking",
// and the author would have no way to opt out.
//
// The flag here is deliberately a String, not a Bool: if interception were
// still winning, Parse would return ErrHelpRequested and "networking" would
// never be consumed as a value. Asserting on the VALUE, not just on err == nil,
// is what makes the case airtight.
//
// PT — A condição no longFlag é "a busca falhou E o nome é help", e a ordem é o
// ponto inteiro. Um framework que tomasse --help incondicionalmente tornaria
// impossível construir "app --help networking", e o autor não teria como sair
// disso.
//
// A flag aqui é String de propósito, não Bool: se a interceptação ainda
// estivesse ganhando, o Parse devolveria ErrHelpRequested e "networking" nunca
// seria consumido como valor. Afirmar sobre o VALOR, e não só sobre err == nil,
// é o que deixa o caso à prova.
func TestDeclaredHelpFlagWins(t *testing.T) {
	root := &Command{
		Name:  "app",
		Flags: []Flag{{Name: "help", Short: "h", Type: String, Default: "", Usage: "topic"}},
	}

	ctx, err := Parse(root, []string{"--help", "networking"})
	if err != nil {
		t.Fatalf("a declared --help must be parsed as a normal flag: %v", err)
	}
	if got := ctx.String("help"); got != "networking" {
		t.Errorf("String(\"help\") = %q, want \"networking\"", got)
	}
}
