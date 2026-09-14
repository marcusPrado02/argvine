// Package argvine is a dependency-free command-line framework for Go.
//
// EN — A CLI is described as an immutable tree of Command nodes. Parsing, help
// rendering and shell completion are three independent readers of that same
// tree, which is what keeps them from ever disagreeing with each other: a flag
// added to the tree shows up in all three without anyone editing three places.
//
// PT — Uma CLI é descrita como uma árvore imutável de nós Command. Parsing,
// renderização do help e completion de shell são três leitores independentes
// dessa mesma árvore, e é isso que impede que eles discordem entre si: uma flag
// acrescentada à árvore aparece nos três sem ninguém editar três lugares.
package argvine

import (
	"fmt"
)

// FlagType enumerates the value types a Flag can hold.
//
// EN — The type is what decides whether a flag consumes the token that follows
// it on the command line, so it must be known before parsing begins. That is
// why it lives in the declaration rather than being inferred while parsing.
//
// PT — O tipo é o que decide se uma flag consome o token seguinte na linha de
// comando, então ele precisa ser conhecido antes de o parsing começar. É por
// isso que mora na declaração em vez de ser inferido durante a varredura.
type FlagType int

const (
	// EN: Bool flags never consume the following token: "--verbose x" leaves x
	// as a positional argument. An explicit value still works as
	// "--verbose=false".
	// PT: Flags Bool nunca consomem o token seguinte: "--verbose x" deixa x
	// como argumento posicional. Valor explícito ainda funciona como
	// "--verbose=false".
	Bool FlagType = iota

	// EN: String flags consume the following token verbatim.
	// PT: Flags String consomem o token seguinte literalmente.
	String

	// EN: Int flags consume the following token and parse it as a base-10 integer.
	// PT: Flags Int consomem o token seguinte e o parseiam como inteiro base 10.
	Int
)

// String renders the type as it appears in generated output.
//
// EN — Not a debugging aid: generated help prints "--priority int", and type
// errors report the type they expected. This is part of the public surface.
//
// PT — Não é auxílio de depuração: o help gerado imprime "--priority int", e
// erros de tipo reportam o tipo esperado. Isto faz parte da superfície pública.
func (t FlagType) String() string {
	switch t {
	case Bool:
		return "bool"
	case String:
		return "string"
	case Int:
		return "int"
	default:
		return "unknown"
	}
}

// Arity describes how many values a positional Arg accepts.
type Arity int

const (
	// EN: One requires exactly one value; its absence is a usage error.
	// PT: One exige exatamente um valor; sua ausência é erro de uso.
	One Arity = iota

	// EN: ZeroOrOne accepts an optional single value. Absence is a legitimate
	// outcome, not an error.
	// PT: ZeroOrOne aceita um único valor opcional. Ausência é resultado
	// legítimo, não erro.
	ZeroOrOne

	// EN: Many accepts one or more values and absorbs every remaining
	// positional, which is why it is only valid on the last Arg of a command.
	// Note "one or more", not "zero or more": an empty remainder is an error.
	// PT: Many aceita um ou mais valores e absorve todo posicional restante, e
	// por isso só é válido no último Arg de um comando. Note "um ou mais", não
	// "zero ou mais": resto vazio é erro.
	Many
)

