package argvine

import (
	"fmt"
	"strconv"
	"strings"
)

// flagIndex is the set of flags visible at one point of the walk: the current
// command's own flags plus every persistent flag inherited from its ancestors.
//
// EN — It is rebuilt from scratch on every descent rather than accumulated.
// That is what makes a non-persistent parent flag stop being visible in a
// child: nothing has to be removed, because only what should be visible is ever
// added.
//
// PT — É reconstruído do zero a cada descida, em vez de acumulado. É isso que
// faz uma flag não-persistente do pai deixar de ser visível no filho: nada
// precisa ser removido, porque só o que deve ser visível chega a ser posto.
type flagIndex struct {
	byName  map[string]Flag
	byShort map[string]Flag
}

func newFlagIndex() flagIndex {
	return flagIndex{
		byName:  make(map[string]Flag),
		byShort: make(map[string]Flag),
	}
}

// add registers flags under both their long and short names.
//
// EN — The receiver is a value, not a pointer, and that is fine: the maps
// inside are reference types, so writes through a copy of the struct are
// visible to every other copy. Only reassigning a whole map field would need a
// pointer receiver.
//
// PT — O receiver é valor, não ponteiro, e tudo bem: os mapas dentro são tipos
// de referência, então escritas através de uma cópia da struct são visíveis em
// todas as outras cópias. Só reatribuir um campo de mapa inteiro exigiria
// receiver ponteiro.
func (x flagIndex) add(flags []Flag) {
	for _, f := range flags {
		x.byName[f.Name] = f

		// EN: Guarding on the empty string matters: without it every flag
		// without a short form would overwrite the same "" key, and each would
		// appear to have registered a short alias.
		// PT: A guarda contra string vazia importa: sem ela toda flag sem forma
		// curta sobrescreveria a mesma chave "", e cada uma pareceria ter
		// registrado um apelido curto.
		if f.Short != "" {
			x.byShort[f.Short] = f
		}
	}
}

// convert turns a raw command-line token into the flag's declared type. The
// second result is false when the token does not convert.
//
// EN — It reports a bool rather than an error because it cannot build a good
// one: the message a user should see names the command the flag appeared on,
// and convert has no idea where in the tree it is being called from. The caller
// does, so the caller constructs the ErrBadType.
//
// This is the mirror image of the rule that the function detecting a problem
// should describe it — here the detector genuinely lacks the context, so it
// reports the fact and stays out of the wording.
//
// PT — Devolve um bool em vez de um erro porque não consegue montar um bom: a
// mensagem que o usuário deve ver nomeia o comando em que a flag apareceu, e o
// convert não faz ideia de que ponto da árvore está sendo chamado. Quem chama
// sabe, então quem chama constrói o ErrBadType.
//
// É o espelho da regra de que quem detecta o problema deve descrevê-lo — aqui o
// detector genuinamente não tem o contexto, então reporta o fato e fica fora da
// redação.
func convert(f Flag, raw string) (any, bool) {
	switch f.Type {
	case String:
		return raw, true

	case Int:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, false
		}
		return n, true

	case Bool:
		// EN: ParseBool is deliberately liberal — 1, t, T, TRUE, true, and
		// their false counterparts — which matches what people expect from
		// --flag=1.
		// PT: ParseBool é deliberadamente liberal — 1, t, T, TRUE, true, e as
		// contrapartes falsas — o que combina com o que as pessoas esperam de
		// --flag=1.
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, false
		}
		return b, true

	default:
		// EN: Unreachable for any tree that passed Validate. Reaching it means
		// the tree was never validated, which is a programming error, so it
		// panics rather than returning an error a user could see.
		// PT: Inalcançável para qualquer árvore que passou pelo Validate.
		// Chegar aqui significa que a árvore nunca foi validada, o que é erro
		// de programação, então panica em vez de devolver um erro que o usuário
		// veria.
		panic(fmt.Sprintf("argvine: flag --%s has unknown type %d", f.Name, f.Type))
	}
}

