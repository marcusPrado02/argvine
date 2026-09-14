package argvine

import (
	"fmt"
	"strings"
)

// EN — This file holds every error a *user* of a CLI can cause. Errors caused
// by the *author* of a CLI — a malformed tree — stay in command.go as plain
// fmt.Errorf values, because nothing needs to inspect or re-render them.
//
// The split is visible in the code and auditable with one command:
//
//	grep fmt.Errorf *.go   → only command.go should appear
//
// PT — Este arquivo guarda todo erro que um *usuário* de uma CLI pode causar.
// Erros causados pelo *autor* de uma CLI — árvore malformada — ficam em
// command.go como fmt.Errorf simples, porque ninguém precisa inspecionar nem
// re-renderizar esses.
//
// A separação é visível no código e auditável com um comando:
//
//	grep fmt.Errorf *.go   → só command.go deve aparecer

// usageError is embedded in every usage error below.
//
// EN — Nine error types need the same field and the same three methods. There
// were three ways to do that:
//
//	repeat the field nine times  → where the tenth one forgets a case
//	declare an interface         → describes the behaviour without providing it
//	embed a struct               → gives field AND methods, for free
//
// Embedding wins because it does both at once, and any interface these types
// need to satisfy later comes along without them knowing.
//
// It is unexported on purpose: implementation detail, not API. A CLI author
// sees *ErrUnknownFlag, can call PathString on it, and never has to learn where
// that method came from.
//
// PT — Nove tipos de erro precisam do mesmo campo e dos mesmos três métodos.
// Havia três jeitos:
//
//	repetir o campo nove vezes → onde o décimo esquece um caso
//	declarar uma interface     → descreve o comportamento sem fornecê-lo
//	embutir uma struct         → dá campo E métodos, de graça
//
// Embedding ganha porque faz as duas coisas de uma vez, e qualquer interface
// que esses tipos precisem satisfazer depois vem junto sem que eles saibam.
//
// É não exportada de propósito: detalhe de implementação, não API. Um autor de
// CLI vê *ErrUnknownFlag, pode chamar PathString nele, e nunca precisa
// descobrir de onde esse método veio.
type usageError struct {
	// EN: Path is the chain of commands the walk had reached when the error
	// happened, which is what lets a message say "task remote add" rather than
	// a bare "add" the user cannot locate.
	// PT: Path é a cadeia de comandos que a varredura tinha alcançado quando o
	// erro aconteceu, e é o que permite a mensagem dizer "task remote add" em
	// vez de um "add" solto que o usuário não consegue localizar.
	Path []*Command
}

// PathString renders the full invocation path, e.g. "task remote add".
//
// EN — Exported so a caller that branches on a specific error type can read the
// path without re-deriving it from the message text.
//
// PT — Exportado para que quem ramifica num tipo específico de erro consiga ler
// o caminho sem re-extraí-lo do texto da mensagem.
func (u usageError) PathString() string {
	names := make([]string, len(u.Path))
	for i, c := range u.Path {
		names[i] = c.Name
	}
	return strings.Join(names, " ")
}

// hint is the second half every usage message ends with.
//
// EN — Every error in this file has the same two-part shape, and the split is
// not cosmetic: the first line says what happened, the second says what to do
// next. An error that only diagnoses leaves the user stuck.
//
//	unknown flag --nope in "task remote add"
//
//	run "task remote add --help" for usage
//
// PT — Todo erro deste arquivo tem a mesma forma de duas partes, e a divisão
// não é enfeite: a primeira linha diz o que houve, a segunda diz o que fazer
// agora. Erro que só diagnostica deixa o usuário parado.
func (u usageError) hint() string {
	return fmt.Sprintf("\n\nrun %q for usage", u.PathString()+" --help")
}

// ErrUnknownFlag reports a flag that is not visible on the parsed command.
//
// EN — Flag is the token exactly as typed, dashes included, so the message can
// echo "--nope" or "-z" back without the parser having to remember which form
// the user wrote.
//
// PT — Flag é o token exatamente como digitado, hifens incluídos, para que a
// mensagem possa devolver "--nope" ou "-z" sem o parser ter que lembrar qual
// forma o usuário escreveu.
type ErrUnknownFlag struct {
	usageError
	Flag string
}

func (e *ErrUnknownFlag) Error() string {
	return fmt.Sprintf("unknown flag %s in %q%s", e.Flag, e.PathString(), e.hint())
}

