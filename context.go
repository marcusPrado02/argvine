package argvine

import (
	"fmt"
	"strings"
)

// Context is the result of a successful Parse.
//
// EN — A fresh Context is produced by every call to Parse, so two parses over
// the same tree never contaminate each other. That is the whole reason parsed
// values live here instead of being written back into the Flag declarations,
// and it is what makes the table-driven tests possible: one tree, many argv
// lines, no state carried between them.
//
// PT — Um Context novo é produzido a cada chamada de Parse, então dois parses
// sobre a mesma árvore nunca se contaminam. É essa a razão de os valores
// parseados morarem aqui em vez de serem escritos de volta nas declarações de
// Flag, e é o que torna os testes de tabela possíveis: uma árvore, vários argv,
// sem estado carregado entre eles.
type Context struct {
	// EN: Cmd is the command that won the routing — the deepest node reached.
	// PT: Cmd é o comando que ganhou o roteamento — o nó mais profundo alcançado.
	Cmd *Command

	// EN: Path is the chain from the root down to Cmd. Help and error messages
	// use it to say "task remote add" instead of a bare "add" the user cannot
	// locate in the tree.
	// PT: Path é a cadeia da raiz até Cmd. Help e mensagens de erro usam isso
	// para dizer "task remote add" em vez de um "add" solto que o usuário não
	// consegue localizar na árvore.
	Path []*Command

	// EN: flags maps a long flag name to its already-converted value.
	// Unexported so every read goes through the typed accessors, which are the
	// surface the CLI author is meant to use.
	// PT: flags mapeia o nome longo da flag para seu valor já convertido. Não
	// exportado para que toda leitura passe pelos acessores tipados, que são a
	// superfície que o autor da CLI deve usar.
	flags map[string]any

	// EN: args maps a positional Arg name to its values. A slice even for
	// single values, so Many needs no separate storage.
	// PT: args mapeia o nome de um Arg posicional para seus valores. Slice
	// mesmo para valor único, assim Many não precisa de armazenamento próprio.
	args map[string][]string

	// EN: rawPositional holds the positional tokens in the order they appeared,
	// before they are bound to the command's declared Args. Keeping the raw
	// list separate from args means binding can be rewritten — arity, error
	// reporting — without the parser knowing anything about it.
	// PT: rawPositional guarda os tokens posicionais na ordem em que
	// apareceram, antes de serem ligados aos Args declarados do comando. Manter
	// a lista crua separada de args permite reescrever a ligação — aridade,
	// relato de erro — sem que o parser saiba nada a respeito.
	rawPositional []string
}

// newContext returns an empty Context with both maps ready to write into.
//
// EN — Unexported on purpose: a Context is only ever produced by Parse. Handing
// callers a constructor would invite them to build one by hand and then expect
// it to behave like a parsed one — with no defaults seeded and no Path set,
// every accessor would panic.
//
// PT — Não exportado de propósito: um Context só é produzido pelo Parse.
// Entregar um construtor a quem usa a lib convidaria a montar um à mão e
// esperar que se comportasse como um parseado — sem defaults semeados e sem
// Path, todo acessor panicaria.
func newContext() *Context {
	return &Context{
		flags: make(map[string]any),
		args:  make(map[string][]string),
	}
}

// PathString renders the full invocation path, e.g. "task remote add".
//
// EN — Every diagnostic in the package goes through this rather than through
// Cmd.Name, so a message never names a subcommand without saying where it lives
// in the tree. "unknown flag --nope in add" leaves the user hunting; "in task
// remote add" does not.
//
// PT — Todo diagnóstico do pacote passa por aqui em vez de por Cmd.Name, para
// que uma mensagem nunca nomeie um subcomando sem dizer onde ele vive na
// árvore. "unknown flag --nope in add" deixa o usuário caçando; "in task remote
// add" não.
func (c *Context) PathString() string {
	names := make([]string, len(c.Path))
	for i, cmd := range c.Path {
		names[i] = cmd.Name
	}
	return strings.Join(names, " ")
}

// Bool returns the value of a bool flag.
//
// EN — It panics when no such flag is visible on the parsed command, and when
// the flag is not a bool. Both are programming errors in the CLI that uses
// argvine, never usage errors by whoever ran it: the name passed here is
// written by the same person who declared the flag, so a mismatch means the two
// drifted apart. Failing loudly on the developer's first run beats silently
// yielding a zero value in production.
//
// PT — Panica quando nenhuma flag com esse nome é visível no comando parseado,
// e quando a flag não é booleana. Os dois são erros de programação de quem usa
// a argvine, nunca erros de uso de quem rodou a CLI: o nome passado aqui é
// escrito pela mesma pessoa que declarou a flag, então divergência significa
// que os dois se descolaram. Falhar alto na primeira execução do desenvolvedor
// é melhor que devolver zero em silêncio em produção.
func (c *Context) Bool(name string) bool {
	v := c.lookupFlag(name)
	b, ok := v.(bool)
	if !ok {
		panic(fmt.Sprintf("argvine: flag --%s on %q is %T, not bool", name, c.PathString(), v))
	}
	return b
}