// parser holds the mutable state of one walk over the tree.
//
// EN — It exists so the step helpers do not have to thread half a dozen
// arguments between themselves. Note that all of this state is per-call: it
// lives here, never in the Command tree, which is what keeps Parse a pure
// function of (tree, argv).
//
// PT — Existe para que os helpers de passo não tenham que passar meia dúzia de
// argumentos entre si. Note que todo esse estado é por chamada: mora aqui,
// nunca na árvore de Command, e é isso que mantém o Parse uma função pura de
// (árvore, argv).
type parser struct {
	ctx *Context

	// EN: cur is the deepest command reached so far.
	// PT: cur é o comando mais profundo alcançado até agora.
	cur *Command

	// EN: visible is the flag index for cur, rebuilt on every descent.
	// PT: visible é o índice de flags de cur, reconstruído a cada descida.
	visible flagIndex

	// EN: persistent accumulates every Persistent flag seen along the path,
	// root first. A descent seeds the new index from it.
	// PT: persistent acumula toda flag Persistent vista ao longo do caminho, da
	// raiz para baixo. Uma descida semeia o índice novo a partir dele.
	persistent []Flag

	// EN: declared accumulates every flag declared along the WALKED path. It is
	// the set that gets default values seeded at the end, and it deliberately
	// excludes flags belonging to branches the walk never entered.
	// PT: declared acumula toda flag declarada ao longo do caminho PERCORRIDO.
	// É o conjunto que recebe os defaults no fim, e exclui de propósito flags
	// de ramos em que a varredura nunca entrou.
	declared []Flag

	// EN: seen records which flags actually appeared in argv, so seedDefaults
	// can tell "absent" from "explicitly set to the zero value".
	// PT: seen registra quais flags de fato apareceram em argv, para que o
	// seedDefaults distinga "ausente" de "explicitamente posto no valor zero".
	seen map[string]bool

	// EN: positional collects tokens that are neither flags nor subcommands.
	// PT: positional junta tokens que não são flags nem subcomandos.
	positional []string

	argv []string

	// EN: i is the index of the next token to read. Helpers advance it by
	// however much they consumed, which is how a valued flag skips its value.
	// PT: i é o índice do próximo token a ler. Os helpers o avançam conforme o
	// que consumiram, e é assim que uma flag com valor pula o próprio valor.
	i int

	// EN: afterTerminator latches on at the first "--" and never resets: from
	// there to the end of argv, every token is a positional whatever its shape.
	// PT: afterTerminator trava no primeiro "--" e nunca reseta: dali até o fim
	// do argv, todo token é posicional, qualquer que seja sua forma.
	afterTerminator bool
}

