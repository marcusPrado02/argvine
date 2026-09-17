package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/marcusPrado02/argvine"
)

// root is package level so runCompletion can render the very tree it belongs
// to.
//
// EN — A handler receives a *Context, which knows the command it landed on but
// not the whole tree. Generating a completion script needs the root, so the one
// piece of state this program keeps is exactly that.
//
// Careful with the assignment in main: it must be `root = buildTree()`, not
// `root := buildTree()`. The short form declares a NEW local variable that
// shadows this one, leaves it nil, and the only symptom is a nil dereference
// inside runCompletion — the one path that reads the package-level root. It
// compiles, and `go vet` in its default set does not flag it.
//
// PT — Um handler recebe um *Context, que sabe em qual comando caiu mas não a
// árvore inteira. Gerar um script de completion precisa da raiz, então o único
// estado que este programa guarda é exatamente esse.
//
// Cuidado com a atribuição no main: tem que ser `root = buildTree()`, não
// `root := buildTree()`. A forma curta declara uma variável local NOVA que
// sombreia esta, deixa ela nil, e o único sintoma é um desreferenciamento de nil
// dentro do runCompletion — o único caminho que lê o root de pacote. Compila, e
// o `go vet` no conjunto padrão não sinaliza.
var root *argvine.Command

// main is the only place in this repository that prints or decides an exit
// code.
//
// EN — Step by step:
//
//  1. Build the tree and Validate it. A malformed tree is a bug in THIS
//     program, never user input, so it panics rather than returning an error
//     a user could not act on.
//  2. Parse. On error, separate the help sentinel from a real failure — the
//     first exits 0, the second exits 1.
//  3. A node with children and no Run is a namespace: print its help and exit
//     1, because here the help IS the error message ("you did not say what to
//     do"), not an answer to what was asked.
//  4. Run the handler and report whatever it returns.
//
// Two exit codes, not three. 0 covers success INCLUDING --help; 1 covers every
// failure. Conventions for finer codes exist (sysexits.h: 64 for usage, 74 for
// I/O) but almost nothing consumes them, and committing to them means
// classifying every future error.
//
// PT — Passo a passo:
//
//  1. Monta a árvore e a Valida. Árvore malformada é bug DESTE programa, nunca
//     entrada do usuário, então panica em vez de devolver um erro sobre o qual
//     o usuário não teria o que fazer.
//  2. Parseia. No erro, separa o sentinela de help de uma falha de verdade — o
//     primeiro sai com 0, a segunda com 1.
//  3. Um nó com filhos e sem Run é namespace: imprime o help dele e sai com 1,
//     porque aqui o help É a mensagem de erro ("você não disse o que fazer"),
//     não resposta ao que foi pedido.
//  4. Roda o handler e reporta o que ele devolver.
//
// Dois códigos de saída, não três. 0 cobre sucesso INCLUINDO --help; 1 cobre
// toda falha. Convenções para códigos mais finos existem (sysexits.h: 64 para
// uso, 74 para I/O) mas quase nada as consome, e assumi-las significa
// classificar todo erro futuro.
func main() {
	// EN: Plain "=", not ":=". See the note on the root declaration above.
	// PT: "=" simples, não ":=". Ver a nota na declaração do root acima.
	root = buildTree()

	// EN: Validate before Parse, always. It is the only thing standing between
	// a typo in the tree literal and a confusing runtime panic much later.
	// PT: Validate antes do Parse, sempre. É a única coisa entre um engano no
	// literal da árvore e um panic confuso muito mais tarde.
	if err := root.Validate(); err != nil {
		panic(err)
	}

	// EN: os.Args[1:] drops the program name — the tree's root already carries
	// it, and passing it in would make the parser try to route on "task".
	// PT: os.Args[1:] descarta o nome do programa — a raiz da árvore já o
	// carrega, e passá-lo faria o parser tentar rotear em "task".
	ctx, err := argvine.Parse(root, os.Args[1:])
	if err != nil {
		// EN: The sentinel check comes first and exits 0. Someone who asked for
		// help got help; a script checking $? must not read that as a failure.
		// PT: A checagem do sentinela vem primeiro e sai com 0. Quem pediu ajuda
		// recebeu ajuda; um script conferindo $? não pode ler isso como falha.
		var help *argvine.ErrHelpRequested
		if errors.As(err, &help) {
			fmt.Print(help.Help())
			os.Exit(0)
		}

		// EN: Errors go to stderr, prefixed with the program name — the Unix
		// convention that lets a user piping stdout still see what went wrong.
		// PT: Erros vão para stderr, prefixados com o nome do programa — a
		// convenção Unix que deixa quem redireciona stdout ainda ver o problema.
		fmt.Fprintln(os.Stderr, "task:", err)
		os.Exit(1)
	}

	// EN: "task" alone, or "task remote" in a deeper tree, lands here. Exit 1,
	// not 0: the user has not asked for anything yet.
	// PT: "task" sozinho, ou "task remote" numa árvore mais funda, cai aqui.
	// Sai com 1, não 0: o usuário ainda não pediu nada.
	if ctx.Cmd.Run == nil {
		fmt.Print(ctx.Help())
		os.Exit(1)
	}

	if err := ctx.Cmd.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "task:", err)
		os.Exit(1)
	}
}

