package argvine

import (
	"errors"
	"reflect"
	"testing"
)

// argsTree gives each arity its own subcommand, plus one that combines two.
//
// EN — Separate leaves rather than one command with every arity, because the
// arities interact: a single command declaring One, ZeroOrOne and Many would
// make it impossible to tell which branch consumed which token when something
// went wrong. The "mixed" leaf exists precisely to test the interaction, once,
// deliberately.
//
// PT — Folhas separadas em vez de um comando com todas as aridades, porque elas
// interagem: um único comando declarando One, ZeroOrOne e Many tornaria
// impossível saber qual ramo consumiu qual token quando algo desse errado. A
// folha "mixed" existe justamente para testar a interação, uma vez, de propósito.
func argsTree() *Command {
	return &Command{
		Name: "app",
		Sub: []*Command{
			{
				Name: "one",
				Args: []Arg{{Name: "target", Arity: One}},
			},
			{
				Name: "opt",
				Args: []Arg{{Name: "target", Arity: ZeroOrOne}},
			},
			{
				Name: "many",
				Args: []Arg{{Name: "ids", Arity: Many}},
			},
			{
				Name: "mixed",
				Args: []Arg{
					{Name: "name", Arity: One},
					{Name: "tags", Arity: Many},
				},
			},
		},
	}
}