// Parse walks the tree consuming argv left to right and returns a fresh
// Context.
//
// EN — Step by step:
//
//  1. Set up the walk at the root: visible flags, declared flags, persistent
//     flags, and Path.
//  2. Loop one token at a time; step classifies each and says how far to
//     advance. A token can be a flag, a subcommand, or a positional.
//  3. After the last token: seed defaults, then validate, then bind
//     positionals. None of those three can run earlier — see the block below.
//
// It never prints and never terminates the process: on bad input it returns an
// error and lets the caller decide what to show and which exit code to use.
// A new Context per call is what allows two parses over the same tree to be
// independent, and what makes the table-driven tests possible.
//
// PT — Passo a passo:
//
//  1. Prepara a varredura na raiz: flags visíveis, flags declaradas, flags
//     persistentes e o Path.
//  2. Laço de um token por vez; o step classifica cada um e diz quanto
//     avançar. Um token pode ser flag, subcomando ou posicional.
//  3. Depois do último token: semeia defaults, depois valida, depois liga os
//     posicionais. Nenhum dos três pode rodar antes — ver o bloco abaixo.
//
// Nunca imprime e nunca encerra o processo: com entrada ruim devolve um erro e
// deixa quem chama decidir o que mostrar e qual código de saída usar. Um
// Context novo por chamada é o que permite dois parses sobre a mesma árvore
// serem independentes, e o que torna os testes de tabela possíveis.
func Parse(root *Command, argv []string) (*Context, error) {
	p := &parser{
		ctx:     newContext(),
		cur:     root,
		visible: newFlagIndex(),
		seen:    make(map[string]bool),
		argv:    argv,
	}

	p.ctx.Path = []*Command{root}
	p.visible.add(root.Flags)
	p.declared = append(p.declared, root.Flags...)
	p.collectPersistent(root)

	// EN: One token at a time. step decides what the token is and how far to
	// advance, so the loop itself has no idea what a flag looks like.
	// PT: Um token por vez. O step decide o que o token é e quanto avançar, de
	// modo que o laço em si não faz ideia de como uma flag se parece.
	for p.i < len(p.argv) {
		if err := p.step(); err != nil {
			return nil, err
		}
	}

	p.ctx.Cmd = p.cur
	p.ctx.rawPositional = p.positional

	// EN: All three finishing steps run only after the last token is read, and
	// that is forced by the question each one answers. "This flag was never
	// given" and "this argument was never filled" are only true statements once
	// there is no more argv left to contradict them.
	//
	// The order between them is forced too, not stylistic:
	//
	//	seedDefaults   → every declared flag now has a value
	//	validateFlags  → Required and Choices, now that absence is knowable
	//	bindArgs       → positionals, now that flags are settled
	//
	// Swapping the first two would make Required complain about flags that have
	// a default. Swapping the last two would report a missing argument when the
	// real problem was a missing required flag — the wrong diagnosis for the
	// same command line.
	//
	// PT: Os três passos finais só rodam depois de o último token ser lido, e
	// isso é forçado pela pergunta que cada um responde. "Esta flag nunca foi
	// dada" e "este argumento nunca foi preenchido" só viram afirmações
	// verdadeiras quando não há mais argv para contradizê-las.
	//
	// A ordem entre eles também é forçada, não estilística:
	//
	//	seedDefaults   → toda flag declarada agora tem valor
	//	validateFlags  → Required e Choices, agora que ausência é sabível
	//	bindArgs       → posicionais, agora que as flags estão resolvidas
	//
	// Inverter os dois primeiros faria o Required reclamar de flags que têm
	// default. Inverter os dois últimos reportaria argumento faltando quando o
	// problema real era flag obrigatória ausente — diagnóstico errado para a
	// mesma linha de comando.
	p.seedDefaults()

	if err := p.validateFlags(); err != nil {
		return nil, err
	}

	if err := p.bindArgs(); err != nil {
		return nil, err
	}

	return p.ctx, nil
}

// step classifies the token at p.i and dispatches to the right handler.
//
// EN — The order of the cases IS the specification of the syntax, and it is not
// arbitrary:
//
//  1. afterTerminator first — it disables every pattern below it.
//  2. tok == "--" before the "--" prefix, or it would parse as a long flag
//     with an empty name.
//  3. "--" prefix  → long flag.
//  4. "-" prefix, length > 1 → short group. The length guard keeps a lone "-"
//     out, because that is the conventional name for stdin, not a flag.
//  5. Everything else → a subcommand if one matches and no positional has
//     been seen yet; otherwise a positional.
//
// Only two pieces of state make position meaningful — afterTerminator and "no
// positional seen yet". Everything else is classified independently of what
// came before, which is exactly why flags and positionals can be interleaved
// freely and still produce the same result.
//
// PT — A ordem dos casos É a especificação da sintaxe, e não é arbitrária:
//
//  1. afterTerminator primeiro — ele desliga todos os padrões abaixo.
//  2. tok == "--" antes do prefixo "--", senão seria lido como flag longa de
//     nome vazio.
//  3. prefixo "--"  → flag longa.
//  4. prefixo "-" com tamanho > 1 → grupo curto. A guarda de tamanho mantém um
//     "-" sozinho fora, porque esse é o nome convencional de stdin, não flag.
//  5. Todo o resto → subcomando se algum casar e nenhum posicional tiver
//     aparecido ainda; caso contrário, posicional.
//
// Só duas memórias fazem a posição importar — afterTerminator e "nenhum
// posicional visto ainda". Todo o resto é classificado independentemente do que
// veio antes, e é exatamente por isso que flags e posicionais podem se
// intercalar livremente e ainda produzir o mesmo resultado.
func (p *parser) step() error {
	tok := p.argv[p.i]

	switch {
	case p.afterTerminator:
		p.positional = append(p.positional, tok)
		p.i++
		return nil

	case tok == "--":
		p.afterTerminator = true
		p.i++
		return nil

	case strings.HasPrefix(tok, "--"):
		return p.longFlag()

	// EN: A lone "-" is the conventional name for stdin, not a flag, so the
	// length guard keeps it out of shortGroup and lets it fall through.
	// PT: Um "-" sozinho é o nome convencional de stdin, não uma flag, então a
	// guarda de tamanho o mantém fora do shortGroup e o deixa cair adiante.
	case len(tok) > 1 && strings.HasPrefix(tok, "-"):
		return p.shortGroup()

	default:
		// EN: A subcommand only wins while no positional has been seen. Once
		// the user has started supplying arguments, a token that happens to
		// match a child's name is just another argument — "task add remote"
		// adds a task titled "remote".
		// PT: Um subcomando só vence enquanto nenhum posicional tiver
		// aparecido. Depois que o usuário começou a fornecer argumentos, um
		// token que por acaso casa com o nome de um filho é só mais um
		// argumento — "task add remote" adiciona uma tarefa chamada "remote".
		if sub := p.cur.findSub(tok); sub != nil && len(p.positional) == 0 {
			p.descend(sub)
			return nil
		}
		p.positional = append(p.positional, tok)
		p.i++
		return nil
	}
}

