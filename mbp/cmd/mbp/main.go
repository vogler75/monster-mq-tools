package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vogler75/monster-mq-tools/mbp/internal/component"
	"github.com/vogler75/monster-mq-tools/mbp/internal/runner"
	"github.com/vogler75/monster-mq-tools/mbp/internal/tui"
)

var (
	// Version is set during build with -ldflags
	Version = "0.1.0"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "status":
			runStatusCmd(os.Args[2:])
			return
		case "build":
			runBuildCmd(os.Args[2:])
			return
		case "publish":
			runPublishCmd(os.Args[2:])
			return
		case "clean":
			runCleanCmd(os.Args[2:])
			return
		case "version", "--version", "-v":
			fmt.Printf("mbp (Monster Build Pipeline) version %s\n", Version)
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	// Default: Launch TUI
	runTUI(os.Args[1:])
}

func printUsage() {
	fmt.Printf(`mbp - MonsterMQ Build Pipeline & Orchestrator v%s

USAGE:
  mbp [flags]                    Launch interactive Terminal UI (default)
  mbp status [flags]             Print status table of all components to stdout
  mbp build <comp|all> [flags]   Build a component or all components headlessly
  mbp publish <comp> [flags]     Publish release assets headlessly
  mbp clean <comp|all> [flags]   Clean build artifacts
  mbp version                    Display version

COMPONENTS:
  main        MonsterMQ Main Broker
  edge        MonsterMQ Edge Broker
  dashboard   MonsterMQ Dashboard
  explorer    MonsterMQ Explorer
  tools       MonsterMQ Tools (cli/mmq, i3x)
  all         All components sequentially

FLAGS:
  --root <path>    Override path to monster repositories directory (default: parent dir)
  --json           Output status in JSON format (status command only)
  --target <id>    Specify explicit target ID (e.g. build-docker, build-deb)
  -y, --yes        Auto-confirm without prompt (publish command only)
  -h, --help       Show help
`, Version)
}