// buildTree declares the whole CLI as one composite literal.
//
// EN — This function IS the CLI. Read it top to bottom and you know every
// command, every flag and every argument the program accepts — there is no
// registration elsewhere, no init(), no second phase wiring parents to
// children. Help and completion are generated from exactly this.
//
// Each subcommand exercises a different capability of the framework on purpose,
// because cmd/task exists to prove the ergonomics, not just to be useful:
//
//	add         required positional, int flag with a short form, string flag
//	list        string flag with Choices, so an invalid value is rejected
//	done        Many arity, so it takes one or more ids
//	completion  a leaf whose whole job is to render the tree it lives in
//
// --verbose is Persistent on the root, so it works after any subcommand:
// "task -v list" and "task list -v" are the same thing.
//
// PT — Esta função É a CLI. Leia de cima a baixo e você sabe todo comando, toda
// flag e todo argumento que o programa aceita — não há registro em outro lugar,
// não há init(), não há segunda fase costurando pais a filhos. Help e completion
// são gerados exatamente disto.
//
// Cada subcomando exercita uma capacidade diferente do framework de propósito,
// porque o cmd/task existe para provar a ergonomia, não só para ser útil:
//
//	add         posicional obrigatório, flag int com forma curta, flag string
//	list        flag string com Choices, então valor inválido é recusado
//	done        aridade Many, então aceita um ou mais ids
//	completion  uma folha cujo trabalho inteiro é renderizar a árvore onde mora
//
// --verbose é Persistent na raiz, então funciona depois de qualquer subcomando:
// "task -v list" e "task list -v" são a mesma coisa.
func buildTree() *argvine.Command {
	return &argvine.Command{
		Name:  "task",
		Short: "local task manager",
		Long:  "task keeps a list of things to do in a local JSON file.",
		Flags: []argvine.Flag{
			{Name: "verbose", Short: "v", Type: argvine.Bool, Default: false,
				Usage: "print extra detail", Persistent: true},
		},
		Sub: []*argvine.Command{
			{
				Name:  "add",
				Short: "create a task",
				Long:  "Create a task. The title is taken verbatim; use -- before a title that starts with a dash.",
				Flags: []argvine.Flag{
					{Name: "priority", Short: "p", Type: argvine.Int, Default: 3,
						Usage: "priority from 1 (high) to 5 (low)"},
					{Name: "due", Short: "d", Type: argvine.String, Default: "",
						Usage: "due date as YYYY-MM-DD"},
				},
				Args: []argvine.Arg{
					{Name: "title", Arity: argvine.One, Usage: "what to do"},
				},
				Run: runAdd,
			},
			{
				Name:  "list",
				Short: "list tasks",
				Flags: []argvine.Flag{
					{Name: "status", Short: "s", Type: argvine.String, Default: "open",
						Choices: []string{"open", "done", "all"}, Usage: "which tasks to show"},
				},
				Run: runList,
			},
			{
				Name:  "done",
				Short: "close one or more tasks",
				Args: []argvine.Arg{
					{Name: "ids", Arity: argvine.Many, Usage: "task ids to close"},
				},
				Run: runDone,
			},
			{
				Name:  "completion",
				Short: "print a shell completion script",
				Args: []argvine.Arg{
					{Name: "shell", Arity: argvine.One, Usage: "target shell (bash)"},
				},
				Run: runCompletion,
			},
		},
	}
}