// longFlag parses a token beginning with "--", in either the "--name value" or
// the "--name=value" form.
//
// EN — Step by step:
//
//  1. Split the token on the first "=" to separate name from inline value.
//  2. Look the name up in the visible index; unknown means a usage error.
//  3. If the flag is Bool and there was no "=", record true and consume ONE
//     token — a bool never eats the token after it.
//  4. Otherwise take the value: from after the "=" (one token consumed), or
//     from the next token (two consumed).
//  5. Convert the raw string to the declared type and record it.
//
// PT — Passo a passo:
//
//  1. Divide o token no primeiro "=" para separar nome de valor embutido.
//  2. Procura o nome no índice de visíveis; desconhecido é erro de uso.
//  3. Se a flag é Bool e não havia "=", grava true e consome UM token — uma
//     booleana nunca come o token seguinte.
//  4. Caso contrário pega o valor: depois do "=" (um token consumido), ou do
//     token seguinte (dois consumidos).
//  5. Converte a string crua para o tipo declarado e grava.
func (p *parser) longFlag() error {
	tok := p.argv[p.i]

	// EN: Cut splits on the first "=" only, so --message=a=b keeps "a=b".
	// PT: O Cut divide só no primeiro "=", então --message=a=b mantém "a=b".
	name, inline, hasInline := strings.Cut(tok[2:], "=")

	f, ok := p.visible.byName[name]
	if !ok {
		return &ErrUnknownFlag{p.at(), "--" + name}
	}

	// EN: The declared type, never the next token, decides consumption. A parser
	// that guessed by looking ahead ("starts with a dash, so not a value")
	// would break on --offset -3 and on --message -v2-fix. The information
	// needed is not in the token; it is in the declaration, and it was known
	// before parsing began. This single line is the heart of the milestone.
	// PT: O tipo declarado, nunca o token seguinte, decide o consumo. Um parser
	// que chutasse espiando à frente ("começa com hífen, logo não é valor")
	// quebraria em --offset -3 e em --message -v2-fix. A informação necessária
	// não está no token; está na declaração, e já era conhecida antes de o
	// parsing começar. Esta única linha é o coração do marco.
	if f.Type == Bool && !hasInline {
		p.set(f, true)
		p.i++
		return nil
	}

	raw, consumed := inline, 1
	if !hasInline {
		if p.i+1 >= len(p.argv) {
			return &ErrMissingValue{p.at(), f}
		}
		// EN: Two tokens consumed: the flag and its value.
		// PT: Dois tokens consumidos: a flag e o valor dela.
		raw, consumed = p.argv[p.i+1], 2
	}

	v, ok := convert(f, raw)
	if !ok {
		return &ErrBadType{p.at(), f, raw}
	}
	p.set(f, v)
	p.i += consumed
	return nil
}