// ErrMissingValue reports a non-bool flag that ended the command line with
// nothing after it.
//
// EN — It carries the whole Flag rather than just its name so the message can
// name the type that was expected: "needs a int value" tells the user what to
// type next, where "needs a value" only tells them something is missing.
//
// PT — Carrega a Flag inteira e não só o nome, para que a mensagem possa
// nomear o tipo esperado: "needs a int value" diz ao usuário o que digitar em
// seguida, enquanto "needs a value" só diz que falta algo.
type ErrMissingValue struct {
	usageError
	Flag Flag
}

func (e *ErrMissingValue) Error() string {
	return fmt.Sprintf("flag --%s needs a %s value%s", e.Flag.Name, e.Flag.Type, e.hint())
}

// ErrBadType reports a value that does not convert to the flag's declared type.
//
// EN — This is the error convert cannot build itself: convert knows the value
// is wrong but not where on the command tree it appeared, which is why it
// returns a bool and lets the caller — who has the path — construct this.
//
// PT — Este é o erro que o convert não consegue montar sozinho: ele sabe que o
// valor está errado mas não sabe em que ponto da árvore apareceu, e por isso
// devolve um bool e deixa quem chama — que tem o caminho — construir este.
type ErrBadType struct {
	usageError
	Flag Flag
	Got  string
}

func (e *ErrBadType) Error() string {
	return fmt.Sprintf("invalid value %q for --%s: expected %s%s", e.Got, e.Flag.Name, e.Flag.Type, e.hint())
}

// ErrMissingRequired reports a required flag that never appeared.
//
// EN — It can only be raised after the whole command line has been read: until
// the last token, "this flag was never given" is not yet a true statement.
//
// PT — Só pode ser levantado depois que a linha de comando inteira foi lida:
// até o último token, "esta flag nunca foi dada" ainda não é uma afirmação
// verdadeira.
type ErrMissingRequired struct {
	usageError
	Flag Flag
}

func (e *ErrMissingRequired) Error() string {
	return fmt.Sprintf("required flag --%s is missing%s", e.Flag.Name, e.hint())
}

// ErrBadChoice reports a value outside the flag's Choices set.
//
// EN — The message lists the accepted values rather than just rejecting: a user
// who typed "closed" instead of "done" should not have to go find --help to
// learn the three words that were allowed.
//
// PT — A mensagem lista os valores aceitos em vez de só recusar: quem digitou
// "closed" em vez de "done" não deveria precisar ir atrás do --help para
// descobrir as três palavras permitidas.
type ErrBadChoice struct {
	usageError
	Flag Flag
	Got  string
}

func (e *ErrBadChoice) Error() string {
	return fmt.Sprintf("invalid value %q for --%s: expected one of %s%s",
		e.Got, e.Flag.Name, strings.Join(e.Flag.Choices, ", "), e.hint())
}

// ErrMissingArg reports a positional argument whose arity was not satisfied.
//
// EN — Raised by One with nothing left, and by Many with nothing left. Not by
// ZeroOrOne, which treats absence as a legitimate outcome. The angle brackets
// match how the argument is rendered in the generated usage line, so the error
// and the help name the same thing the same way.
//
// PT — Levantado por One sem nada sobrando, e por Many sem nada sobrando. Não
// por ZeroOrOne, que trata ausência como resultado legítimo. Os sinais de menor
// e maior combinam com a renderização do argumento na linha de uso gerada, para
// que erro e help nomeiem a mesma coisa do mesmo jeito.
type ErrMissingArg struct {
	usageError
	Arg Arg
}

func (e *ErrMissingArg) Error() string {
	return fmt.Sprintf("missing required argument <%s>%s", e.Arg.Name, e.hint())
}

// ErrUnknownCommand reports a leftover token on a command that has subcommands,
// where the token was most likely meant to be one of them.
//
// EN — This and ErrUnexpectedArg describe the same raw situation — a positional
// nobody claimed — and are separate types because the user's next move differs.
// Here it is "I got the subcommand name wrong, show me the list"; in
// ErrUnexpectedArg it is "the command was right, I passed one argument too
// many". A single error would say "unexpected token" for both and leave the
// user to work out which problem they have.
//
// The test for whether two errors should be one: does the user do the same
// thing next? If yes, one error. If no, two.
//
// PT — Este e o ErrUnexpectedArg descrevem a mesma situação bruta — um
// posicional que ninguém reivindicou — e são tipos separados porque a ação
// seguinte do usuário difere. Aqui é "errei o nome do subcomando, me mostre a
// lista"; no ErrUnexpectedArg é "o comando estava certo, passei um argumento a
// mais". Um erro único diria "token inesperado" nos dois casos e deixaria o
// usuário descobrir qual dos dois problemas ele tem.
//
// O teste para saber se dois erros deveriam ser um só: o usuário faz a mesma
// coisa em seguida? Se sim, um erro. Se não, dois.
type ErrUnknownCommand struct {
	usageError
	Got string
}