// runAdd creates a task.
//
// EN — Look at the shape rather than the logic: three values come out of the
// Context, and everything after that is plain Go. The Context DIES HERE — it is
// never passed to Store, which is what keeps the domain testable without the
// framework.
//
// PT — Olhe a forma em vez da lógica: três valores saem do Context, e tudo
// depois disso é Go comum. O Context MORRE AQUI — nunca é passado ao Store, e é
// isso que mantém o domínio testável sem o framework.
func runAdd(ctx *argvine.Context) error {
	store, err := Load()
	if err != nil {
		return err
	}

	// EN: A range check the framework cannot do for us. Choices restricts a
	// value to a SET; this is an interval, so it belongs to the application.
	// Worth noticing where the framework stops being enough.
	// PT: Uma checagem de intervalo que o framework não faz por nós. Choices
	// restringe um valor a um CONJUNTO; isto é um intervalo, então pertence à
	// aplicação. Vale notar onde o framework deixa de bastar.
	priority := ctx.Int("priority")
	if priority < 1 || priority > 5 {
		return fmt.Errorf("priority must be between 1 and 5, got %d", priority)
	}

	t := store.Add(ctx.Arg("title"), priority, ctx.String("due"))
	if err := store.Save(); err != nil {
		return err
	}

	// EN: --verbose is declared two levels up and read here without ceremony.
	// That is what Persistent buys, and this is the line that demonstrates it.
	// PT: --verbose é declarada dois níveis acima e lida aqui sem cerimônia. É
	// isso que o Persistent compra, e esta é a linha que demonstra.
	if ctx.Bool("verbose") {
		fmt.Printf("added #%d %q (priority %d, due %q)\n", t.ID, t.Title, t.Priority, t.Due)
		return nil
	}

	// EN: Quiet by default. A CLI that prints a paragraph on every success is
	// unusable in a pipeline; the id is the one thing the user needs next.
	// PT: Silencioso por padrão. Uma CLI que imprime um parágrafo a cada sucesso
	// é inútil num pipeline; o id é a única coisa que o usuário precisa em
	// seguida.
	fmt.Printf("added #%d\n", t.ID)
	return nil
}

// runList prints the tasks matching --status.
//
// EN — Note what is NOT here: no validation of the status value. Choices on the
// flag already rejected anything outside {open, done, all} before this ran,
// with a message listing the accepted words. The handler receives a value it
// can trust, which is the practical payoff of declaring constraints in the tree
// instead of checking them in every handler.
//
// PT — Note o que NÃO está aqui: nenhuma validação do valor de status. O Choices
// da flag já recusou qualquer coisa fora de {open, done, all} antes disto rodar,
// com mensagem listando as palavras aceitas. O handler recebe um valor em que
// pode confiar, que é o retorno prático de declarar restrições na árvore em vez
// de checá-las em todo handler.
func runList(ctx *argvine.Context) error {
	store, err := Load()
	if err != nil {
		return err
	}

	tasks := store.List(ctx.String("status"))

	// EN: An empty list is not an error and not silence. Printing nothing would
	// leave the user unsure whether the command ran.
	// PT: Lista vazia não é erro nem silêncio. Não imprimir nada deixaria o
	// usuário em dúvida se o comando rodou.
	if len(tasks) == 0 {
		fmt.Println("no tasks")
		return nil
	}

	for _, t := range tasks {
		mark := " "
		if t.Done {
			mark = "x"
		}

		// EN: %-3d left-aligns the id in a three-wide column, so the titles line
		// up whether the id is 1 or 100 — the same measure-then-draw concern as
		// the help renderer, solved here by a fixed width because ids are small.
		// PT: %-3d alinha o id à esquerda numa coluna de três, então os títulos
		// ficam alinhados com id 1 ou 100 — a mesma preocupação de medir-depois-
		// desenhar do renderizador de help, resolvida aqui com largura fixa
		// porque ids são pequenos.
		fmt.Printf("[%s] #%-3d p%d  %s", mark, t.ID, t.Priority, t.Title)
		if t.Due != "" {
			fmt.Printf("  (due %s)", t.Due)
		}
		if ctx.Bool("verbose") {
			fmt.Printf("  created %s", t.Created.Format("2006-01-02 15:04"))
		}
		fmt.Println()
	}
	return nil
}