// set records a parsed value and marks the flag as having appeared.
//
// EN — Both writes happen here so they can never drift apart: a value recorded
// without its seen entry would be silently overwritten by seedDefaults.
//
// PT — As duas escritas acontecem aqui para que nunca se descolem: um valor
// gravado sem a entrada em seen seria sobrescrito em silêncio pelo seedDefaults.
func (p *parser) set(f Flag, v any) {
	p.ctx.flags[f.Name] = v
	p.seen[f.Name] = true
}

// collectPersistent appends a command's persistent flags to the running set
// that descendants will inherit.
//
// EN — Called once per node as the walk descends, so the set only ever grows
// along the path actually taken.
//
// PT — Chamado uma vez por nó conforme a varredura desce, então o conjunto só
// cresce ao longo do caminho efetivamente percorrido.
func (p *parser) collectPersistent(c *Command) {
	for _, f := range c.Flags {
		if f.Persistent {
			p.persistent = append(p.persistent, f)
		}
	}
}

// seedDefaults fills in every flag declared along the walked path that did not
// appear in argv.
//
// EN — Why it cannot run during the walk: at token 2 of 6 you do not yet know
// whether --port shows up at token 5. Writing the default early and overwriting
// it later would work, but then "--port absent" and "--port 8080 typed" become
// indistinguishable — and that distinction is what a future config-precedence
// layer would need.
//
// This is also why the typed accessors can panic on a missing key without being
// hostile: after Parse, every declared flag has an entry, so a missing key can
// only mean a name that was never declared — a typo, not an absent flag.
//
// A nil Default is not an error; it means the flag falls back to its type's
// zero value, which keeps trivial declarations short.
//
// PT — Por que não pode rodar durante a varredura: no token 2 de 6 você ainda
// não sabe se --port aparece no token 5. Escrever o default cedo e sobrescrever
// depois funcionaria, mas aí "--port ausente" e "--port 8080 digitado" ficam
// indistinguíveis — e é justamente essa distinção que uma futura camada de
// precedência de configuração precisaria.
//
// É também por isso que os acessores tipados podem panicar em chave faltando
// sem serem hostis: depois do Parse, toda flag declarada tem entrada, então
// chave faltando só pode ser nome que nunca foi declarado — um typo, não uma
// flag ausente.
//
// Default nil não é erro; significa que a flag recai no valor zero do tipo, o
// que mantém declarações triviais curtas.
func (p *parser) seedDefaults() {
	for _, f := range p.declared {
		if p.seen[f.Name] {
			continue
		}
		if f.Default != nil {
			p.ctx.flags[f.Name] = f.Default
			continue
		}
		switch f.Type {
		case Bool:
			p.ctx.flags[f.Name] = false
		case String:
			p.ctx.flags[f.Name] = ""
		case Int:
			p.ctx.flags[f.Name] = 0
		}
	}
}

