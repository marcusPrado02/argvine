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
