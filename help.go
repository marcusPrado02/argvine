package argvine

import (
	"fmt"
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

// Help renders the help text for the command at the end of path.
//
// EN — It returns a string and never writes to any stream. Printing is the
// caller's decision, which is what makes this function testable by plain
// comparison instead of by capturing stdout.
//
// Every line is derived from the tree. There is no per-command template and no
// stored help text anywhere in the model — which is the whole reason a flag you
// add shows up here without anyone editing this file.
//
// Layout, in order:
//
//	<Long, or Short when Long is empty>
//
//	Usage: <usage line>
//
//	Commands:          only when the node has children
//	  <name>   <Short>
//
//	Flags:             only when the node declares any
//	  -f, --force            <Usage> (default: false)
//
//	Global Flags:      only when it inherits any
//	  -v, --verbose          <Usage> (default: false)
//
// Each section is skipped entirely when empty, so a leaf with no flags does not
// print a bare "Flags:" heading with nothing under it.
//
// PT — Devolve uma string e nunca escreve em stream nenhum. Imprimir é decisão
// de quem chama, e é isso que torna esta função testável por comparação simples
// em vez de por captura de stdout.
//
// Toda linha é derivada da árvore. Não existe template por comando nem texto de
// help guardado em lugar nenhum do modelo — que é a razão inteira de uma flag
// que você acrescenta aparecer aqui sem ninguém editar este arquivo.
//
// Layout, em ordem:
//
//	<Long, ou Short quando Long está vazio>
//
//	Usage: <linha de uso>
//
//	Commands:          só quando o nó tem filhos
//	  <nome>   <Short>
//
//	Flags:             só quando o nó declara alguma
//	  -f, --force            <Usage> (default: false)
//
//	Global Flags:      só quando ele herda alguma
//	  -v, --verbose          <Usage> (default: false)
//
// Cada seção é pulada por completo quando vazia, então uma folha sem flags não
// imprime um título "Flags:" sozinho sem nada embaixo.
func Help(path []*Command) string {
	// EN: Defensive, not expected: every caller builds the path from a real
	// walk. Returning "" beats panicking inside something whose only job is to
	// produce text for a human.
	// PT: Defensivo, não esperado: todo chamador monta o caminho a partir de uma
	// varredura real. Devolver "" é melhor que panicar dentro de algo cuja única
	// função é produzir texto para um humano.
	if len(path) == 0 {
		return ""
	}

	cmd := path[len(path)-1]

	// EN: A Builder rather than string concatenation — this appends many small
	// pieces, and += would copy the whole accumulated string every time.
	// PT: Um Builder em vez de concatenação — isto acrescenta muitos pedaços
	// pequenos, e += copiaria a string acumulada inteira a cada vez.
	var b strings.Builder

	// EN: Long wins when present, Short is the fallback. A command with neither
	// simply starts at the usage line; nothing is invented to fill the gap.
	// PT: Long ganha quando existe, Short é o recurso. Um comando sem nenhum dos
	// dois simplesmente começa na linha de uso; nada é inventado para preencher.
	switch {
	case cmd.Long != "":
		b.WriteString(cmd.Long + "\n\n")
	case cmd.Short != "":
		b.WriteString(cmd.Short + "\n\n")
	}

	b.WriteString("Usage: " + usageLine(path) + "\n")

	if len(cmd.Sub) > 0 {
		b.WriteString("\nCommands:\n")
		b.WriteString(renderCommands(cmd.Sub))
	}

	// EN: Two separate sections, never one merged list. The reader needs to
	// know which options belong to the command they typed and which come from
	// further up the tree — a single list would be accurate and misleading.
	// PT: Duas seções separadas, nunca uma lista mesclada. O leitor precisa
	// saber quais opções pertencem ao comando que ele digitou e quais vêm de
	// mais acima na árvore — uma lista só seria exata e enganosa.
	own, inherited := splitFlags(path)
	if len(own) > 0 {
		b.WriteString("\nFlags:\n")
		b.WriteString(renderFlags(own))
	}
	if len(inherited) > 0 {
		b.WriteString("\nGlobal Flags:\n")
		b.WriteString(renderFlags(inherited))
	}

	return b.String()
}

// renderCommands lists a node's children, one per line, name column aligned.
//
// EN — Two passes over the same slice, and that is unavoidable: aligned columns
// need the width of the longest entry, which is only known after looking at all
// of them. Measure, then draw. Doing it in one pass would mean going back and
// rewriting what was already emitted.
//
// The slice is copied before sorting. sort.Slice reorders in place, and subs is
// the caller's cmd.Sub — sorting it directly would silently reorder the user's
// declaration, changing nothing visible here and everything in the parser.
//
// PT — Duas passadas sobre a mesma slice, e é inevitável: colunas alinhadas
// precisam da largura da maior entrada, que só se conhece depois de olhar todas.
// Medir, depois desenhar. Fazer numa passada só exigiria voltar e reescrever o
// que já foi emitido.
//
// A slice é copiada antes de ordenar. O sort.Slice reordena no lugar, e subs é o
// cmd.Sub de quem chamou — ordenar direto reordenaria em silêncio a declaração
// do usuário, sem mudar nada visível aqui e mudando tudo no parser.
func renderCommands(subs []*Command) string {
	sorted := make([]*Command, len(subs))
	copy(sorted, subs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	// EN: Pass one — measure.
	// PT: Passada um — medir.
	width := 0
	for _, s := range sorted {
		if len(s.Name) > width {
			width = len(s.Name)
		}
	}

	// EN: Pass two — draw. Two leading spaces indent the block, three trailing
	// ones separate the name column from the description.
	// PT: Passada dois — desenhar. Dois espaços iniciais indentam o bloco, três
	// finais separam a coluna de nome da descrição.
	var b strings.Builder
	for _, s := range sorted {
		b.WriteString("  " + pad(s.Name, width) + "   " + s.Short + "\n")
	}
	return b.String()
}

// renderFlags lists flags, one per line, label column aligned.
//
// EN — Same measure-then-draw shape as renderCommands, but the label has to be
// built first because it is not a plain field:
//
//	-p, --priority int     short form present, non-bool so the type shows
//	    --due string       no short form, four spaces keep the column aligned
//	-v, --verbose          bool, so no type is printed
//
// The four spaces standing in for a missing "-x, " are what keep every "--name"
// starting at the same column. Without them the list would ragged-left and the
// eye would lose the column.
//
// The type is omitted for bools on purpose: "--verbose bool" tells the reader
// nothing they did not already infer from the flag taking no value.
//
// PT — Mesma forma medir-depois-desenhar do renderCommands, mas o rótulo precisa
// ser montado antes porque não é um campo simples:
//
//	-p, --priority int     forma curta presente, não-bool então o tipo aparece
//	    --due string       sem forma curta, quatro espaços mantêm a coluna
//	-v, --verbose          bool, então nenhum tipo é impresso
//
// Os quatro espaços no lugar de um "-x, " ausente são o que mantém todo "--nome"
// começando na mesma coluna. Sem eles a lista ficaria irregular à esquerda e o
// olho perderia a coluna.
//
// O tipo é omitido para booleanas de propósito: "--verbose bool" não diz ao
// leitor nada que ele já não tenha inferido de a flag não receber valor.
func renderFlags(flags []Flag) string {
	labels := make([]string, len(flags))
	width := 0

	// EN: Pass one — build each label and remember the widest.
	// PT: Passada um — monta cada rótulo e guarda o mais largo.
	for i, f := range flags {
		short := "    "
		if f.Short != "" {
			short = "-" + f.Short + ", "
		}

		label := short + "--" + f.Name
		if f.Type != Bool {
			label += " " + f.Type.String()
		}

		labels[i] = label
		if len(label) > width {
			width = len(label)
		}
	}

	// EN: Pass two — draw.
	// PT: Passada dois — desenhar.
	var b strings.Builder

	for i, f := range flags {
		b.WriteString("  " + pad(labels[i], width) + "   " + f.Usage)

		// EN: Required and Default are mutually exclusive in practice — a flag
		// the user must supply has no meaningful default — so a switch says
		// that better than two ifs would.
		// PT: Required e Default são mutuamente exclusivos na prática — uma flag
		// que o usuário tem que fornecer não tem default significativo — então um
		// switch diz isso melhor que dois ifs diriam.
		switch {
		case f.Required:
			b.WriteString(" (required)")
		case f.Default != nil:
			b.WriteString(" (default: " + formatDefault(f.Default) + ")")
		}

		if len(f.Choices) > 0 {
			b.WriteString(" [" + strings.Join(f.Choices, "|") + "]")
		}

		b.WriteString("\n")
	}
	return b.String()
}

// formatDefault renders a default value for display in help.
//
// EN — Strings are quoted, everything else is not. The reason is the empty
// string: with plain %v, a flag defaulting to "" renders as
//
//	--due string   due date (default: )
//
// which reads like the code forgot to print something. Quoting makes the value
// visible as a value:
//
//	--due string   due date (default: "")
//
// Quoting also disambiguates a default with leading or trailing spaces, which
// would otherwise be invisible.
//
// PT — Strings são citadas, o resto não. A razão é a string vazia: com %v puro,
// uma flag com default "" renderiza como
//
//	--due string   due date (default: )
//
// que se lê como se o código tivesse esquecido de imprimir algo. Citar torna o
// valor visível como valor:
//
//	--due string   due date (default: "")
//
// Citar também desambigua um default com espaços no início ou no fim, que de
// outro modo seriam invisíveis.
func formatDefault(v any) string {
	if s, ok := v.(string); ok {
		return fmt.Sprintf("%q", s)
	}
	return fmt.Sprint(v)
}

// pad right-pads s with spaces up to width.
//
// EN — len() counts BYTES, not runes, so a flag name with an accented character
// would misalign the column. Acceptable here because flag names are ASCII by
// convention — but it is the line to change if that ever stops being true.
//
// PT — len() conta BYTES, não runes, então um nome de flag com caractere
// acentuado desalinharia a coluna. Aceitável aqui porque nomes de flag são ASCII
// por convenção — mas é esta a linha a mudar se isso deixar de ser verdade.
func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
