package argvine

import "testing"

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