// Flag describes a named option.
//
// EN — A Flag is immutable description: parsing never writes into it, and every
// parsed value lands in a Context instead. That separation between description
// and result is what makes Parse a pure function, and it is why Command holds
// []Flag by value rather than []*Flag.
//
// PT — Uma Flag é descrição imutável: o parsing nunca escreve dentro dela, e
// todo valor parseado vai parar num Context. Essa separação entre descrição e
// resultado é o que torna o Parse uma função pura, e é por isso que Command
// guarda []Flag por valor e não []*Flag.
type Flag struct {
	// EN: Name is the long form, written without dashes: "force" renders as --force.
	// PT: Name é a forma longa, escrita sem hifens: "force" vira --force.
	Name string

	// EN: Short is the single-character form, without the dash. Empty means the
	// flag has no short form.
	// PT: Short é a forma de um caractere, sem o hífen. Vazio significa que a
	// flag não tem forma curta.
	Short string

	// EN: Type decides whether this flag consumes the token that follows it.
	// PT: Type decide se esta flag consome o token que vem depois dela.
	Type FlagType

	// EN: Default is the value used when the flag is absent. Its dynamic type
	// must match Type; Validate rejects the mismatch at startup.
	// PT: Default é o valor usado quando a flag está ausente. Seu tipo dinâmico
	// precisa bater com Type; o Validate recusa a divergência no boot.
	Default any

	// EN: Usage is the one-line description rendered in generated help.
	// PT: Usage é a descrição de uma linha renderizada no help gerado.
	Usage string

	// EN: Required makes the flag mandatory on the command that declares it.
	// PT: Required torna a flag obrigatória no comando que a declara.
	Required bool

	// EN: Choices restricts the accepted values. Validated only when non-nil,
	// and meaningless on a Bool flag, which Validate rejects.
	// PT: Choices restringe os valores aceitos. Validado só quando não-nil, e
	// sem sentido numa flag Bool, o que o Validate recusa.
	Choices []string

	// EN: Persistent makes the flag visible to every descendant command, so a
	// root --verbose can be written after a deeply nested subcommand.
	// PT: Persistent torna a flag visível a todo comando descendente, de modo
	// que um --verbose da raiz possa ser escrito depois de um subcomando fundo.
	Persistent bool
}

// Arg describes a positional argument.
type Arg struct {
	// EN: Name is both the lookup key for Context.Arg and the placeholder
	// rendered as <name> in the usage line.
	// PT: Name é ao mesmo tempo a chave de busca do Context.Arg e o marcador
	// renderizado como <name> na linha de uso.
	Name string

	// EN: Arity decides how many values this argument consumes.
	// PT: Arity decide quantos valores este argumento consome.
	Arity Arity

	// EN: Usage is the one-line description rendered in generated help.
	// PT: Usage é a descrição de uma linha renderizada no help gerado.
	Usage string

	// EN: Choices restricts the accepted values, the same way it does on a
	// Flag. Not in the v1 design; wire it into validation or drop it.
	// PT: Choices restringe os valores aceitos, do mesmo jeito que numa Flag.
	// Não está no design da v1; ligue à validação ou remova.
	Choices []string
}

// Command is a node in the command tree.
//
// EN — The whole CLI is meant to be written as a single composite literal: a
// node holds its children directly, and no node points back at its parent.
// Keeping the tree acyclic is what lets Validate walk it without cycle
// detection, and what keeps the literal declarative — there is no second phase
// where parents get wired to children.
//
// PT — A CLI inteira deve ser escrita como um único literal composto: um nó
// guarda seus filhos diretamente, e nenhum nó aponta de volta para o pai.
// Manter a árvore acíclica é o que permite ao Validate percorrê-la sem detecção
// de ciclo, e o que mantém o literal declarativo — não existe uma segunda fase
// em que pais são costurados aos filhos.
type Command struct {
	// EN: Name is the word that selects this command on the command line.
	// PT: Name é a palavra que seleciona este comando na linha de comando.
	Name string

	// EN: Short is the one-line description shown when the parent lists its children.
	// PT: Short é a descrição de uma linha mostrada quando o pai lista os filhos.
	Short string

	// EN: Long is the paragraph shown in this command's own help. Help falls
	// back to Short when Long is empty.
	// PT: Long é o parágrafo mostrado no help do próprio comando. O Help recorre
	// ao Short quando Long está vazio.
	Long string

	// EN: Flags are the options declared directly on this command.
	// PT: Flags são as opções declaradas diretamente neste comando.
	Flags []Flag

	// EN: Args are the positional arguments this command accepts.
	// PT: Args são os argumentos posicionais que este comando aceita.
	Args []Arg

	// EN: Sub are the child commands.
	// PT: Sub são os comandos filhos.
	Sub []*Command

	// EN: Run is the handler. A node with children and no Run is a namespace:
	// the caller is expected to print help for it rather than execute anything.
	// PT: Run é o handler. Um nó com filhos e sem Run é um namespace: espera-se
	// que quem chama imprima o help dele em vez de executar qualquer coisa.
	Run func(*Context) error
}

