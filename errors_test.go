package argvine

import (
	"errors"
	"strings"
	"testing"
)

// TestUsageErrorsCarryPathAndHint checks the contract every usage error shares,
// rather than the wording of any one of them.
//
// EN — Three assertions, three different things:
//
//   - errors.As succeeds → the caller can branch on the specific failure, which
//     is the whole reason these are types instead of fmt.Errorf strings
//   - the message mentions --help → it tells the user what to do next
//   - the message has a newline → the two-part shape survived
//
// None of them pins a phrase. Wording will be rewritten; the shape is contract.
//
// Note errors.As needs a POINTER to the target type, so target holds a
// **ErrUnknownFlag — new(*ErrUnknownFlag), not new(ErrUnknownFlag).
//
// PT — Três asserções, três coisas diferentes:
//
//   - errors.As dá certo → quem chama consegue ramificar na falha específica,
//     que é a razão inteira de serem tipos e não strings de fmt.Errorf
//   - a mensagem cita --help → ela diz ao usuário o que fazer em seguida
//   - a mensagem tem quebra de linha → a forma de duas partes sobreviveu
//
// Nenhuma delas fixa uma frase. A redação será reescrita; a forma é contrato.
//
// Note que errors.As precisa de um PONTEIRO para o tipo alvo, então target
// guarda um **ErrUnknownFlag — new(*ErrUnknownFlag), não new(ErrUnknownFlag).
func TestUsageErrorsCarryPathAndHint(t *testing.T) {
	root := remoteTree()

	tests := []struct {
		name   string
		argv   []string
		target any
	}{
		{name: "unknown flag", argv: []string{"remote", "add", "--nope"}, target: new(*ErrUnknownFlag)},
		// EN: --config is declared on the root and is the last token, so there
		// is nothing left to consume as its value.
		// PT: --config é declarada na raiz e é o último token, então não sobra
		// nada para ser consumido como valor dela.
		{name: "missing value", argv: []string{"--config"}, target: new(*ErrMissingValue)},

		// EN: "maybe" is not a bool. The inline form is used so the failure is
		// in conversion and not in a missing value.
		// PT: "maybe" não é booleano. A forma inline é usada para que a falha
		// seja na conversão e não em valor faltando.
		{name: "bad type", argv: []string{"remote", "add", "--force=maybe", "a", "b"}, target: new(*ErrBadType)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(root, tt.argv)
			if err == nil {
				t.Fatalf("Parse(%v) = nil error, want one", tt.argv)
			}
			if !errors.As(err, tt.target) {
				t.Fatalf("Parse(%v) returned %T, want %T", tt.argv, err, tt.target)
			}

			msg := err.Error()
			if !strings.Contains(msg, "--help") {
				t.Errorf("message %q should point the user at --help", msg)
			}
			if strings.Count(msg, "\n") == 0 {
				t.Errorf("message %q should have a what-happened line and a what-to-do-next line", msg)
			}
		})
	}
}

// TestUnknownFlagNamesTheDeepestCommand is the one test here that reaches into
// a concrete type, and it earns that by checking the two fields a caller would
// actually want to branch on.
//
// EN — "task remote add" rather than "add" is the point: the error was built
// from the walk's Path at the moment of failure, not from the leaf's name. An
// implementation that reported Cmd.Name would pass every other test in this
// file and still leave the user hunting for which "add" was meant.
//
// PT — "task remote add" em vez de "add" é o ponto: o erro foi construído a
// partir do Path da travessia no momento da falha, não do nome da folha. Uma
// implementação que reportasse Cmd.Name passaria em todos os outros testes deste
// arquivo e ainda deixaria o usuário caçando qual "add" era.
func TestUnknownFlagNamesTheDeepestCommand(t *testing.T) {
	_, err := Parse(remoteTree(), []string{"remote", "add", "--nope"})

	var uf *ErrUnknownFlag
	if !errors.As(err, &uf) {
		t.Fatalf("got %T, want *ErrUnknownFlag", err)
	}
	if got := uf.PathString(); got != "task remote add" {
		t.Errorf("PathString() = %q, want \"task remote add\"", got)
	}
	if uf.Flag != "--nope" {
		t.Errorf("Flag = %q, want \"--nope\"", uf.Flag)
	}
}

