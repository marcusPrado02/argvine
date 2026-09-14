package argvine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBashScriptStructure checks the script's skeleton: the function is
// defined, registered, and has an arm for every level of the tree.
//
// EN — Substring checks are the right tool HERE, unlike for help, because what
// matters is that these pieces exist at all — not how they are laid out. The
// three quoted keys are the interesting ones: "task", "task remote" and "task
// remote add" prove the walk reached depth two and built the full path as the
// case pattern, not just the leaf name.
//
// PT — Checagem por substring é a ferramenta certa AQUI, ao contrário do help,
// porque o que importa é que estas peças existam — não como estão dispostas. As
// três chaves entre aspas são as interessantes: "task", "task remote" e "task
// remote add" provam que a travessia alcançou profundidade dois e montou o
// caminho completo como padrão do case, não só o nome da folha.
func TestBashScriptStructure(t *testing.T) {
	script := Bash(helpTree())

	wants := []string{
		"_task_completions()",
		"complete -F _task_completions task",
		`"task")`,
		`"task remote")`,
		`"task remote add")`,
		"compgen -W",
	}
	for _, w := range wants {
		if !strings.Contains(script, w) {
			t.Errorf("script should contain %q; got:\n%s", w, script)
		}
	}
}

// TestBashScriptListsSubcommandsAndFlags checks that each arm offers exactly
// what can follow that command — and nothing that cannot.
//
// EN — The last assertion is the one that earns its keep. Checking that
// "task add" offers --priority proves the node's own flags are there; checking
// that it does NOT offer --status proves the generator is walking the tree
// rather than dumping every flag it ever saw. A version that accumulated flags
// globally would pass every positive check above and offer nonsense.
//
// --verbose appearing under "task add" is the other half: it is declared only
// on the root, and reaches here only because it is Persistent. Completion and
// parsing agree on inheritance because both read the same rule from the tree.
//
// PT — A última asserção é a que se paga. Conferir que "task add" oferece
// --priority prova que as flags próprias do nó estão lá; conferir que ele NÃO
// oferece --status prova que o gerador está percorrendo a árvore em vez de
// despejar toda flag que já viu. Uma versão que acumulasse flags globalmente
// passaria em todas as checagens positivas acima e ofereceria besteira.
//
// --verbose aparecer sob "task add" é a outra metade: ele é declarado só na
// raiz, e chega aqui só por ser Persistent. Completion e parsing concordam sobre
// herança porque os dois leem a mesma regra da árvore.
func TestBashScriptListsSubcommandsAndFlags(t *testing.T) {
	script := Bash(helpTree())

	arm := armFor(t, script, "task")
	for _, w := range []string{"add", "list", "done", "remote", "--verbose", "--help"} {
		if !strings.Contains(arm, w) {
			t.Errorf("root arm should offer %q; got: %s", w, arm)
		}
	}

	addArm := armFor(t, script, "task add")
	for _, w := range []string{"--priority", "--due", "--verbose", "--help"} {
		if !strings.Contains(addArm, w) {
			t.Errorf("\"task add\" arm should offer %q (--verbose is persistent); got: %s", w, addArm)
		}
	}
	if strings.Contains(addArm, "--status") {
		t.Error("\"task add\" must not offer --status: it belongs to the list command")
	}
}

// armFor returns the single case arm whose pattern is exactly key.
//
// EN — The marker includes the closing quote and paren — `"task add")` — which
// is what makes the lookup exact. Searching for "task add" alone would also
// match inside `"task add something")`, and the test would silently assert
// against the wrong arm.
//
// t.Helper() makes a failure report the caller's line number instead of a line
// inside this function, which is the difference between "assertion 3 failed"
// and "something failed somewhere in armFor".
//
// PT — O marcador inclui as aspas e o parêntese de fechamento — `"task add")` —
// e é isso que torna a busca exata. Procurar só por "task add" também casaria
// dentro de `"task add something")`, e o teste afirmaria em silêncio contra o
// braço errado.
//
// t.Helper() faz a falha reportar a linha de quem chamou em vez de uma linha
// dentro desta função, que é a diferença entre "a asserção 3 falhou" e "algo
// falhou em algum lugar no armFor".
func armFor(t *testing.T, script, key string) string {
	t.Helper()

	marker := `"` + key + `")`
	i := strings.Index(script, marker)
	if i < 0 {
		t.Fatalf("no case arm for %q in:\n%s", key, script)
	}
	rest := script[i:]
	end := strings.Index(rest, ";;")
	if end < 0 {
		t.Fatalf("case arm for %q is not terminated", key)
	}
	return rest[:end]
}