// Int returns the value of an int flag.
//
// EN — See Bool for the panic contract; it is identical.
// PT — Ver Bool para o contrato de panic; é idêntico.
func (c *Context) Int(name string) int {
	v := c.lookupFlag(name)
	n, ok := v.(int)
	if !ok {
		panic(fmt.Sprintf("argvine: flag --%s on %q is %T, not int", name, c.PathString(), v))
	}
	return n
}

// String returns the value of a string flag.
//
// EN — See Bool for the panic contract; it is identical.
// PT — Ver Bool para o contrato de panic; é idêntico.
func (c *Context) String(name string) string {
	v := c.lookupFlag(name)
	s, ok := v.(string)
	if !ok {
		panic(fmt.Sprintf("argvine: flag --%s on %q is %T, not string", name, c.PathString(), v))
	}
	return s
}

// Arg returns the first value of a positional argument.
//
// EN — An argument declared with ZeroOrOne arity and absent from the command
// line yields "". That is a legitimate outcome, not an error, which is why this
// does not panic on an empty slice. Only an undeclared name panics — the
// function distinguishes "empty" from "never declared", the same distinction
// Go's own two-value map read makes.
//
// PT — Um argumento declarado com aridade ZeroOrOne e ausente da linha de
// comando devolve "". Isso é resultado legítimo, não erro, e por isso a função
// não panica com slice vazia. Só nome não declarado panica — a função distingue
// "vazio" de "nunca declarado", a mesma distinção que a leitura de mapa com
// dois valores de retorno faz em Go.
func (c *Context) Arg(name string) string {
	vs := c.lookupArg(name)
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

// ArgList returns every value of a positional argument.
//
// EN — This is the accessor for an Arg declared with Many arity. On any other
// arity it yields at most one element.
//
// PT — Este é o acessor para um Arg declarado com aridade Many. Em qualquer
// outra aridade devolve no máximo um elemento.
func (c *Context) ArgList(name string) []string {
	return c.lookupArg(name)
}

// lookupFlag is the single place a missing flag turns into a panic.
//
// EN — Centralizing it keeps the three typed accessors short and makes it
// impossible for them to disagree about the message. Three copies of the same
// panic is where the fourth one gets it wrong.
//
// PT — Centralizar mantém os três acessores tipados curtos e torna impossível
// que discordem sobre a mensagem. Três cópias do mesmo panic é onde a quarta
// sai errada.
func (c *Context) lookupFlag(name string) any {
	v, ok := c.flags[name]
	if !ok {
		panic(fmt.Sprintf("argvine: no flag --%s visible on %q", name, c.PathString()))
	}
	return v
}

// lookupArg is the equivalent of lookupFlag for positional arguments.
//
// EN — The angle brackets match how the argument is rendered in the generated
// usage line, so a panic message and the help output name the same thing the
// same way.
//
// PT — Os sinais de menor e maior combinam com a forma como o argumento é
// renderizado na linha de uso gerada, para que a mensagem de panic e a saída do
// help nomeiem a mesma coisa do mesmo jeito.
func (c *Context) lookupArg(name string) []string {
	v, ok := c.args[name]
	if !ok {
		panic(fmt.Sprintf("argvine: no argument <%s> declared on %q", name, c.PathString()))
	}
	return v
}

// positionalsForTest exposes the raw positional tokens to the package's own
// tests.
//
// EN — Not part of the public API, and the name says so on purpose: nothing
// outside the package can reach it, and nobody reading it inside the package
// mistakes it for something a CLI author should call.
//
// PT — Não faz parte da API pública, e o nome diz isso de propósito: nada fora
// do pacote alcança, e ninguém lendo dentro do pacote confunde com algo que um
// autor de CLI deveria chamar.
func (c *Context) positionalsForTest() []string {
	return c.rawPositional
}

// Help renders the help text of the parsed command.
//
// EN — Handlers use it to print usage on their own terms — a node that is a
// namespace with no Run, for instance, prints this and exits. The package
// itself still never prints: this returns the string and the caller decides
// where it goes and what exit code follows.
//
// It is the success-path twin of usageError.Help(): one for when parsing
// worked, one for when it did not, both rendering from the same Path.
//
// PT — Handlers usam isto para imprimir o uso nos próprios termos — um nó que é
// namespace sem Run, por exemplo, imprime isto e sai. O pacote em si continua
// nunca imprimindo: a função devolve a string e quem chama decide para onde ela
// vai e qual código de saída vem depois.
//
// É o gêmeo do usageError.Help() no caminho de sucesso: um para quando o parsing
// deu certo, outro para quando não deu, os dois renderizando a partir do mesmo
// Path.
func (c *Context) Help() string {
	return Help(c.Path)
}