// shortGroup parses a token like "-abc", where every character is a short flag.
//
// EN — The rule that makes the group unambiguous: bool flags chain freely, and
// the first non-bool flag ENDS the group and takes its value — from whatever is
// left in the token, or from the next token when nothing is left. Same
// convention as "tar -czf archive.tar.gz", where c and z are bools and f is
// valued.
//
//	-abc     → a, b, c, all bool
//	-ap 1    → a bool; p ends the group and takes the next token
//	-ap1     → a bool; p ends the group and takes "1", left in the token
//	-p -2    → p ends the group and takes -2, never classified as a flag
//
// The user never has to know any flag's type. The order they type resolves it,
// and the only way to write something ambiguous is to write something that was
// already an error.
//
// PT — A regra que torna o grupo não-ambíguo: flags booleanas encadeiam
// livremente, e a primeira não-booleana ENCERRA o grupo e pega seu valor — do
// que sobrou no token, ou do token seguinte quando nada sobrou. Mesma convenção
// de "tar -czf arquivo.tar.gz", onde c e z são booleanas e f tem valor.
//
//	-abc     → a, b, c, todas booleanas
//	-ap 1    → a booleana; p encerra o grupo e pega o token seguinte
//	-ap1     → a booleana; p encerra o grupo e pega "1", que sobrou no token
//	-p -2    → p encerra o grupo e pega -2, nunca classificado como flag
//
// O usuário nunca precisa saber o tipo de nenhuma flag. A ordem em que ele
// digita resolve, e a única forma de escrever algo ambíguo é escrever algo que
// já era erro.
func (p *parser) shortGroup() error {
	chars := p.argv[p.i][1:]

	for j := 0; j < len(chars); j++ {
		ch := string(chars[j])

		f, ok := p.visible.byShort[ch]
		if !ok {
			return &ErrUnknownFlag{p.at(), "-" + ch}
		}

		// EN: Bools do not end the group: keep walking the characters.
		// PT: Booleanas não encerram o grupo: segue percorrendo os caracteres.
		if f.Type == Bool {
			p.set(f, true)
			continue
		}

		// EN: Value glued to the flag: "-p1" or "-p=1". chars[j+1:] is safe at
		// the last index — it yields "", and TrimPrefix leaves it "".
		// PT: Valor colado na flag: "-p1" ou "-p=1". chars[j+1:] é seguro no
		// último índice — devolve "", e o TrimPrefix deixa "".
		if rest := strings.TrimPrefix(chars[j+1:], "="); rest != "" {
			v, ok := convert(f, rest)
			if !ok {
				return &ErrBadType{p.at(), f, rest}
			}
			p.set(f, v)
			p.i++
			return nil
		}

		// EN: Nothing left in the token, so the value is the next one. Reaching
		// here with "-p -2" consumes "-2" as the value without ever classifying
		// it, which is why a negative number is never mistaken for a flag.
		// PT: Nada sobrou no token, então o valor é o próximo. Chegar aqui com
		// "-p -2" consome "-2" como valor sem nunca classificá-lo, e é por isso
		// que um número negativo nunca é confundido com flag.
		if p.i+1 >= len(p.argv) {
			return &ErrMissingValue{p.at(), f}
		}
		v, ok := convert(f, p.argv[p.i+1])
		if !ok {
			return &ErrBadType{p.at(), f, p.argv[p.i+1]}
		}

		p.set(f, v)
		p.i += 2
		return nil
	}

	// EN: Only reached when every character was a bool — the group consumed
	// exactly one token.
	// PT: Só alcançado quando todo caractere era booleano — o grupo consumiu
	// exatamente um token.
	p.i++
	return nil
}

// descend moves the walk into a child command.
//
// EN — The visible flag index is thrown away and rebuilt from scratch:
// inherited persistent flags first, then the child's own. Rebuilding rather
// than adding is what makes a non-persistent parent flag stop being visible
// here — nothing has to be removed, because only what should be visible is ever
// put in.
//
// In tree code, rebuilding is usually easier to reason about than removing: you
// do not have to know what to take out, only what to put in.
//
// PT — O índice de flags visíveis é jogado fora e reconstruído do zero:
// primeiro as persistentes herdadas, depois as do próprio filho. Reconstruir em
// vez de somar é o que faz uma flag não-persistente do pai deixar de ser visível
// aqui — nada precisa ser removido, porque só o que deve ser visível chega a ser
// posto.
//
// Em código de árvore, reconstruir costuma ser mais fácil de raciocinar do que
// remover: você não precisa saber o que tirar, só o que colocar.
func (p *parser) descend(sub *Command) {
	p.cur = sub
	p.ctx.Path = append(p.ctx.Path, sub)

	p.visible = newFlagIndex()
	p.visible.add(p.persistent)
	p.visible.add(sub.Flags)

	// EN: declared grows along the walked path only, so seedDefaults never
	// seeds a flag from a branch this argv did not enter.
	// PT: declared cresce só ao longo do caminho percorrido, então o
	// seedDefaults nunca semeia flag de um ramo em que este argv não entrou.
	p.declared = append(p.declared, sub.Flags...)
	p.collectPersistent(sub)
	p.i++
}

