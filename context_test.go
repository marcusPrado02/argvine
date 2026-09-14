package argvine

import "testing"

// TestContextTypedAccessors builds a Context by hand and reads it back.
//
// EN — Reaching into the unexported maps is deliberate: when this was written
// Parse did not exist yet, and the accessors had to be correct before anything
// filled them. An in-package test is the only thing that can do this, which is
// one reason the test file shares the package instead of being argvine_test.
//
// PT — Mexer nos mapas não exportados é deliberado: quando isto foi escrito o
// Parse ainda não existia, e os acessores precisavam estar certos antes de algo
// preenchê-los. Um teste dentro do pacote é a única coisa capaz disso, e essa é
// uma das razões de o arquivo de teste compartilhar o pacote em vez de ser
// argvine_test.
func TestContextTypedAccessors(t *testing.T) {
	root := &Command{Name: "task"}
	add := &Command{Name: "add"}

	ctx := newContext()
	ctx.Cmd = add
	ctx.Path = []*Command{root, add}
	ctx.flags["verbose"] = true
	ctx.flags["priority"] = 2
	ctx.flags["status"] = "done"
	ctx.args["title"] = []string{"buy bread"}
	ctx.args["ids"] = []string{"1", "2", "3"}

	if got := ctx.Bool("verbose"); got != true {
		t.Errorf("Bool(\"verbose\") = %v, want true", got)
	}
	if got := ctx.Int("priority"); got != 2 {
		t.Errorf("Int(\"priority\") = %v, want 2", got)
	}
	if got := ctx.String("status"); got != "done" {
		t.Errorf("String(\"status\") = %q, want \"done\"", got)
	}
	if got := ctx.Arg("title"); got != "buy bread" {
		t.Errorf("Arg(\"title\") = %q, want \"buy bread\"", got)
	}
	// EN: ArgList is checked element by element rather than with
	// reflect.DeepEqual so a failure names which position went wrong.
	// PT: ArgList é checado elemento a elemento em vez de com reflect.DeepEqual
	// para que a falha diga qual posição saiu errada.
	if got := ctx.ArgList("ids"); len(got) != 3 || got[0] != "1" || got[1] != "2" || got[2] != "3" {
		t.Errorf("ArgList(\"ids\") = %v, want [\"1\", \"2\", \"3\"]", got)
	}
	if got := ctx.PathString(); got != "task add" {
		t.Errorf("PathString() = %q, want \"task add\"", got)
	}
}

// TestContextUnknownFlagPanics pins the decision that a misspelled flag name is
// a programming error, not a usage error.
//
// EN — Returning the zero value instead would hide the bug: the CLI would run,
// the wrong value would flow through, and the mistake would surface far from
// its cause. Failing loudly on the developer's first run is the point.
//
// PT — Devolver o valor zero esconderia o bug: a CLI rodaria, o valor errado
// fluiria adiante, e o engano apareceria longe da causa. Falhar alto na primeira
// execução do desenvolvedor é o ponto.
func TestContextUnknownFlagPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Int on unknown flag did not panic; it is a programming error, not a usage error")
		}
	}()

	add := &Command{Name: "add"}
	ctx := newContext()
	ctx.Cmd = add
	// EN: Path is set so the panic message names the command; without it the
	// message would read `on ""`, which helps nobody.
	// PT: Path é setado para que a mensagem de panic nomeie o comando; sem ele a
	// mensagem sairia `on ""`, que não ajuda ninguém.
	ctx.Path = []*Command{{Name: "task"}, add}

	_ = ctx.Int("prot")
}

// TestContextWrongTypePanics covers the second half of the same contract: the
// flag exists, but the accessor asks for the wrong type.
//
// EN — This is what a typo in the tree costs — declaring a flag as String and
// reading it with Int — and it must fail as loudly as an unknown name.
//
// PT — É isto que um engano na árvore custa — declarar uma flag como String e
// lê-la com Int — e tem que falhar tão alto quanto um nome desconhecido.
func TestContextWrongTypePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Int on string flag did not panic; it is a programming error, not a usage error")
		}
	}()

	add := &Command{Name: "add"}
	ctx := newContext()
	ctx.Cmd = add
	ctx.Path = []*Command{{Name: "task"}, add}
	ctx.flags["status"] = "done"

	_ = ctx.Int("status")
}
