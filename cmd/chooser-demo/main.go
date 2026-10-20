package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	overlay "github.com/rorita0x/go-overlay"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ov, err := overlay.New(overlay.Options{Namespace: "chooser-demo"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "chooser-demo:", err)
		os.Exit(1)
	}
	defer ov.Close()

	var picked string
	ov.SetChooser(overlay.Chooser{
		Choices:  gitMenu(),
		Vertical: true,
		OnChoice: func(_ int, value string) {
			picked = value
			cancel()
		},
		OnCancel: cancel,
	})

	if err := ov.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "chooser-demo:", err)
		os.Exit(1)
	}

	if picked == "" {
		fmt.Println("cancelled")
		return
	}
	fmt.Println("picked:", picked)
}

func gitMenu() []overlay.Choice {
	return []overlay.Choice{
		{Text: "Status", Value: "git status"},
		{Text: "Branch", Children: []overlay.Choice{
			{Text: "List", Value: "git branch"},
			{Text: "Create", Value: "git branch <name>"},
			{Text: "Switch", Value: "git switch <name>"},
			{Text: "Delete", Value: "git branch -d <name>"},
		}},
		{Text: "Commit", Children: []overlay.Choice{
			{Text: "Commit", Value: "git commit"},
			{Text: "Amend", Value: "git commit --amend"},
			{Text: "Commit all", Value: "git commit -a"},
		}},
		{Text: "Remote", Children: []overlay.Choice{
			{Text: "Push", Value: "git push"},
			{Text: "Pull", Value: "git pull"},
			{Text: "Fetch", Value: "git fetch"},
		}},
		{Text: "Stash", Children: []overlay.Choice{
			{Text: "Save", Value: "git stash"},
			{Text: "Pop", Value: "git stash pop"},
			{Text: "List", Value: "git stash list"},
		}},
		{Text: "Log", Value: "git log --oneline"},
		{Text: "Diff", Value: "git diff"},
	}
}
