package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/marcusPrado02/argvine"
)


var root *argvine.Command

func main() {
	root := buildTree()


	if err  := root.Validate(); err != nil {
		panic(err)
	}

	ctx, err := argvine.Parse(root, os.Args[1:])
	if err != nil {
		var help *argvine.ErrHelpRequested
		if errors.As(err, &help) {
			fmt.Print(help.Help())
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "task:", err)
		os.Exit(1)
	}

	if ctx.Cmd.Run == nil {
		fmt.Print(ctx.Help())
		os.Exit(1)
	}

	if err := ctx.Cmd.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "task:", err)
		os.Exit(1)
	}

}


func buildTree() *argvine.Command {
	return &argvine.Command{
			Name:  "task",
		Short: "local task manager",
		Long:  "task keeps a list of things to do in a local JSON file.",
		Flags: []argvine.Flag{
			{Name: "verbose", Short: "v", Type: argvine.Bool, Default: false,
				Usage: "print extra detail", Persistent: true},
		},
		Sub: []*argvine.Command{
			{
				Name:  "add",
				Short: "create a task",
				Long:  "Create a task. The title is taken verbatim; use -- before a title that starts with a dash.",
				Flags: []argvine.Flag{
					{Name: "priority", Short: "p", Type: argvine.Int, Default: 3,
						Usage: "priority from 1 (high) to 5 (low)"},
					{Name: "due", Short: "d", Type: argvine.String, Default: "",
						Usage: "due date as YYYY-MM-DD"},
				},
				Args: []argvine.Arg{
					{Name: "title", Arity: argvine.One, Usage: "what to do"},
				},
				Run: runAdd,
			},
			{
				Name:  "list",
				Short: "list tasks",
				Flags: []argvine.Flag{
					{Name: "status", Short: "s", Type: argvine.String, Default: "open",
						Choices: []string{"open", "done", "all"}, Usage: "which tasks to show"},
				},
				Run: runList,
			},
			{
				Name:  "done",
				Short: "close one or more tasks",
				Args: []argvine.Arg{
					{Name: "ids", Arity: argvine.Many, Usage: "task ids to close"},
				},
				Run: runDone,
			},
			{
				Name:  "completion",
				Short: "print a shell completion script",
				Args: []argvine.Arg{
					{Name: "shell", Arity: argvine.One, Usage: "target shell (bash)"},
				},
				Run: runCompletion,
			},
		},
	}
}

func runAdd(ctx *argvine.Context) error {
	store, err := Load()
	if err != nil {
		return err
	}

	priority := ctx.Int("priority")
	if priority < 1 || priority > 5  {
		return fmt.Errorf("priority must be between 1 and 5, got %d", priority)
	}

	t := store.Add(ctx.Arg("title"), priority, ctx.String("due"))
	if err := store.Save(); err != nil {
		return err
	}

	if ctx.Bool("verbose") {
		fmt.Printf("added #%d %q (priority %d, due %q)\n", t.ID, t.Title, t.Priority, t.Due)
		return nil
	}
	fmt.Printf("added #%d\n", t.ID)
	return nil

}

func runList(ctx *argvine.Context) error {
	store, err := Load()
	if err != nil {
		return err
	}

	tasks := store.List(ctx.String("status"))
	if len(tasks) == 0 {
		fmt.Println("no tasks")
		return nil
	}

	for _, t := range tasks {
		mark := " "
		if t.Done {
			mark = "x"
		}
		fmt.Printf("[%s] #%-3d p%d  %s", mark, t.ID, t.Priority, t.Title)
		if t.Due != "" {
			fmt.Printf("  (due %s)", t.Due)
		}
		if ctx.Bool("verbose") {
			fmt.Printf("  created %s", t.Created.Format("2006-01-02 15:04"))
		}
		fmt.Println()
	}
	return nil
}

func runDone(ctx *argvine.Context) error {
	store, err := Load()
	if err != nil {
		return err
	}

	raw := ctx.ArgList("ids")
	ids := make([]int, 0, len(raw))
	for _, s := range raw {
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("invalid task id %q", s)
		}
		ids = append(ids, n)
	}

	n, err := store.Close(ids)
	if err != nil {
		return err
	}
	if err := store.Save(); err != nil {
		return err
	}
	fmt.Printf("closed %d task(s)\n", n)
	return nil
}

func runCompletion(ctx *argvine.Context) error {
	switch shell := ctx.Arg("shell"); shell {
	case "bash":
		fmt.Print(argvine.Bash(root))
		return nil
	default:
		return fmt.Errorf("unsupported shell %q: only bash is supported", shell)
	}
}
