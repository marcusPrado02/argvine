package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Task is one entry in the local task list.
//
// EN — The struct tags matter more than they look. Struct tag syntax is
// unforgiving: it must be `json:"name,opts"` with NO spaces around the colon or
// inside the value. Written as `json: "due, omitempty"` the whole tag is
// silently discarded — reflect.StructTag.Get("json") returns "" — and the field
// serialises under its Go name ("Due") with omitempty ignored. Nothing errors;
// the JSON is just wrong. `go vet` catches this one, which is a good reason to
// run it before every commit.
//
// PT — As struct tags importam mais do que parecem. A sintaxe é implacável:
// tem que ser `json:"name,opts"` SEM espaços em volta dos dois-pontos nem dentro
// do valor. Escrita como `json: "due, omitempty"`, a tag inteira é descartada em
// silêncio — reflect.StructTag.Get("json") devolve "" — e o campo é serializado
// com o nome Go dele ("Due") e o omitempty ignorado. Nada dá erro; o JSON só sai
// errado. O `go vet` pega esta, que é um bom motivo para rodá-lo antes de todo
// commit.
type Task struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Priority int    `json:"priority"`

	// EN: omitempty keeps an undated task out of the file entirely rather than
	// writing `"due": ""`.
	// PT: omitempty mantém uma tarefa sem data fora do arquivo por completo, em
	// vez de escrever `"due": ""`.
	Due string `json:"due,omitempty"`

	Done    bool      `json:"done"`
	Created time.Time `json:"created"`
}

// Store is the on-disk task list.
//
// EN — This file knows NOTHING about argvine. It takes plain values and returns
// plain values, which is why store_test.go can exercise it without building a
// command tree or faking an argv.
//
// That boundary is the point of the example CLI, seen from the other side: the
// Context dies in main.go. If Store.Add took a *argvine.Context, testing the
// domain would require the framework, and swapping the framework later would be
// a rewrite rather than an edit.
//
// PT — Este arquivo não sabe NADA sobre a argvine. Recebe valores comuns e
// devolve valores comuns, e é por isso que o store_test.go consegue exercitá-lo
// sem montar árvore de comandos nem falsificar um argv.
//
// Essa fronteira é o ponto da CLI de exemplo, visto do outro lado: o Context
// morre no main.go. Se Store.Add recebesse um *argvine.Context, testar o domínio
// exigiria o framework, e trocar o framework depois seria reescrita em vez de
// edição.
type Store struct {
	Tasks  []Task `json:"tasks"`
	NextID int    `json:"next_id"`

	// EN: Unexported, so encoding/json ignores it — the path is where the file
	// lives, not part of what the file contains. Load sets it after unmarshal.
	// PT: Não exportado, então o encoding/json o ignora — o caminho é onde o
	// arquivo mora, não parte do que ele contém. O Load o seta após o unmarshal.
	path string
}

// storePath resolves the file the task list lives in.
//
// EN — TASK_FILE overrides the default, and that is what lets the tests run
// against a temp directory instead of the developer's real ~/.task.json. An
// env-var escape hatch on the storage location is the cheapest way to make a
// file-backed program testable.
//
// PT — TASK_FILE sobrepõe o padrão, e é isso que permite aos testes rodarem
// contra um diretório temporário em vez do ~/.task.json real do desenvolvedor.
// Uma saída por variável de ambiente no local de armazenamento é o jeito mais
// barato de tornar testável um programa apoiado em arquivo.
func storePath() (string, error) {
	if p := os.Getenv("TASK_FILE"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".task.json"), nil
}

// Load reads the task list from disk.
//
// EN — A MISSING FILE IS NOT AN ERROR. First run of the program is the normal
// case, not the exceptional one, so it returns an empty store. Distinguishing
// "not there yet" from "there and unreadable" is what errors.Is(err,
// os.ErrNotExist) buys — a plain `err != nil` would make the first run fail.
//
// NextID is repaired rather than trusted: a hand-edited or truncated file could
// carry 0, and every task would then be created with the same id.
//
// PT — ARQUIVO AUSENTE NÃO É ERRO. A primeira execução do programa é o caso
// normal, não o excepcional, então devolve um store vazio. Distinguir "ainda não
// existe" de "existe e está ilegível" é o que o errors.Is(err, os.ErrNotExist)
// compra — um `err != nil` simples faria a primeira execução falhar.
//
// NextID é reparado em vez de confiado: um arquivo editado à mão ou truncado
// poderia trazer 0, e toda tarefa passaria a ser criada com o mesmo id.
func Load() (*Store, error) {
	path, err := storePath()
	if err != nil {
		return nil, err
	}

	s := &Store{NextID: 1}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		s.path = path
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if s.NextID < 1 {
		s.NextID = 1
	}

	// EN: After unmarshal, not before: json.Unmarshal writes into s, and path
	// being unexported means it survives — but setting it here keeps the order
	// obvious to anyone reading, and correct if path is ever exported.
	// PT: Depois do unmarshal, não antes: o json.Unmarshal escreve em s, e path
	// ser não exportado faz com que ele sobreviva — mas setar aqui mantém a
	// ordem óbvia para quem lê, e correta se path um dia for exportado.
	s.path = path
	return s, nil
}