// choicesTree is flat on purpose.
//
// EN — Nesting would add nothing here, because Required and Choices are checked
// over the flags of the walked path and have no relationship to depth.
//
// The three flags cover the three cases that behave differently: one required
// with no default, one with Choices on a string, and one with Choices on an int
// — the last existing to prove that the comparison is not string-only.
//
// PT — Aninhar não acrescentaria nada aqui, porque Required e Choices são
// checados sobre as flags do caminho percorrido e não têm relação com
// profundidade.
//
// As três flags cobrem os três casos que se comportam diferente: uma obrigatória
// sem default, uma com Choices em string, e uma com Choices em int — esta última
// existindo para provar que a comparação não é só para strings.
func choicesTree() *Command {
	return &Command{
		Name: "app",
		Flags: []Flag{
			{Name: "token", Type: String, Required: true, Usage: "api token"},
			{Name: "status", Type: String, Default: "open",
				Choices: []string{"open", "done", "all"}, Usage: "filter"},
			{Name: "level", Type: Int, Default: 1,
				Choices: []string{"1", "2", "3"}, Usage: "verbosity"},
		},
	}
}

// TestRequiredAndChoices covers the two checks that can only run once the whole
// command line has been read.
//
// EN — The last subtest is the one people leave out. Required and Choices are
// easy to implement in a way that also rejects perfectly good input: if the
// check ran over every declared flag instead of only the ones the user typed, a
// flag left at its default would be validated too, and a CLI author who chose a
// default outside their own Choices would blame the user for it.
//
// PT — O último subteste é o que as pessoas deixam de fora. Required e Choices
// são fáceis de implementar de um jeito que também recusa entrada perfeitamente
// boa: se a checagem rodasse sobre toda flag declarada em vez de só as que o
// usuário digitou, uma flag deixada no default seria validada também, e um autor
// de CLI que escolheu um default fora do próprio Choices culparia o usuário.
func TestRequiredAndChoices(t *testing.T) {
	root := choicesTree()
	// EN: Validate first — a test written against a malformed tree would fail
	// in a way that looks like a parser bug.
	// PT: Validate primeiro — um teste escrito contra árvore malformada falharia
	// de um jeito que parece bug do parser.
	if err := root.Validate(); err != nil {
		t.Fatalf("tree must be valid: %v", err)
	}

	t.Run("missing required flag", func(t *testing.T) {
		_, err := Parse(root, nil)
		var e *ErrMissingRequired
		if !errors.As(err, &e) {
			t.Fatalf("got %v (%T), want *ErrMissingRequired", err, err)
		}
		if e.Flag.Name != "token" {
			t.Errorf("Flag.Name = %q, want \"token\"", e.Flag.Name)
		}
	})

	t.Run("value outside choices", func(t *testing.T) {
		_, err := Parse(root, []string{"--token", "x", "--status", "closed"})

		var e *ErrBadChoice
		if !errors.As(err, &e) {
			t.Fatalf("got %v (%T), want *ErrBadChoice", err, err)
		}
		// EN: Listing the accepted values is the whole point of this error.
		// Someone who typed "closed" instead of "done" should not have to go
		// find --help to learn the three words that were allowed.
		// PT: Listar os valores aceitos é o ponto inteiro deste erro. Quem
		// digitou "closed" em vez de "done" não deveria ter que ir atrás do
		// --help para descobrir as três palavras permitidas.
		if !strings.Contains(e.Error(), "open, done, all") {
			t.Errorf("message %q should list the accepted values", e.Error())
		}
	})

	t.Run("int choices are honored", func(t *testing.T) {
		// EN: 9 is a perfectly good int, so convert succeeds and the rejection
		// has to come from the Choices check. A non-numeric value here would
		// make the subtest pass on ErrBadType and never reach Choices at all —
		// green, and testing the wrong thing.
		// PT: 9 é um int perfeitamente bom, então o convert dá certo e a recusa
		// tem que vir da checagem de Choices. Um valor não-numérico aqui faria o
		// subteste passar por ErrBadType e nunca chegar a Choices — verde, e
		// testando a coisa errada.
		_, err := Parse(root, []string{"--token", "x", "--level", "9"})

		var e *ErrBadChoice
		if !errors.As(err, &e) {
			t.Fatalf("got %v (%T), want *ErrBadChoice: --level 9 converts fine but is not one of 1, 2, 3", err, err)
		}
		if e.Got != "9" {
			t.Errorf("Got = %q, want \"9\": the parsed int is rendered back to a string for the comparison", e.Got)
		}

		if _, err := Parse(root, []string{"--token", "x", "--level", "2"}); err != nil {
			t.Fatalf("--level 2 should be accepted: %v", err)
		}
	})

	// EN: The only positive case in this test, and the one that constrains the
	// implementation most — --status and --level are absent, so they hold their
	// defaults and must not be validated at all.
	// PT: O único caso positivo deste teste, e o que mais restringe a
	// implementação — --status e --level estão ausentes, então seguram seus
	// defaults e não podem ser validados de jeito nenhum.
	t.Run("default is not checked against choices", func(t *testing.T) {
		if _, err := Parse(root, []string{"--token", "x"}); err != nil {
			t.Fatalf("absent flag with a valid default should parse: %v", err)
		}
	})
}