// bindArgs assigns the collected positional tokens to the current command's
// declared Args, honoring each Arg's arity.
//
// EN — The algorithm is "greedy from the left": each Arg takes the minimum its
// arity demands off the front of what is left, and Many — which Validate forces
// to be the last Arg — takes everything that remains.
//
//  1. rest starts as every positional token collected during the walk.
//  2. Walk the declared Args in declaration order.
//  3. One       → must take exactly one; an empty rest is a usage error.
//     ZeroOrOne → takes one if there is one, otherwise binds nil and moves on.
//     Many      → takes all of rest; an empty rest is a usage error, because
//     Many means "one or more", not "zero or more".
//  4. Every Arg that took something shortens rest, so the next Arg sees only
//     what is still unclaimed.
//
// Worked example — "mixed release urgent backend" with Args [name:One, tags:Many]
//
//	rest = [release urgent backend]
//	name takes "release"        → rest = [urgent backend]
//	tags takes everything left  → rest = []
//
// Every declared Arg ends up with an entry even when nothing filled it — a nil
// slice, never a missing key. That is what lets Context.Arg distinguish "this
// argument is declared and absent" (returns "") from "this name was never
// declared" (panics): after Parse, a missing key can only be a typo.
//
// This is also why Validate rejects Many anywhere but last. With
// [tags:Many, name:One] and three tokens, both "tags=[a] name=b" and
// "tags=[a b] name=c" are defensible readings, and no obvious rule breaks the
// tie. Rejecting the declaration is cheaper than inventing one.
//
// PT — O algoritmo é "guloso da esquerda": cada Arg pega o mínimo que sua
// aridade exige da frente do que sobrou, e Many — que o Validate obriga a ser o
// último Arg — leva todo o resto.
//
//  1. rest começa com todos os tokens posicionais coletados na varredura.
//  2. Percorre os Args declarados, na ordem de declaração.
//  3. One       → tem que pegar exatamente um; rest vazio é erro de uso.
//     ZeroOrOne → pega um se houver, senão liga nil e segue adiante.
//     Many      → pega todo o rest; rest vazio é erro de uso, porque Many
//     significa "um ou mais", não "zero ou mais".
//  4. Todo Arg que pegou algo encurta rest, então o Arg seguinte só enxerga o
//     que ainda não foi reivindicado.
//
// Exemplo — "mixed release urgent backend" com Args [name:One, tags:Many]
//
//	rest = [release urgent backend]
//	name pega "release"        → rest = [urgent backend]
//	tags leva o que sobrou     → rest = []
//
// Todo Arg declarado termina com uma entrada mesmo que nada o preencha — slice
// nil, nunca chave ausente. É isso que permite ao Context.Arg distinguir "este
// argumento é declarado e está ausente" (devolve "") de "este nome nunca foi
// declarado" (panica): depois do Parse, chave faltando só pode ser um typo.
//
// É também por isso que o Validate recusa Many fora da última posição. Com
// [tags:Many, name:One] e três tokens, tanto "tags=[a] name=b" quanto
// "tags=[a b] name=c" são leituras defensáveis, e não há critério óbvio de
// desempate. Recusar a declaração sai mais barato que inventar um.
func (p *parser) bindArgs() error {
	// EN: rest is the unclaimed remainder; every branch below shortens it.
	// PT: rest é o restante não reivindicado; todo ramo abaixo o encurta.
	rest := p.positional

	for _, a := range p.cur.Args {
		switch a.Arity {
		case One:
			// EN: Mandatory. Nothing left means the user omitted it.
			// PT: Obrigatório. Nada sobrando significa que o usuário omitiu.
			if len(rest) == 0 {
				return &ErrMissingArg{p.at(), a}
			}
			p.ctx.args[a.Name] = []string{rest[0]}
			rest = rest[1:]

		case ZeroOrOne:
			// EN: Absence is a legitimate outcome, so bind nil instead of
			// erroring. Context.Arg turns that into "" for the handler.
			// PT: Ausência é resultado legítimo, então liga nil em vez de dar
			// erro. O Context.Arg transforma isso em "" para o handler.
			if len(rest) == 0 {
				p.ctx.args[a.Name] = nil
				continue
			}
			p.ctx.args[a.Name] = []string{rest[0]}
			rest = rest[1:]

		case Many:
			// EN: One or more, never zero — so an empty rest is an error here,
			// unlike ZeroOrOne just above.
			// PT: Um ou mais, nunca zero — então rest vazio é erro aqui, ao
			// contrário do ZeroOrOne logo acima.
			if len(rest) == 0 {
				return &ErrMissingArg{p.at(), a}
			}
			p.ctx.args[a.Name] = rest
			rest = nil
		}
	}

	// EN: Tokens left in rest are still unhandled. That is the next task, and
	// the error it raises depends on whether this node has subcommands.
	// PT: Tokens que sobraram em rest ainda não são tratados. Isso é a próxima
	// tarefa, e o erro depende de este nó ter ou não subcomandos.
	return nil
}