// findSub returns the direct child named name, or nil when there is none.
//
// EN — A linear scan is deliberate: command trees have a handful of children
// per node, and a map would have to be built and kept in sync with Sub, which
// is exactly the kind of duplicated state this design exists to avoid.
//
// PT — A busca linear é deliberada: árvores de comando têm um punhado de filhos
// por nó, e um mapa teria que ser construído e mantido em sincronia com Sub,
// que é exatamente o tipo de estado duplicado que este design existe para
// evitar.
func (c *Command) findSub(name string) *Command {
	for _, s := range c.Sub {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// Validate walks the tree and reports the first structural problem it finds.
//
// EN — Call it once at startup, before any Parse. Every error it returns is a
// programming error in the CLI definition, never a usage error by whoever ran
// the CLI — which is exactly why it lives outside Parse. Run inside Parse, a
// malformed tree would come back as an ordinary error, indistinguishable from a
// mistyped flag, and the user would be shown a message only the developer could
// act on.
//
// It is also the safety net for the one weak spot of the Context design: flag
// names are strings, so ctx.Int("prot") only fails at runtime. Validate catches
// the declaration-side half of that class of mistake at boot.
//
// PT — Chame uma vez no boot, antes de qualquer Parse. Todo erro que devolve é
// erro de programação na definição da CLI, nunca erro de uso de quem rodou — e
// é exatamente por isso que mora fora do Parse. Rodando dentro do Parse, uma
// árvore malformada voltaria como erro comum, indistinguível de uma flag
// digitada errado, e o usuário veria uma mensagem que só o desenvolvedor pode
// resolver.
//
// É também a rede de segurança do único ponto fraco do design do Context: nomes
// de flag são strings, então ctx.Int("prot") só falha em runtime. O Validate
// pega a metade dessa classe de erro que fica do lado da declaração, no boot.
func (c *Command) Validate() error {
	return c.validate(nil)
}

// validate is the recursive half of Validate.
//
// EN — Step by step, for each node:
//
//  1. Seed two "already seen" sets with the inherited persistent flags, so a
//     child redeclaring an ancestor's flag is caught as a duplicate.
//  2. Check this node's own flags: empty name, duplicate, bad Short, Choices
//     on a Bool, Default of the wrong type.
//  3. Check the positional Args: empty name, Many out of last position.
//  4. Check the subcommands: empty or duplicate names.
//  5. Build what the CHILDREN inherit: a copy of what arrived, plus this
//     node's own persistent flags.
//  6. Recurse into each child with that new set.
//
// Steps 2–4 run before step 6 — pre-order — so a node's own problems are
// reported before its children's, and the first error points at the outermost
// mistake rather than a symptom deeper down.
//
// It returns the first problem rather than collecting all of them: this runs at
// startup and stops the program, so the developer fixes one and re-runs. A list
// of errors would cost complexity for no gain in that loop.
//
// PT — Passo a passo, para cada nó:
//
//  1. Semeia dois conjuntos de "já visto" com as flags persistentes herdadas,
//     para que um filho que redeclare a flag de um ancestral seja pego como
//     duplicata.
//  2. Checa as flags do próprio nó: nome vazio, duplicata, Short inválido,
//     Choices em Bool, Default de tipo errado.
//  3. Checa os Args posicionais: nome vazio, Many fora da última posição.
//  4. Checa os subcomandos: nomes vazios ou repetidos.
//  5. Monta o que os FILHOS herdam: cópia do que chegou, mais as persistentes
//     deste nó.
//  6. Recursa em cada filho com esse conjunto novo.
//
// Os passos 2–4 rodam antes do passo 6 — pré-ordem — para que os problemas do
// próprio nó sejam reportados antes dos dos filhos, e o primeiro erro aponte
// para o engano mais externo em vez de um sintoma mais fundo.
//
// Devolve o primeiro problema em vez de coletar todos: isso roda no boot e para
// o programa, então o desenvolvedor conserta um e roda de novo. Uma lista de
// erros custaria complexidade sem ganho nesse laço.
func (c *Command) validate(inherited []Flag) error {
	// EN: Seed the seen-sets with what this command inherits, so a child that
	// redeclares an ancestor's persistent flag is caught as a duplicate.
	// PT: Semeia os conjuntos de vistos com o que este comando herda, para que
	// um filho que redeclare uma persistente do ancestral vire duplicata.
	seenName := make(map[string]bool, len(inherited)+len(c.Flags))
	seenShort := make(map[string]bool, len(inherited)+len(c.Flags))
	for _, f := range inherited {
		seenName[f.Name] = true
		if f.Short != "" {
			seenShort[f.Short] = true
		}
	}

	for _, f := range c.Flags {
		if f.Name == "" {
			return fmt.Errorf("argvine: command %q declares a flag with an empty name", c.Name)
		}

		// EN: One message covers both duplicate and shadowing because the fix
		// is the same in either case: rename one of the two.
		// PT: Uma mensagem cobre duplicata e sombreamento porque a correção é a
		// mesma nos dois casos: renomear uma das duas.
		if seenName[f.Name] {
			return fmt.Errorf("argvine: command %q declares --%s twice, or shadows an inherited flag of the same name", c.Name, f.Name)
		}
		seenName[f.Name] = true

		if f.Short != "" {
			// EN: A multi-character Short would be indistinguishable from a
			// group of single-character flags once parsing starts: "-fo" HAS to
			// mean -f -o, so it can never also mean a flag named "fo".
			// PT: Um Short de vários caracteres seria indistinguível de um grupo
			// de flags de um caractere quando o parsing começa: "-fo" TEM que
			// significar -f -o, então nunca pode também significar uma flag
			// chamada "fo".
			if len(f.Short) != 1 {
				return fmt.Errorf("argvine: command %q flag --%s has short %q; short flags are exactly one character", c.Name, f.Name, f.Short)
			}
			if seenShort[f.Short] {
				return fmt.Errorf("argvine: command %q reuses short flag -%s", c.Name, f.Short)
			}
			seenShort[f.Short] = true
		}

		// EN: A bool flag has exactly two possible values, so restricting them
		// is either a no-op or a contradiction. Reaching for Choices here
		// almost always means the Type field was meant to be String.
		// PT: Uma flag booleana tem exatamente dois valores possíveis, então
		// restringi-los é no-op ou contradição. Usar Choices aqui quase sempre
		// significa que o campo Type deveria ser String.
		if f.Type == Bool && f.Choices != nil {
			return fmt.Errorf("argvine: command %q flag --%s is bool but declares Choices", c.Name, f.Name)
		}
		if err := checkDefault(c.Name, f); err != nil {
			return err
		}
	}

	for i, a := range c.Args {
		if a.Name == "" {
			return fmt.Errorf("argvine: command %q declares a positional with an empty name", c.Name)
		}

		// EN: Many absorbs every remaining token, so anything declared after it
		// could never be filled. And there is no rule that would make the shape
		// unambiguous anyway: with {tags: Many}, {name: One} and three tokens,
		// both "tags=[a] name=b" and "tags=[a b] name=c" are defensible.
		// Rejecting the declaration is cheaper than inventing a tie-breaker.
		// PT: Many absorve todo token restante, então qualquer coisa declarada
		// depois nunca seria preenchida. E não existe regra que torne a forma
		// não-ambígua: com {tags: Many}, {name: One} e três tokens, tanto
		// "tags=[a] name=b" quanto "tags=[a b] name=c" são defensáveis. Recusar
		// a declaração sai mais barato que inventar um critério de desempate.
		if a.Arity == Many && i != len(c.Args)-1 {
			return fmt.Errorf("argvine: command %q declares <%s> with Many arity but it is not the last positional", c.Name, a.Name)
		}
	}

	seenSub := make(map[string]bool, len(c.Sub))
	for _, s := range c.Sub {
		if s.Name == "" {
			return fmt.Errorf("argvine: command %q declares a subcommand with an empty name", c.Name)
		}

		// EN: Duplicates would make routing depend on declaration order, which
		// is invisible to the person typing the command.
		// PT: Duplicatas fariam o roteamento depender da ordem de declaração,
		// que é invisível para quem digita o comando.
		if seenSub[s.Name] {
			return fmt.Errorf("argvine: command %q declares subcommand %q twice", c.Name, s.Name)
		}
		seenSub[s.Name] = true
	}

	// EN: Build what the children inherit: whatever this node already inherited,
	// plus its own persistent flags.
	//
	// Copy rather than `next := inherited`. The naive version appends into the
	// caller's backing array whenever it has spare capacity, which makes this
	// function's correctness depend on how a caller several frames up built its
	// slice. It happens not to produce a wrong read in this particular walk —
	// each child receives its own length and never looks past it — but owning
	// the array is what keeps the reasoning local, and this runs once at startup.
	//
	// PT: Monta o que os filhos herdam: o que este nó já herdou, mais as
	// persistentes dele próprio.
	//
	// Copiar em vez de `next := inherited`. A versão ingênua escreve no array do
	// chamador sempre que houver capacidade sobrando, o que faz a correção desta
	// função depender de como um chamador vários frames acima construiu a slice
	// dele. Nesta varredura específica não produz leitura errada — cada filho
	// recebe seu próprio comprimento e nunca olha além dele — mas ser dono do
	// array é o que mantém o raciocínio local, e isto roda uma vez no boot.
	next := make([]Flag, len(inherited), len(inherited)+len(c.Flags))
	copy(next, inherited)
	for _, f := range c.Flags {
		if f.Persistent {
			next = append(next, f)
		}
	}

	// EN: Recursing last means a node's own problems are reported before its
	// children's, so the first error points at the outermost mistake.
	// PT: Recursar por último faz os problemas do próprio nó serem reportados
	// antes dos filhos, então o primeiro erro aponta para o engano mais externo.
	for _, s := range c.Sub {
		if err := s.validate(next); err != nil {
			return err
		}
	}
	return nil
}

// checkDefault verifies that a flag's Default matches its declared Type.
//
// EN — Default is `any`, so nothing stops {Type: Int, Default: "high"} from
// compiling. Left unchecked it would surface much later as a panic inside
// Context.Int, at a call site that has nothing to do with the declaration that
// caused it. Catching it here turns a confusing runtime panic into a startup
// message that names the command and the flag.
//
// A nil Default is legal: it means "no default", and Parse seeds the type's
// zero value instead.
//
// PT — Default é `any`, então nada impede {Type: Int, Default: "high"} de
// compilar. Sem checagem, isso apareceria muito depois como panic dentro do
// Context.Int, num ponto de chamada que não tem relação nenhuma com a
// declaração que o causou. Pegar aqui transforma um panic confuso em runtime
// numa mensagem de boot que nomeia o comando e a flag.
//
// Default nil é legal: significa "sem default", e o Parse semeia o valor zero
// do tipo no lugar.
func checkDefault(cmdName string, f Flag) error {
	if f.Default == nil {
		return nil
	}

	// EN: The comma-ok form of the type assertion is the whole check: the value
	// is discarded, only the assertion's success matters.
	// PT: A forma comma-ok da asserção de tipo é a checagem inteira: o valor é
	// descartado, só o sucesso da asserção importa.
	var ok bool
	switch f.Type {
	case Bool:
		_, ok = f.Default.(bool)
	case String:
		_, ok = f.Default.(string)
	case Int:
		_, ok = f.Default.(int)
	}
	if !ok {
		// EN: %s prints the declared type via FlagType.String, %T the actual
		// one: "is int but its Default is string" reads as the fix itself.
		// PT: %s imprime o tipo declarado via FlagType.String, %T o real: "is
		// int but its Default is string" já se lê como a própria correção.
		return fmt.Errorf("argvine: command %q flag --%s is %s but its Default is %T", cmdName, f.Name, f.Type, f.Default)
	}
	return nil
}
