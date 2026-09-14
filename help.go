package argvine

import (
	"sort"
	"strings"
)

// usageLine renders the "Usage:" line for the command at the end of path.
//
// EN — Step by step:
//
//  1. Join every command name along the path, so the line shows how to
//     actually invoke this command: "task remote add", never a bare "add".
//  2. Append "<command>" when the node has children — something must follow.
//  3. Append "[flags]" when anything is accepted, own or inherited.
//  4. Append one placeholder per declared Arg, shaped by its arity:
//     <name> required, [name] optional, <name>... one or more.
//
// The result for a nested leaf looks like:
//
//	task remote add [flags] <name> [url]
//
// Note this takes the PATH, not a single *Command. With only the node there is
// no way to render the invocation prefix, and no way to reach the ancestors
// that hold the persistent flags. The alternative — a Parent pointer on
// Command — would make the tree cyclic and stop it from being one declarative
// literal, so the path travels as a slice instead.
//
// PT — Passo a passo:
//
//  1. Junta o nome de cada comando do caminho, para que a linha mostre como
//     de fato invocar este comando: "task remote add", nunca um "add" solto.
//  2. Acrescenta "<command>" quando o nó tem filhos — algo precisa vir depois.
//  3. Acrescenta "[flags]" quando alguma é aceita, própria ou herdada.
//  4. Acrescenta um marcador por Arg declarado, com a forma da aridade:
//     <name> obrigatório, [name] opcional, <name>... um ou mais.
//
// O resultado para uma folha aninhada fica:
//
//	task remote add [flags] <name> [url]
//
// Note que recebe o CAMINHO, não um único *Command. Só com o nó não há como
// renderizar o prefixo de invocação, nem como alcançar os ancestrais que
// guardam as flags persistentes. A alternativa — um ponteiro Parent no Command
// — tornaria a árvore cíclica e a impediria de ser um literal declarativo, então
// o caminho viaja como slice.
func usageLine(path []*Command) string {
	cmd := path[len(path)-1]

	names := make([]string, len(path))
	for i, c := range path {
		names[i] = c.Name
	}

	line := strings.Join(names, " ")

	// EN: Every segment carries its own leading space. Building the line this
	// way keeps each branch independent — no branch has to know whether another
	// one already ran.
	// PT: Cada segmento carrega o próprio espaço inicial. Montar a linha assim
	// mantém cada ramo independente — nenhum precisa saber se outro já rodou.
	if len(cmd.Sub) > 0 {
		line += " <command>"
	}

	// EN: Inherited flags count too: a node with no flags of its own still
	// accepts the persistent ones from above, and the line must say so.
	// PT: Flags herdadas contam também: um nó sem flags próprias ainda aceita as
	// persistentes de cima, e a linha precisa dizer isso.
	own, inherited := splitFlags(path)
	if len(own)+len(inherited) > 0 {
		line += " [flags]"
	}

	// EN: The brackets are the convention every CLI user already reads: angle
	// brackets mean required, square brackets mean optional, the ellipsis means
	// repeatable. The shape comes straight from Arity — nothing is written by
	// hand per command.
	// PT: Os sinais são a convenção que todo usuário de CLI já lê: menor/maior
	// significa obrigatório, colchetes significam opcional, as reticências
	// significam repetível. A forma vem direto da Arity — nada é escrito à mão
	// por comando.
	for _, a := range cmd.Args {
		switch a.Arity {
		case One:
			line += " <" + a.Name + ">"
		case ZeroOrOne:
			line += " [" + a.Name + "]"
		case Many:
			line += " <" + a.Name + ">..."
		}
	}

	return line
}

// splitFlags separates the flags declared on the last command of path from the
// persistent flags it inherits from its ancestors.
//
// EN — The two groups are rendered under different headings ("Flags" and
// "Global Flags"), because the user needs to know which options belong to the
// command they typed and which come from further up. A single merged list would
// be accurate and misleading.
//
// Only the ancestors' PERSISTENT flags cross over: a non-persistent flag on a
// parent is invisible here, exactly as the parser treats it. Help and parsing
// read the same tree by the same rule, which is what stops them from drifting.
//
// Both lists are sorted by name so generated output is byte-stable across runs.
// Without that, golden-file tests would fail at random and quickly be ignored.
//
// PT — Os dois grupos são renderizados sob títulos diferentes ("Flags" e
// "Global Flags"), porque o usuário precisa saber quais opções pertencem ao
// comando que ele digitou e quais vêm de mais acima. Uma lista única e mesclada
// seria exata e enganosa.
//
// Só as PERSISTENTES dos ancestrais atravessam: uma flag não-persistente no pai
// é invisível aqui, exatamente como o parser a trata. Help e parsing leem a
// mesma árvore pela mesma regra, e é isso que impede que se descolem.
//
// As duas listas são ordenadas por nome para que a saída gerada seja estável
// byte a byte entre execuções. Sem isso, testes de golden file falhariam ao
// acaso e logo seriam ignorados.
func splitFlags(path []*Command) (own, inherited []Flag) {
	cmd := path[len(path)-1]

	// EN: append to a nil slice allocates a new array, so own never aliases
	// cmd.Flags — sorting it below cannot reorder the declaration itself.
	// PT: append numa slice nil aloca um array novo, então own nunca faz alias
	// de cmd.Flags — ordenar abaixo não reordena a declaração em si.
	own = append(own, cmd.Flags...)

	// EN: path[:len(path)-1] is every ancestor, excluding the node itself.
	// PT: path[:len(path)-1] são todos os ancestrais, excluindo o próprio nó.
	for _, ancestor := range path[:len(path)-1] {
		for _, f := range ancestor.Flags {
			if f.Persistent {
				inherited = append(inherited, f)
			}
		}
	}

	sortByName(own)
	sortByName(inherited)
	return own, inherited
}

// sortByName orders flags alphabetically, in place.
//
// EN — Alphabetical rather than declaration order, and that is a decision about
// the reader, not about tidiness: declaration order reflects how the CLI author
// happened to type the literal, while a user scanning help looks things up by
// name.
//
// PT — Alfabética em vez de ordem de declaração, e isso é uma decisão sobre o
// leitor, não sobre arrumação: a ordem de declaração reflete como o autor da CLI
// por acaso digitou o literal, enquanto um usuário lendo o help procura por nome.
func sortByName(flags []Flag) {
	sort.Slice(flags, func(i, j int) bool {
		return flags[i].Name < flags[j].Name
	})
}