// Save writes the task list back to disk.
//
// EN — MarshalIndent rather than Marshal because this file is meant to be read
// and hand-edited: it is a user's todo list, not a wire format.
//
// Mode 0o600 — owner read/write only. A task list can hold anything someone
// jotted down, and there is no reason for other users on the machine to read it.
//
// PT — MarshalIndent em vez de Marshal porque este arquivo deve ser lido e
// editado à mão: é a lista de tarefas de uma pessoa, não formato de transporte.
//
// Modo 0o600 — leitura e escrita só do dono. Uma lista de tarefas pode guardar
// qualquer coisa que alguém anotou, e não há razão para outros usuários da
// máquina lerem.
func (s *Store) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding tasks: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	return nil
}

// Add appends a task and returns it.
//
// EN — Three plain parameters, not a struct and not a Context. The signature is
// the boundary: everything the handler in main.go knows how to extract, this
// function knows how to receive.
//
// It returns the created Task so the caller can report the assigned id without
// reaching back into s.Tasks.
//
// PT — Três parâmetros comuns, não uma struct e não um Context. A assinatura é a
// fronteira: tudo que o handler no main.go sabe extrair, esta função sabe
// receber.
//
// Devolve a Task criada para que quem chama possa reportar o id atribuído sem
// voltar a mexer em s.Tasks.
func (s *Store) Add(title string, priority int, due string) Task {
	t := Task{
		ID:       s.NextID,
		Title:    title,
		Priority: priority,
		Due:      due,
		// EN: UTC, so a list moved between machines or timezones keeps a stable
		// ordering. Display can localise; storage should not.
		// PT: UTC, para que uma lista movida entre máquinas ou fusos mantenha
		// ordenação estável. A exibição pode localizar; o armazenamento não.
		Created: time.Now().UTC(),
	}

	s.NextID++
	s.Tasks = append(s.Tasks, t)
	return t
}

// Close marks the given ids as done and returns how many actually changed.
//
// EN — The count is of CHANGES, not of ids processed. Closing an already-closed
// task is not an error and not a change, so "closed 0 task(s)" is a truthful
// answer to `task done 1` when 1 was already done.
//
// An unknown id stops the loop and returns the count so far. That is a
// deliberate choice with a visible consequence: `task done 1 99` closes 1 and
// then reports the failure, rather than silently doing nothing. Partial work
// plus a clear error beats all-or-nothing here, because the user's next move is
// to fix the id and rerun — and rerunning a close is harmless.
//
// PT — A contagem é de MUDANÇAS, não de ids processados. Fechar uma tarefa já
// fechada não é erro e não é mudança, então "closed 0 task(s)" é resposta
// verdadeira para `task done 1` quando 1 já estava fechada.
//
// Um id desconhecido para o laço e devolve a contagem até ali. É escolha
// deliberada com consequência visível: `task done 1 99` fecha a 1 e então reporta
// a falha, em vez de não fazer nada em silêncio. Trabalho parcial mais erro claro
// ganha do tudo-ou-nada aqui, porque a ação seguinte do usuário é corrigir o id e
// rodar de novo — e refazer um fechamento é inofensivo.
func (s *Store) Close(ids []int) (int, error) {
	n := 0

	for _, id := range ids {
		found := false

		// EN: `range s.Tasks` with an index, not a value copy: s.Tasks[i].Done
		// writes into the slice, while a `for _, t := range` would write into a
		// copy and lose the change.
		// PT: `range s.Tasks` com índice, não cópia do valor: s.Tasks[i].Done
		// escreve na slice, enquanto um `for _, t := range` escreveria numa
		// cópia e perderia a mudança.
		for i := range s.Tasks {
			if s.Tasks[i].ID != id {
				continue
			}
			found = true
			if !s.Tasks[i].Done {
				s.Tasks[i].Done = true
				n++
			}
		}

		if !found {
			return n, fmt.Errorf("no task with id %d", id)
		}
	}

	return n, nil
}

// List returns the tasks matching status: "open", "done" or "all".
//
// EN — "open" is the default branch rather than an explicit case, so an
// unrecognised status behaves like "open" instead of returning nothing. That is
// safe here because Choices on the --status flag already rejects anything else
// before this is reached — the default exists so a direct caller of the domain
// cannot accidentally get an empty list from a typo.
//
// The result is a new slice; it never aliases s.Tasks, so a caller cannot
// mutate the store by writing into what List handed back.
//
// PT — "open" é o ramo default em vez de um case explícito, para que um status
// não reconhecido se comporte como "open" em vez de devolver nada. É seguro aqui
// porque o Choices da flag --status já recusa qualquer outra coisa antes de
// chegar aqui — o default existe para que quem chame o domínio direto não receba
// por acidente uma lista vazia por causa de um typo.
//
// O resultado é uma slice nova; nunca faz alias de s.Tasks, então quem chama não
// consegue mutar o store escrevendo no que o List devolveu.
func (s *Store) List(status string) []Task {
	out := make([]Task, 0, len(s.Tasks))

	for _, t := range s.Tasks {
		switch status {
		case "all":
			out = append(out, t)
		case "done":
			if t.Done {
				out = append(out, t)
			}
		default:
			if !t.Done {
				out = append(out, t)
			}
		}
	}

	return out
}