// TestBashScriptIsSyntacticallyValid runs the generated script through
// `bash -n`, which parses without executing.
//
// EN — Know what this does NOT prove. `bash -n` checks SYNTAX, and a bare
// command line is perfectly valid syntax — a script whose first line reads
//
//	bash completion for task
//
// (a "#" dropped by accident) passes this test and then tries to RUN bash when
// sourced. So does a script whose case keys are all wrong: it parses fine and
// completes nothing.
//
// This test catches unbalanced quotes, a missing `esac`, a broken `for ((...))`
// — real and easy mistakes in generated shell. It cannot catch a script that is
// well-formed and useless. The only check for that is sourcing it in a real
// terminal and pressing TAB, which is why the milestone requires it by hand.
//
// It skips rather than fails when bash is absent, so the suite stays green on a
// machine without it. A skipped test says so out loud; a test that cannot run
// and pretends to pass does not.
//
// PT — Saiba o que isto NÃO prova. O `bash -n` checa SINTAXE, e uma linha de
// comando solta é sintaxe perfeitamente válida — um script cuja primeira linha
// diz
//
//	bash completion for task
//
// (um "#" perdido por acidente) passa neste teste e depois tenta RODAR o bash
// quando recebe source. O mesmo vale para um script cujas chaves do case estão
// todas erradas: ele parseia bem e não completa nada.
//
// Este teste pega aspas desbalanceadas, um `esac` faltando, um `for ((...))`
// quebrado — enganos reais e fáceis em shell gerado. Não consegue pegar um
// script bem-formado e inútil. A única checagem para isso é dar source num
// terminal de verdade e apertar TAB, e é por isso que o marco exige isso à mão.
//
// Ele pula em vez de falhar quando o bash não existe, para que a suíte continue
// verde numa máquina sem ele. Um teste pulado diz isso em voz alta; um teste que
// não consegue rodar e finge passar não diz.
func TestBashScriptIsSyntacticallyValid(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}

	file := filepath.Join(t.TempDir(), "completion.bash")
	if err := os.WriteFile(file, []byte(Bash(helpTree())), 0o644); err != nil {
		t.Fatalf("writing script: %v", err)
	}

	out, err := exec.Command(bash, "-n", file).CombinedOutput()
	if err != nil {
		t.Fatalf("bash -n rejected the generated script: %v\n%s", err, out)
	}
}

// TestBashScriptIsStable is the completion twin of TestHelpIsStable, and exists
// for the same Go behaviour: map iteration order is randomised on purpose.
//
// EN — It matters more here than for help. A completion script is generated
// once and written to a file that people commit or install; if the word order
// reshuffled between runs, every regeneration would produce a spurious diff and
// nobody would be able to tell a real change from noise.
//
// The sort in completionWords is what this protects.
//
// PT — Importa mais aqui do que no help. Um script de completion é gerado uma
// vez e escrito num arquivo que as pessoas commitam ou instalam; se a ordem das
// palavras se embaralhasse entre execuções, toda regeneração produziria um diff
// espúrio e ninguém conseguiria distinguir mudança real de ruído.
//
// É a ordenação em completionWords que isto protege.
func TestBashScriptIsStable(t *testing.T) {
	root := helpTree()
	for i := 0; i < 20; i++ {
		if Bash(root) != Bash(root) {
			t.Fatal("Bash() is not deterministic")
		}
	}
}