// runDone closes one or more tasks.
//
// EN — ArgList, not Arg: "ids" is declared with Many arity, so it can hold
// several values and Arg would silently return only the first.
//
// The conversion loop is here and not in the framework on purpose. The v1 flag
// types are bool, string and int, and positionals are always strings — adding
// an int arity would mean answering "what does an invalid one look like in the
// generated help", which is a design question the milestone deliberately did
// not open. Fifteen lines in the handler is the honest price of that decision.
//
// PT — ArgList, não Arg: "ids" é declarado com aridade Many, então pode ter
// vários valores e o Arg devolveria só o primeiro em silêncio.
//
// O laço de conversão está aqui e não no framework de propósito. Os tipos de
// flag da v1 são bool, string e int, e posicionais são sempre strings —
// acrescentar aridade tipada significaria responder "como um valor inválido
// aparece no help gerado", que é uma pergunta de design que o marco
// deliberadamente não abriu. Quinze linhas no handler é o preço honesto dessa
// decisão.
func runDone(ctx *argvine.Context) error {
	store, err := Load()
	if err != nil {
		return err
	}

	raw := ctx.ArgList("ids")
	ids := make([]int, 0, len(raw))
	for _, s := range raw {
		n, err := strconv.Atoi(s)
		if err != nil {
			// EN: The strconv error is dropped and replaced. "parsing \"x\":
			// invalid syntax" is about strconv; "invalid task id \"x\"" is about
			// what the user typed. Same rule the framework follows in convert.
			// PT: O erro do strconv é descartado e substituído. "parsing \"x\":
			// invalid syntax" fala de strconv; "invalid task id \"x\"" fala do
			// que o usuário digitou. Mesma regra que o framework segue no convert.
			return fmt.Errorf("invalid task id %q", s)
		}
		ids = append(ids, n)
	}

	n, err := store.Close(ids)
	if err != nil {
		return err
	}
	if err := store.Save(); err != nil {
		return err
	}
	fmt.Printf("closed %d task(s)\n", n)
	return nil
}

// runCompletion prints a shell completion script for this program.
//
// EN — The only handler that reads the package-level root, and therefore the
// only one that breaks if main shadows it with ":=". Worth knowing when
// something here suddenly dereferences nil.
//
// The script goes to stdout so it can be sourced directly:
//
//	source <(task completion bash)
//
// A future improvement would be declaring Choices: []string{"bash"} on the
// shell argument — Arg already carries the field, nothing reads it yet. That
// would move this default branch into the framework and make the accepted
// value show up in the generated help for free.
//
// PT — O único handler que lê o root de pacote, e portanto o único que quebra
// se o main o sombrear com ":=". Vale saber quando algo aqui de repente
// desreferencia nil.
//
// O script vai para stdout para poder receber source direto:
//
//	source <(task completion bash)
//
// Uma melhoria futura seria declarar Choices: []string{"bash"} no argumento
// shell — o Arg já carrega o campo, ninguém o lê ainda. Isso moveria este ramo
// default para dentro do framework e faria o valor aceito aparecer no help
// gerado de graça.
func runCompletion(ctx *argvine.Context) error {
	switch shell := ctx.Arg("shell"); shell {
	case "bash":
		fmt.Print(argvine.Bash(root))
		return nil
	default:
		return fmt.Errorf("unsupported shell %q: only bash is supported", shell)
	}
}