// at captures where the walk currently is, for embedding in a usage error.
//
// EN — It reads Path rather than cur because an error should name the whole
// invocation — "task remote add", not "add". Calling it at the moment the
// problem is detected is what keeps the snapshot accurate: a descent later in
// the same parse would otherwise change what the error reports.
//
// PT — Lê Path em vez de cur porque um erro deve nomear a invocação inteira —
// "task remote add", não "add". Chamar no momento em que o problema é detectado
// é o que mantém o retrato correto: uma descida mais adiante no mesmo parse
// mudaria o que o erro reporta.
func (p *parser) at() usageError {
	return usageError{Path: p.ctx.Path}
}

// validateFlags enforces Required and Choices over the flags declared along the
// walked path.
//
// EN — It runs after the walk, and it has to: "this flag was never given" is
// not a true statement until the last token has been read. It also runs after
// seedDefaults, so every declared flag already has a value to look at.
//
// The scope is p.declared, not the whole tree — a Required flag on a branch
// this argv never entered is nobody's problem.
//
// PT — Roda depois da varredura, e tem que ser assim: "esta flag nunca foi
// dada" não é afirmação verdadeira até o último token ser lido. Roda também
// depois do seedDefaults, então toda flag declarada já tem um valor para olhar.
//
// O escopo é p.declared, não a árvore toda — uma flag obrigatória num ramo em
// que este argv nunca entrou não é problema de ninguém.
func (p *parser) validateFlags() error {
	for _, f := range p.declared {
		if f.Required && !p.seen[f.Name] {
			return &ErrMissingRequired{p.at(), f}
		}

		// EN: Only what the user actually typed is checked. A default outside
		// its own Choices is the CLI author's mistake, caught by Validate at
		// startup; rejecting it here would blame the user for it.
		// PT: Só o que o usuário de fato digitou é checado. Um default fora do
		// próprio Choices é erro do autor da CLI, pego pelo Validate no boot;
		// recusar aqui culparia o usuário por ele.
		if f.Choices == nil || !p.seen[f.Name] {
			continue
		}

		// EN: Sprint renders int and string alike, so one comparison covers
		// every type that can carry Choices. Rare case where going through a
		// string is the right call rather than a smell: Choices is declared as
		// []string by the CLI author, so the domain of the comparison is
		// already textual. A switch on f.Type would need three branches to
		// reproduce this line, and each would be one more place to forget when
		// a fourth type is added.
		// PT: O Sprint renderiza int e string do mesmo jeito, então uma
		// comparação cobre todo tipo que pode carregar Choices. Caso raro em
		// que passar por string é a escolha certa e não um cheiro: Choices é
		// declarado como []string pelo autor da CLI, então o domínio da
		// comparação já é textual. Um switch em f.Type precisaria de três ramos
		// para reproduzir esta linha, e cada um seria mais um lugar para
		// esquecer quando um quarto tipo entrar.
		got := fmt.Sprint(p.ctx.flags[f.Name])
		if !containsString(f.Choices, got) {
			return &ErrBadChoice{p.at(), f, got}
		}
	}
	return nil
}

// containsString reports whether needle is in haystack.
//
// EN — A linear scan because a Choices set is a handful of values written by
// hand. Building a map per flag per parse would cost more than it saves, and
// would be one more structure to keep in sync with the declaration.
//
// PT — Varredura linear porque um conjunto Choices é um punhado de valores
// escritos à mão. Construir um mapa por flag por parse custaria mais do que
// economiza, e seria mais uma estrutura para manter em sincronia com a
// declaração.
func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
