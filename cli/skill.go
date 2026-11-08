package main

// `mmq skill` generates an agent skill (SKILL.md) from the command catalog, so the
// skill can never drift from the binary. Install location defaults to ~/.claude/skills.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func generateSkill() string {
	doc := buildCatalogDoc()
	var b strings.Builder
	b.WriteString("---\nname: monstermq-cli\ndescription: Operate MonsterMQ (Full and Edge broker) with the mmq CLI - topics, publish, archives, sessions, users, devices, scripts, HMI and any GraphQL operation. Use when reading or changing a MonsterMQ broker.\n---\n\n")
	b.WriteString("# MonsterMQ CLI (`mmq`) - generated from `mmq commands`\n\n")
	b.WriteString("## Rules for agents\n\n")
	b.WriteString("1. Always pass `--json` (accepted anywhere). Failures print `{\"success\":false,\"error\":\"...\"}` and exit 1; check the exit code.\n")
	b.WriteString("2. Connection: `--url`/`--host`/`--port`/`--https`, `--user`/`--pass` or `--token`, or env `MQ_URL, MQ_HOST, MQ_PORT, MQ_USER, MQ_PASS, MQ_TOKEN`, or `.env`.\n")
	b.WriteString("3. Run `mmq features` first: Full broker has archives, users, namespaces, flows; Edge has fewer operations.\n")
	b.WriteString("4. Never guess JSON. Discover it from the connected broker:\n")
	b.WriteString("   - `mmq schema [filter]` lists every query/mutation; `mmq schema user.createUser` shows arguments and input types; `mmq schema CreateUserInput` describes a type.\n")
	b.WriteString("   - `mmq api template <path>` prints an argument skeleton; `mmq device template <type>` a device document.\n")
	b.WriteString("5. Preview before mutating: `--dry-run` (`api`, `<noun>`, `device apply`). Secrets are redacted in output (`--reveal` to show).\n")
	b.WriteString("6. JSON input comes from `--args '<json>'`, `--args @file.json`, or stdin (`--args -`). `key=value` pairs also work; for operations with one input object the fields may be given flat.\n")
	b.WriteString("7. For anything without a dedicated command use `mmq api <path>` or `mmq gql '<query>' --vars <json>`.\n")
	b.WriteString("8. Prefer `subscribe` over `monitor` (monitor is an interactive TUI).\n\n")
	b.WriteString("## Commands\n\n| Command | Brokers | Input | Writes | Usage |\n|---|---|---|---|---|\n")
	for _, c := range doc.Commands {
		w := ""
		if c.Mutating {
			w = "yes"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | `%s` - %s |\n", c.Name, c.Brokers, c.Input, w, strings.ReplaceAll(c.Usage, "|", "\\|"), strings.ReplaceAll(c.Summary, "|", "\\|"))
	}
	b.WriteString("\nFull option lists: `mmq commands <name> --json`. Every command also has `--help`.\n\n")
	b.WriteString("## Global options\n\n")
	for _, o := range doc.GlobalOptions {
		fmt.Fprintf(&b, "- `%s` - %s\n", o.Name, o.Description)
	}
	b.WriteString("\n## Exit codes\n\n")
	for _, k := range []string{"0", "1", "2"} {
		fmt.Fprintf(&b, "- `%s` - %s\n", k, doc.ExitCodes[k])
	}
	b.WriteString("\n## Typical workflows\n\n```bash\nmmq --json features\nmmq --json searchTopics 'sensors/#'\nmmq --json publish demo/a '{\"v\":1}' --retain\nmmq --json schema topicNamespace\nmmq --json namespace create --args @ns.json --dry-run\nmmq --json device template MQTT-Client > dev.json && mmq --json device apply dev.json --dry-run\nmmq --json gql '{ brokers { nodeId version } }'\n```\n\n")
	b.WriteString("Examples ship in `examples/devices` and `examples/api` of the tools repository.\n")
	return b.String()
}

const skillHelp = `Usage: mmq skill [--output|-o file] [--install [dir]]

Print the generated agent skill (SKILL.md) for mmq. --install writes it to
[dir] or ~/.claude/skills/monstermq-cli/SKILL.md.
`

func runSkill(args []string) error {
	var out, install string
	doInstall := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help", "help":
			fmt.Print(skillHelp)
			return nil
		case "--output", "-o":
			i++
			if i >= len(args) {
				return fmt.Errorf("%s requires a file", args[i-1])
			}
			out = args[i]
		case "--install":
			doInstall = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				install = args[i]
			}
		default:
			return fmt.Errorf("unknown option %s", args[i])
		}
	}
	content := generateSkill()
	if doInstall {
		if install == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			install = filepath.Join(home, ".claude", "skills", "monstermq-cli")
		}
		if err := os.MkdirAll(install, 0o755); err != nil {
			return err
		}
		out = filepath.Join(install, "SKILL.md")
	}
	if out == "" {
		fmt.Print(content)
		return nil
	}
	if err := os.WriteFile(out, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Println("Skill written to", out)
	return nil
}