func (e *ErrUnknownCommand) Error() string {
	return fmt.Sprintf("unknown command %q for %q%s", e.Got, e.PathString(), e.hint())
}

// ErrUnexpectedArg reports a leftover token on a command that declares no
// further positionals and has no subcommands.
//
// EN — The leaf counterpart of ErrUnknownCommand; see there for why the two are
// separate types.
//
// PT — A contraparte de folha do ErrUnknownCommand; ver lá por que os dois são
// tipos separados.
type ErrUnexpectedArg struct {
	usageError
	Got string
}

func (e *ErrUnexpectedArg) Error() string {
	return fmt.Sprintf("unexpected argument %q for %q%s", e.Got, e.PathString(), e.hint())
}

// Help renders the help text of the command where the error happened.
//
// EN — Every usage error inherits this through the embedded usageError, which
// is the payoff of that embedding: a caller holding any error from this package
// can print the right help without knowing which error it caught.
//
//	fmt.Fprintln(os.Stderr, err)
//	fmt.Fprint(os.Stderr, err.(interface{ Help() string }).Help())
//
// Note this is the one place errors.go reaches into help.go. It is not the
// parse/help/completion coupling the design forbids — those three still do not
// know about each other; this is the presentation layer asking the renderer for
// text, which is exactly what it is for.
//
// PT — Todo erro de uso herda isto pela usageError embutida, que é o retorno do
// embedding: quem chama, segurando qualquer erro deste pacote, consegue imprimir
// o help certo sem saber qual erro pegou.
//
//	fmt.Fprintln(os.Stderr, err)
//	fmt.Fprint(os.Stderr, err.(interface{ Help() string }).Help())
//
// Note que este é o único ponto em que errors.go alcança help.go. Não é o
// acoplamento parse/help/completion que o design proíbe — esses três continuam
// sem se conhecer; isto é a camada de apresentação pedindo texto ao
// renderizador, que é exatamente para o que ela serve.
func (u usageError) Help() string {
	return Help(u.Path)
}

// ErrHelpRequested reports that --help or -h appeared on the command line.
//
// EN — It is a SENTINEL, not a failure. The user asked for help and got help;
// nothing went wrong. Returning it as an error is a deliberate reuse of the
// error channel, for one reason: Parse already has exactly one early-exit path,
// and adding a second — a valid Context carrying a HelpRequested bool — would
// force every caller to test two conditions, in the right order, forever.
//
// With the sentinel, main stays one error check with two outcomes:
//
//	ctx, err := argvine.Parse(root, os.Args[1:])
//	if err != nil {
//	    var help *argvine.ErrHelpRequested
//	    if errors.As(err, &help) {
//	        fmt.Print(help.Help())
//	        os.Exit(0)          // zero: this was not a failure
//	    }
//	    fmt.Fprintln(os.Stderr, err)
//	    os.Exit(1)
//	}
//
// Same pattern as io.EOF: an error value that means "stop and hand control
// back", not "something broke".
//
// PT — É um SENTINELA, não uma falha. O usuário pediu ajuda e recebeu ajuda;
// nada deu errado. Devolvê-lo como erro é reúso deliberado do canal de erro, por
// uma razão: o Parse já tem exatamente um caminho de saída antecipada, e
// acrescentar um segundo — um Context válido carregando um bool HelpRequested —
// obrigaria todo chamador a testar duas condições, na ordem certa, para sempre.
//
// Com o sentinela, a main continua sendo uma checagem de erro com dois
// desfechos, e códigos de saída distintos.
//
// Mesmo padrão do io.EOF: um valor de erro que significa "pare e devolva o
// controle", não "algo quebrou".
type ErrHelpRequested struct {
	usageError
}

// Error satisfies the error interface but is not meant to be shown to anyone.
//
// EN — The caller is expected to detect this type and print Help() instead. The
// text exists so that a caller which forgets to branch prints something
// self-explanatory rather than "<nil>".
//
// PT — Espera-se que quem chama detecte este tipo e imprima Help() no lugar. O
// texto existe para que um chamador que esqueça de ramificar imprima algo
// autoexplicativo em vez de "<nil>".
func (e *ErrHelpRequested) Error() string {
	return "help requested for " + e.PathString()
}
