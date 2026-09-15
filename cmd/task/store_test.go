package main

import (
	"path/filepath"
	"testing"
)

// tempStore points the store at a throwaway file and loads it.
//
// EN — Two standard-library helpers do all the work, and both matter:
//
//	t.TempDir()  a directory removed automatically when the test ends
//	t.Setenv()   an env var restored automatically when the test ends
//
// Without them these tests would read and write the developer's real
// ~/.task.json — destroying data and producing results that depend on whatever
// happened to be in it.
//
// Note there is no import of argvine anywhere in this file. That is the boundary
// working: the domain is testable without the framework.
//
// PT — Dois helpers da biblioteca padrão fazem todo o trabalho, e os dois
// importam:
//
//	t.TempDir()  um diretório removido automaticamente quando o teste acaba
//	t.Setenv()   uma variável de ambiente restaurada quando o teste acaba
//
// Sem eles, estes testes leriam e escreveriam o ~/.task.json real do
// desenvolvedor — destruindo dados e produzindo resultados que dependem do que
// por acaso estivesse lá.
//
// Note que não há import de argvine em lugar nenhum deste arquivo. É a fronteira
// funcionando: o domínio é testável sem o framework.
func tempStore(t *testing.T) *Store {
	t.Helper()
	t.Setenv("TASK_FILE", filepath.Join(t.TempDir(), "task.json"))

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() returned %v", err)
	}
	return s
}

// TestStoreRoundTrip writes tasks, saves, and loads them back in a new Store.
//
// EN — A round trip is the only test that catches serialisation bugs, because
// both halves have to agree. Checking Add in isolation would pass with a broken
// struct tag; checking Load in isolation would have nothing to load. Only
// write-then-read-with-a-fresh-struct proves the file is a faithful record.
//
// NextID is asserted after reload for the same reason: it is the one piece of
// state that must survive the file, or ids start repeating on the next run.
//
// The second task is added with an empty due date on purpose — that is the
// field carrying omitempty, and the one whose tag was silently broken.
//
// PT — Uma ida e volta é o único teste que pega bug de serialização, porque as
// duas metades têm que concordar. Checar o Add isolado passaria com struct tag
// quebrada; checar o Load isolado não teria o que carregar. Só
// escrever-depois-ler-com-struct-nova prova que o arquivo é registro fiel.
//
// NextID é afirmado depois do reload pela mesma razão: é o único pedaço de estado
// que precisa sobreviver ao arquivo, ou os ids começam a repetir na execução
// seguinte.
//
// A segunda tarefa é adicionada com data vazia de propósito — é o campo que
// carrega omitempty, e aquele cuja tag estava silenciosamente quebrada.
func TestStoreRoundTrip(t *testing.T) {
	s := tempStore(t)

	first := s.Add("buy bread", 2, "2026-09-10")
	second := s.Add("write plan", 1, "")

	if first.ID != 1 || second.ID != 2 {
		t.Fatalf("ids = %d, %d; want 1, 2", first.ID, second.ID)
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() returned %v", err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatalf("Load() returned %v", err)
	}
	if len(reloaded.Tasks) != 2 {
		t.Fatalf("reloaded %d tasks, want 2", len(reloaded.Tasks))
	}
	if reloaded.Tasks[0].Title != "buy bread" || reloaded.Tasks[0].Priority != 2 {
		t.Errorf("first task = %+v, want title \"buy bread\" and priority 2", reloaded.Tasks[0])
	}
	if reloaded.NextID != 3 {
		t.Errorf("NextID = %d, want 3", reloaded.NextID)
	}
}

