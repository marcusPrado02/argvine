package argvine

import "testing"

// TestFlagString pins the strings that reach the user.
//
// EN — These values are not internal labels: they are printed in generated help
// ("--priority int") and in type errors ("expected int"). Changing one changes
// the CLI's output, so the table is the contract, not a convenience.
//
// PT — Estes valores não são rótulos internos: são impressos no help gerado
// ("--priority int") e em erros de tipo ("expected int"). Mudar um muda a saída
// da CLI, então a tabela é contrato, não conveniência.
func TestFlagString(t *testing.T) {
	tests := []struct {
		typ  FlagType
		want string
	}{
		{Bool, "bool"},
		{String, "string"},
		{Int, "int"},
	}

	for _, tt := range tests {
		// EN: %d prints the numeric value even though FlagType is a Stringer —
		// numeric verbs ignore String(), which is what we want when the thing
		// under test is String() itself.
		// PT: %d imprime o valor numérico mesmo FlagType sendo Stringer — verbos
		// numéricos ignoram String(), que é o que queremos quando o que está sob
		// teste é o próprio String().
		if got := tt.typ.String(); got != tt.want {
			t.Errorf("FlagType(%d).String() = %q, want %q", tt.typ, got, tt.want)
		}
	}
}

// TestTreeIsInspectable checks the property the whole design rests on: a CLI is
// one composite literal, and everything about it can be read back out of that
// literal without running anything.
//
// EN — It looks trivial, and that is the point. If this test ever needs a
// constructor call, a registration step or a second phase to wire parents to
// children, the model has stopped being declarative, and help and completion
// can no longer be pure readers of the tree.
//
// PT — Parece trivial, e é justamente esse o ponto. Se algum dia este teste
// precisar de um construtor, de uma etapa de registro ou de uma segunda fase
// costurando pais a filhos, o modelo deixou de ser declarativo, e help e
// completion não conseguem mais ser leitores puros da árvore.
func TestTreeIsInspectable(t *testing.T) {
	root := &Command{
		// EN: Lowercase on purpose in a real CLI — the name is the word typed
		// at the shell, and argv is case-sensitive.
		// PT: Minúsculo de propósito numa CLI real — o nome é a palavra digitada
		// no shell, e argv diferencia maiúsculas de minúsculas.
		Name:  "task",
		Short: "local task manager",
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
						Usage: "priority from 1 (high) to 5 (low)."},
				},
				Args: []Arg{
					{Name: "title", Arity: One, Usage: "task title"},
				},
				Run: func(ctx *Context) error { return nil },
			},
		},
	}

	add := root.findSub("add")
	if add == nil {
		// EN: Fatal, not Error — every assertion below dereferences add.
		// PT: Fatal, não Error — toda asserção abaixo desreferencia add.
		t.Fatal("findSub(\"add\") returned nil, want the add command")
	}
	if got := add.Flags[0].Name; got != "priority" {
		t.Errorf("add.Flags[0].Name = %q, want \"priority\"", got)
	}
	// EN: Arity has no String method, so %d is the honest verb here — the
	// failure message shows the raw constant value rather than a name that does
	// not exist.
	// PT: Arity não tem método String, então %d é o verbo honesto aqui — a
	// mensagem de falha mostra o valor bruto da constante em vez de um nome que
	// não existe.
	if got := add.Args[0].Arity; got != One {
		t.Errorf("add.Args[0].Arity = %d, want One", got)
	}

	// EN: The negative case matters as much as the positive one — a findSub
	// that returned the first child regardless of name would pass every check
	// above.
	// PT: O caso negativo importa tanto quanto o positivo — um findSub que
	// devolvesse o primeiro filho independentemente do nome passaria em todas as
	// checagens acima.
	if root.findSub("nope") != nil {
		t.Error("findSub(\"nope\") should be nil")
	}
}

// TestValidateRejectBadFlags enumerates every way a flag declaration can be
// wrong. Each case is a tree a developer could plausibly write by accident.
//
// EN — The assertion is only "an error came back", not which message: the
// messages are for humans and will be reworded, while the set of rejected
// shapes is the actual contract. Pinning the text here would make every wording
// improvement look like a behavior change.
//
// PT — A asserção é só "veio um erro", não qual mensagem: as mensagens são para
// humanos e serão reescritas, enquanto o conjunto de formas recusadas é o
// contrato de verdade. Fixar o texto aqui faria toda melhoria de redação parecer
// mudança de comportamento.
func TestValidateRejectBadFlags(t *testing.T) {
	tests := []struct {
		name string
		root *Command
	}{
		{
			name: "duplicate long name",
			root: &Command{Name: "app", Flags: []Flag{
				{Name: "force", Type: Bool},
				{Name: "force", Type: Bool},
			}},
		},
		{
			name: "short flag longer than one character",
			root: &Command{Name: "app", Flags: []Flag{
				{Name: "force", Short: "fo", Type: Bool},
			}},
		},
		{
			name: "duplicate short name",
			root: &Command{Name: "app", Flags: []Flag{
				{Name: "force", Short: "f", Type: Bool},
				{Name: "fast", Short: "f", Type: Bool},
			}},
		},
		{
			name: "choices on a bool flag",
			root: &Command{Name: "app", Flags: []Flag{
				{Name: "force", Type: Bool, Choices: []string{"yes", "no"}},
			}},
		},
		{
			name: "default type does not match flag type",
			root: &Command{Name: "app", Flags: []Flag{
				{Name: "priority", Type: Int, Default: "high"},
			}},
		},
		{
			name: "empty flag name",
			root: &Command{Name: "app", Flags: []Flag{
				{Name: "", Type: Bool},
			}},
		},
	}

	for _, tt := range tests {
		// EN: A subtest per case so a failure names the shape that slipped
		// through, instead of reporting "one of six trees was accepted".
		// PT: Um subteste por caso para que a falha nomeie a forma que passou,
		// em vez de reportar "uma das seis árvores foi aceita".
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.root.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want an error for %q", tt.name)
			}
		})
	}
}