// TestBindArgsArity is the truth table of the three arities.
//
// EN — Each arity is covered twice, satisfied and unsatisfied, because the
// interesting half of a rule is where it says no:
//
//	One       "one a"   → binds        | "one"   → ErrMissingArg
//	ZeroOrOne "opt a"   → binds        | "opt"   → binds nil, NOT an error
//	Many      "many 1 2 3" → binds all | "many"  → ErrMissingArg
//
// The pair worth staring at is ZeroOrOne-absent versus Many-with-nothing. Both
// arrive at bindArgs with an empty rest, and they take opposite branches: the
// first is a legitimate outcome, the second is a usage error. That difference
// is the entire meaning of the two constants, and a test that only exercised
// the satisfied cases would let them be swapped without noticing.
//
// reflect.DeepEqual is used rather than comparing lengths because nil and an
// empty slice are different results here, and only DeepEqual tells them apart.
//
// PT — Cada aridade é coberta duas vezes, satisfeita e não satisfeita, porque a
// metade interessante de uma regra é onde ela diz não:
//
//	One       "one a"   → liga         | "one"   → ErrMissingArg
//	ZeroOrOne "opt a"   → liga         | "opt"   → liga nil, NÃO é erro
//	Many      "many 1 2 3" → liga tudo | "many"  → ErrMissingArg
//
// O par que merece atenção é ZeroOrOne-ausente contra Many-sem-nada. Os dois
// chegam ao bindArgs com rest vazio e tomam ramos opostos: o primeiro é
// resultado legítimo, o segundo é erro de uso. Essa diferença é o significado
// inteiro das duas constantes, e um teste que só exercitasse os casos
// satisfeitos deixaria trocarem uma pela outra sem ninguém notar.
//
// reflect.DeepEqual é usado em vez de comparar tamanhos porque nil e slice
// vazia são resultados diferentes aqui, e só o DeepEqual distingue os dois.
func TestBindArgsArity(t *testing.T) {
	root := argsTree()
	// EN: Validate first — a test against a malformed tree fails in a way that
	// looks like a parser bug.
	// PT: Validate primeiro — teste contra árvore malformada falha de um jeito
	// que parece bug do parser.
	if err := root.Validate(); err != nil {
		t.Fatalf("tree must be valid: %v", err)
	}

	tests := []struct {
		name    string
		argv    []string
		want    map[string][]string
		wantErr bool
	}{
		{
			name: "One takes exactly one",
			argv: []string{"one", "a"},
			want: map[string][]string{"target": {"a"}},
		},
		{name: "One missing", argv: []string{"one"}, wantErr: true},
		{
			// EN: nil, not an error, and not an empty slice either.
			// PT: nil, não erro — e também não slice vazia.
			name: "ZeroOrOne absent",
			argv: []string{"opt"},
			want: map[string][]string{"target": nil},
		},
		{
			name: "ZeroOrOne present",
			argv: []string{"opt", "a"},
			want: map[string][]string{"target": {"a"}},
		},
		{
			name: "Many takes the rest",
			argv: []string{"many", "1", "2", "3"},
			want: map[string][]string{"ids": {"1", "2", "3"}},
		},
		{name: "Many with nothing", argv: []string{"many"}, wantErr: true},
		{
			// EN: The greedy-from-the-left rule in one line: One takes the
			// minimum it needs, Many takes whatever is left over.
			// PT: A regra do guloso-da-esquerda numa linha: One pega o mínimo
			// que precisa, Many leva tudo o que sobrou.
			name: "One then Many splits at the first",
			argv: []string{"mixed", "release", "urgent", "backend"},
			want: map[string][]string{"name": {"release"}, "tags": {"urgent", "backend"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := Parse(root, tt.argv)
			if tt.wantErr {
				// EN: Assert the concrete type, never just "some error". A bare
				// err != nil check would be satisfied by ErrUnknownCommand or
				// any other failure and would stop testing arity entirely.
				// PT: Afirma o tipo concreto, nunca só "algum erro". Um
				// err != nil seria satisfeito por ErrUnknownCommand ou qualquer
				// outra falha, e deixaria de testar aridade por completo.
				var e *ErrMissingArg
				if !errors.As(err, &e) {
					t.Fatalf("Parse(%v) = %v (%T), want *ErrMissingArg", tt.argv, err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%v) returned %v", tt.argv, err)
			}
			for name, want := range tt.want {
				got := ctx.ArgList(name)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("ArgList(%q) = %v, want %v", name, got, want)
				}
			}
		})
	}
}

// TestLeftoverPositionals covers the last gap in the parser: a token that no
// declared Arg claimed.
//
// EN — The situation is one thing; the diagnosis is two, and which one you get
// depends on the shape of the node the walk stopped at:
//
//	"task remote nope"      → remote HAS children  → ErrUnknownCommand
//	"task remote add a b c" → add is a leaf        → ErrUnexpectedArg
//
// The test for whether two errors should be one: does the user do the same
// thing next? Here they do not — one goes looking for the list of subcommands,
// the other deletes an argument — so they are two types. A single "unexpected
// token" would be correct and useless.
//
// The third subtest is the negative space: a leaf ending in Many can never
// reach the leftover branch at all, because Many empties rest by definition.
// Without it, an implementation that reported leftovers BEFORE binding args
// would pass the first two and break every Many command.
//
// PT — A situação é uma; o diagnóstico são dois, e qual deles você recebe
// depende da forma do nó onde a varredura parou:
//
//	"task remote nope"      → remote TEM filhos → ErrUnknownCommand
//	"task remote add a b c" → add é folha       → ErrUnexpectedArg
//
// O teste para saber se dois erros deveriam ser um só: o usuário faz a mesma
// coisa em seguida? Aqui não — um vai procurar a lista de subcomandos, o outro
// apaga um argumento — então são dois tipos. Um "token inesperado" único seria
// correto e inútil.
//
// O terceiro subteste é o espaço negativo: uma folha terminada em Many nunca
// alcança o ramo de sobra, porque Many esvazia o rest por definição. Sem ele,
// uma implementação que reportasse sobras ANTES de ligar os args passaria nos
// dois primeiros e quebraria todo comando com Many.
func TestLeftoverPositionals(t *testing.T) {
	t.Run("token on a node with subcommands is an unknown command", func(t *testing.T) {
		_, err := Parse(remoteTree(), []string{"remote", "nope"})

		var e *ErrUnknownCommand
		if !errors.As(err, &e) {
			t.Fatalf("got %v (%T), want *ErrUnknownCommand", err, err)
		}
		if e.Got != "nope" {
			t.Errorf("Got = %q, want \"nope\"", e.Got)
		}
		// EN: "task remote", not "task" — the error is reported from where the
		// walk actually stopped, which is the node whose children the user was
		// trying to name.
		// PT: "task remote", não "task" — o erro é reportado de onde a varredura
		// de fato parou, que é o nó cujos filhos o usuário tentava nomear.
		if got := e.PathString(); got != "task remote" {
			t.Errorf("PathString() = %q, want \"task remote\"", got)
		}
	})

	t.Run("extra token on a leaf is an unexpected argument", func(t *testing.T) {
		_, err := Parse(remoteTree(), []string{"remote", "add", "a", "b", "c"})

		var e *ErrUnexpectedArg
		if !errors.As(err, &e) {
			t.Fatalf("got %v (%T), want *ErrUnexpectedArg", err, err)
		}
		// EN: "add" declares two Args, so "a" and "b" are claimed and "c" is the
		// first unclaimed token — the exact place the command line stopped
		// making sense.
		// PT: "add" declara dois Args, então "a" e "b" são reivindicados e "c" é
		// o primeiro token não reivindicado — exatamente onde a linha de comando
		// deixou de fazer sentido.
		if e.Got != "c" {
			t.Errorf("Got = %q, want \"c\"", e.Got)
		}
	})

	// EN: The negative case. Four tokens for one Arg is not "too many" when that
	// Arg is Many — it is the whole point. This guards the ordering inside
	// bindArgs: the leftover check must come AFTER binding, never before.
	// PT: O caso negativo. Quatro tokens para um Arg não é "demais" quando esse
	// Arg é Many — é justamente o objetivo. Isto protege a ordem dentro do
	// bindArgs: a checagem de sobra tem que vir DEPOIS da ligação, nunca antes.
	t.Run("a leaf that declares Many never has leftovers", func(t *testing.T) {
		if _, err := Parse(argsTree(), []string{"many", "1", "2", "3", "4"}); err != nil {
			t.Fatalf("Many should absorb every leftover: %v", err)
		}
	})
}