// TestStoreLoadMissingFileIsEmpty pins that a first run is not a failure.
//
// EN — tempStore points at a file that was never created, so this is exactly
// the situation of someone installing the CLI and running it for the first
// time. An implementation that treated any read error as fatal would make the
// program unusable until the user hand-created an empty JSON file.
//
// NextID is checked too: an empty store must still be ready to hand out id 1.
//
// PT — tempStore aponta para um arquivo que nunca foi criado, então esta é
// exatamente a situação de alguém instalando a CLI e rodando pela primeira vez.
// Uma implementação que tratasse qualquer erro de leitura como fatal tornaria o
// programa inutilizável até o usuário criar um JSON vazio à mão.
//
// NextID é conferido também: um store vazio ainda precisa estar pronto para
// entregar o id 1.
func TestStoreLoadMissingFileIsEmpty(t *testing.T) {
	s := tempStore(t)

	if len(s.Tasks) != 0 {
		t.Errorf("a missing file must load as an empty store, got %d tasks", len(s.Tasks))
	}
	if s.NextID != 1 {
		t.Errorf("NextID = %d, want 1", s.NextID)
	}
}

// TestStoreClose checks that closing touches exactly the ids asked for.
//
// EN — Three tasks and a non-contiguous selection {1, 3} on purpose: the middle
// one is the control. An implementation that closed a RANGE, or closed
// everything, or closed by slice index instead of by ID, would satisfy the
// count and fail on task 2 still being open.
//
// The last assertion covers the unknown id. It checks only that an error came
// back, not the partial count — that behaviour (close what you can, then
// report) is deliberate but not load-bearing, and pinning it here would make it
// harder to change than it deserves.
//
// PT — Três tarefas e uma seleção não contígua {1, 3} de propósito: a do meio é
// o controle. Uma implementação que fechasse um INTERVALO, ou fechasse tudo, ou
// fechasse por índice da slice em vez de por ID, satisfaria a contagem e
// falharia na tarefa 2 continuar aberta.
//
// A última asserção cobre o id desconhecido. Confere só que veio um erro, não a
// contagem parcial — esse comportamento (feche o que der, depois reporte) é
// deliberado mas não estrutural, e fixá-lo aqui o tornaria mais difícil de mudar
// do que merece.
func TestStoreClose(t *testing.T) {
	s := tempStore(t)
	s.Add("a", 3, "")
	s.Add("b", 3, "")
	s.Add("c", 3, "")

	n, err := s.Close([]int{1, 3})
	if err != nil {
		t.Fatalf("Close() returned %v", err)
	}
	if n != 2 {
		t.Errorf("Close closed %d tasks, want 2", n)
	}
	if !s.Tasks[0].Done || s.Tasks[1].Done || !s.Tasks[2].Done {
		t.Errorf("done flags = %v, %v, %v; want true, false, true",
			s.Tasks[0].Done, s.Tasks[1].Done, s.Tasks[2].Done)
	}

	if _, err := s.Close([]int{99}); err == nil {
		t.Error("closing an unknown id should fail")
	}
}

// TestStoreList checks the three filters against one two-task store where
// exactly one task is closed.
//
// EN — The setup is minimal on purpose: with one open and one done, the three
// answers must be 2, 1 and 1. Any filter that ignored its argument, or inverted
// the condition, breaks at least one of them. A store where both tasks shared a
// state would let an inverted "done" check pass.
//
// PT — O setup é mínimo de propósito: com uma aberta e uma fechada, as três
// respostas têm que ser 2, 1 e 1. Qualquer filtro que ignorasse o argumento, ou
// invertesse a condição, quebra pelo menos uma delas. Um store em que as duas
// tarefas compartilhassem o mesmo estado deixaria passar um "done" invertido.
func TestStoreList(t *testing.T) {
	s := tempStore(t)
	s.Add("a", 3, "")
	s.Add("b", 3, "")
	if _, err := s.Close([]int{1}); err != nil {
		t.Fatalf("Close() returned %v", err)
	}

	if got := len(s.List("all")); got != 2 {
		t.Errorf("List(\"all\") = %d, want 2", got)
	}
	if got := len(s.List("open")); got != 1 {
		t.Errorf("List(\"open\") = %d, want 1", got)
	}
	if got := len(s.List("done")); got != 1 {
		t.Errorf("List(\"done\") = %d, want 1", got)
	}
}