// TestValidateAcceptWellFormedTree is the counterweight to the table above.
//
// EN — Without it, a Validate that returned an error unconditionally would pass
// every negative case. Any suite built from rejections needs at least one
// acceptance, or it is only testing that the function is pessimistic.
//
// PT — Sem ele, um Validate que devolvesse erro incondicionalmente passaria em
// todos os casos negativos. Toda suíte construída sobre recusas precisa de pelo
// menos uma aceitação, senão só testa que a função é pessimista.
func TestValidateAcceptWellFormedTree(t *testing.T) {
	root := &Command{
		Name: "app",
		Flags: []Flag{
			{Name: "verbose", Short: "v", Type: Bool, Default: false},
		},
		Sub: []*Command{
			{Name: "serve", Flags: []Flag{
				{Name: "port", Short: "p", Type: Int, Default: 8080},
			}},
		},
	}

	if err := root.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

// TestValidateRejectsBadArgsAndSubs covers the rules about shape rather than
// about flags: positional arity, subcommand names, and the fact that validate
// now walks the whole tree instead of only the root.
//
// EN — The last two cases carry the most weight. "invalid flag inside a nested
// subcommand" fails only if the recursion reaches depth two, and "subcommand
// short flag collides with an inherited persistent flag" fails only if
// persistent flags are actually threaded down. Both would pass silently against
// a non-recursive validate, which is what makes them a real test of this one.
//
// PT — Os dois últimos casos são os que mais pesam. "invalid flag inside a
// nested subcommand" só falha se a recursão alcançar profundidade dois, e
// "subcommand short flag collides with an inherited persistent flag" só falha
// se as persistentes forem de fato repassadas para baixo. Os dois passariam
// calados contra um validate não-recursivo, e é isso que os torna um teste real
// deste aqui.
func TestValidateRejectsBadArgsAndSubs(t *testing.T) {
	tests := []struct {
		name string
		root *Command
	}{
		{
			name: "Many arity not on the last arg",
			root: &Command{Name: "app", Args: []Arg{
				{Name: "ids", Arity: Many},
				{Name: "target", Arity: One},
			}},
		},
		{
			name: "empty arg name",
			root: &Command{Name: "app", Args: []Arg{
				{Name: "", Arity: One},
			}},
		},
		{
			name: "duplicate subcommand name",
			root: &Command{Name: "app", Sub: []*Command{
				{Name: "serve"},
				{Name: "serve"},
			}},
		},
		{
			name: "invalid flag inside a nested subcommand",
			root: &Command{Name: "app", Sub: []*Command{
				{Name: "remote", Sub: []*Command{
					{Name: "add", Flags: []Flag{{Name: "force", Short: "fo", Type: Bool}}},
				}},
			}},
		},
		{
			name: "subcommand short flag collides with an inherited persistent flag",
			root: &Command{
				Name:  "app",
				Flags: []Flag{{Name: "verbose", Short: "v", Type: Bool, Persistent: true}},
				Sub: []*Command{
					{Name: "serve", Flags: []Flag{{Name: "version", Short: "v", Type: Bool}}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.root.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want an error for %s", tt.name)
			}
		})
	}
}

// TestValidateSiblingsDoNotShareInheritance pins the direction of inheritance:
// flags flow down from a parent, never sideways between siblings.
//
// EN — build declares -o as persistent and serve declares its own -o. They are
// only a collision if serve can somehow see build's flags — which it must not,
// since nobody typing "app serve" has any way to reach a flag that belongs to
// "app build".
//
// This is a regression guard, not a bug reproduction. It passes against the
// naive `next := inherited` too, because each child receives its own slice
// length and never reads past it. It is here to fail loudly if someone later
// "optimizes" the inheritance by sharing a single accumulator across the walk.
//
// PT — build declara -o como persistente e serve declara o próprio -o. Só são
// colisão se serve conseguir de algum modo enxergar as flags de build — o que
// não pode acontecer, já que ninguém digitando "app serve" tem como alcançar uma
// flag que pertence a "app build".
//
// Isto é guarda de regressão, não reprodução de bug. Passa também contra o
// ingênuo `next := inherited`, porque cada filho recebe o próprio comprimento de
// slice e nunca lê além dele. Está aqui para falhar alto se alguém depois
// "otimizar" a herança compartilhando um acumulador único na travessia.
func TestValidateSiblingsDoNotShareInheritance(t *testing.T) {
	root := &Command{
		Name:  "app",
		Flags: []Flag{{Name: "verbose", Short: "v", Type: Bool, Persistent: true}},
		Sub: []*Command{
			{Name: "build", Flags: []Flag{{Name: "output", Short: "o", Type: String, Persistent: true}}},
			{Name: "serve", Flags: []Flag{{Name: "open", Short: "o", Type: Bool}}},
		},
	}

	if err := root.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil: -o on serve must not collide with -o on its sibling build", err)
	}
}