func runTUI(args []string) {
	fs := flag.NewFlagSet("mbp", flag.ExitOnError)
	rootFlag := fs.String("root", "", "Path to monster repositories root")
	_ = fs.Parse(args)

	rootDir, err := component.FindMonsterRoot(*rootFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not determine Monster repository root: %v\n", err)
		os.Exit(1)
	}

	model, err := tui.NewAppModel(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing MBP: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func runStatusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	rootFlag := fs.String("root", "", "Path to monster repositories root")
	jsonFlag := fs.Bool("json", false, "Output JSON format")
	_ = fs.Parse(args)

	rootDir, err := component.FindMonsterRoot(*rootFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	comps, err := component.LoadAndRefreshAll(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading components: %v\n", err)
		os.Exit(1)
	}

	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(comps)
		return
	}

	fmt.Printf("MonsterMQ Components Status (Root: %s)\n\n", rootDir)
	fmt.Printf("%-12s %-10s %-24s %-14s %s\n", "COMPONENT", "VERSION", "GIT STATUS", "BUILD STATUS", "ARTIFACT")
	fmt.Println(strings.Repeat("─", 80))

	for _, c := range comps {
		gitStr := "no git"
		if c.Git.IsRepo {
			parts := []string{c.Git.Branch}
			if c.Git.Behind > 0 {
				parts = append(parts, fmt.Sprintf("▼%d behind (update avail!)", c.Git.Behind))
			} else if c.Git.Ahead > 0 {
				parts = append(parts, fmt.Sprintf("▲%d ahead", c.Git.Ahead))
			}
			if c.Git.IsDirty {
				parts = append(parts, fmt.Sprintf("*%d dirty", c.Git.DirtyFilesCount))
			} else if c.Git.Behind == 0 && c.Git.Ahead == 0 {
				parts = append(parts, "clean")
			}
			gitStr = strings.Join(parts, " ")
		}

		artStr := "none"
		if c.LatestArtifact != nil {
			artStr = fmt.Sprintf("%s (%d bytes)", c.LatestArtifact.Name, c.LatestArtifact.Size)
		}

		fmt.Printf("%-12s %-10s %-24s %-14s %s\n", c.ID, c.Version, gitStr, string(c.Status), artStr)
	}
}

func runBuildCmd(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	rootFlag := fs.String("root", "", "Path to monster repositories root")
	targetFlag := fs.String("target", "", "Explicit target ID")
	_ = fs.Parse(args)

	if fs.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Error: Specify a component ID to build (main, edge, dashboard, explorer, tools) or 'all'\n")
		os.Exit(1)
	}

	targetComp := fs.Arg(0)
	rootDir, err := component.FindMonsterRoot(*rootFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	comps, err := component.LoadAndRefreshAll(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if targetComp == "all" {
		for _, c := range comps {
			if c.ID == "tools" {
				continue
			}
			fmt.Printf("\n=== Building %s (%s) ===\n", c.Name, c.ID)
			if err := executeTaskCLI(c.ID, "build", c.DefaultBuild, c.Directory); err != nil {
				fmt.Fprintf(os.Stderr, "Build failed for %s: %v\n", c.ID, err)
				os.Exit(1)
			}
		}
		fmt.Println("\n✔ All components built successfully!")
		return
	}

	comp, err := component.FindComponent(comps, targetComp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	target := comp.DefaultBuild
	if *targetFlag != "" {
		found := false
		for _, t := range comp.BuildTargets {
			if t.ID == *targetFlag {
				target = t
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Error: Target '%s' not found for component %s\n", *targetFlag, comp.ID)
			os.Exit(1)
		}
	}

	fmt.Printf("Building %s (%s) using target '%s': %s %s\n\n", comp.Name, comp.ID, target.Name, target.Command, strings.Join(target.Args, " "))
	if err := executeTaskCLI(comp.ID, "build", target, comp.Directory); err != nil {
		os.Exit(1)
	}
}

func runPublishCmd(args []string) {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	rootFlag := fs.String("root", "", "Path to monster repositories root")
	targetFlag := fs.String("target", "", "Explicit target ID")
	yesFlag := fs.Bool("y", false, "Auto-confirm")
	_ = fs.Parse(args)

	if fs.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Error: Specify a component ID to publish (main, edge, dashboard, explorer)\n")
		os.Exit(1)
	}

	targetComp := fs.Arg(0)
	rootDir, err := component.FindMonsterRoot(*rootFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	comps, err := component.LoadAndRefreshAll(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	comp, err := component.FindComponent(comps, targetComp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	target := comp.DefaultPublish
	if *targetFlag != "" {
		found := false
		for _, t := range comp.PublishTargets {
			if t.ID == *targetFlag {
				target = t
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Error: Target '%s' not found for component %s\n", *targetFlag, comp.ID)
			os.Exit(1)
		}
	}

	if !*yesFlag {
		fmt.Printf("Are you sure you want to publish %s (v%s)? [y/N]: ", comp.Name, comp.Version)
		var response string
		_, _ = fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Publishing aborted.")
			return
		}
	}

	fmt.Printf("Publishing %s (%s) using target '%s'...\n\n", comp.Name, comp.ID, target.Name)
	if err := executeTaskCLI(comp.ID, "publish", target, comp.Directory); err != nil {
		os.Exit(1)
	}
}

func runCleanCmd(args []string) {
	fs := flag.NewFlagSet("clean", flag.ExitOnError)
	rootFlag := fs.String("root", "", "Path to monster repositories root")
	_ = fs.Parse(args)

	if fs.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Error: Specify a component ID to clean (main, edge, dashboard, explorer, tools) or 'all'\n")
		os.Exit(1)
	}

	targetComp := fs.Arg(0)
	rootDir, err := component.FindMonsterRoot(*rootFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	comps, err := component.LoadAndRefreshAll(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if targetComp == "all" {
		for _, c := range comps {
			if c.CleanTarget != nil {
				fmt.Printf("Cleaning %s...\n", c.Name)
				_ = executeTaskCLI(c.ID, "clean", *c.CleanTarget, c.Directory)
			}
		}
		fmt.Println("✔ Clean complete.")
		return
	}

	comp, err := component.FindComponent(comps, targetComp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if comp.CleanTarget == nil {
		fmt.Printf("No clean target configured for %s\n", comp.ID)
		return
	}

	if err := executeTaskCLI(comp.ID, "clean", *comp.CleanTarget, comp.Directory); err != nil {
		os.Exit(1)
	}
}

func executeTaskCLI(componentID, action string, target component.Target, dir string) error {
	task := runner.NewTask(componentID, action, target.ID, target.Command, target.Args, dir)
	ctx := context.Background()

	if err := task.Start(ctx); err != nil {
		return err
	}

	for line := range task.LineChan() {
		if line.IsError {
			fmt.Fprintf(os.Stderr, "%s\n", line.Text)
		} else {
			fmt.Printf("%s\n", line.Text)
		}
	}

	<-task.DoneChan()

	if task.Status != runner.TaskSuccess {
		return fmt.Errorf("task failed with status %s (exit code %d)", task.Status, task.ExitCode)
	}
	return nil
}
