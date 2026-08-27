# edio

Branchless, high-speed shadow version control built for AI coding agents.

<img width="1920" height="1080" alt="demo" src="https://github.com/user-attachments/assets/777a11f4-c4db-4b1c-8814-d07b51a2da0b" />

---

## Why I Built edio

In my daily workflow, I rely heavily on coding agents like Claude Code, Cursor, Windsurf, and Aider for refactoring and building features. While these tools write good code, iterative agent development creates a specific set of problems:

### The Problem
1. **Broken Workspaces:** When an agent takes a wrong turn midway through a multi-step task, there is no quick way to inspect prior turns or revert just the broken changes.
2. **Polluted Commit History:** If you let an agent commit every small attempt, your Git log fills up with messy trial-and-error commits.
3. **Branch Overhead:** Creating throwaway branches for every short agent session causes constant stash conflicts, resets language servers, and triggers slow file indexers.

### The Solution
I built `edio` to provide an invisible safety net during agent sessions without requiring temporary branches.

`edio` does not replace Git. It is a single Go binary that works inside your existing `.git` folder. It records turn-by-turn snapshots into an isolated shadow history using low-level Git plumbing.

* **Zero Index Pollution:** Your staging area (`.git/index`), unstaged changes, and active branch stay untouched.
* **Instant Rollbacks:** If an agent makes a mistake on Turn 8, you can open the terminal UI (`edio ui`), inspect the diffs, and press `r` to restore your working files to Turn 5 in milliseconds.
* **Single-Command Squash:** When the agent finishes and tests pass, `edio accept "feat: add feature"` squashes the entire session into one clean commit on your current branch.

---

## How It Works Under the Hood

I wanted `edio` to operate without interfering with normal Git workflows. Here is how it achieves isolation:

```text
User Workspace ──► Staged via GIT_INDEX_FILE ──► git write-tree ──► Shadow Commit DAG
                       (Temporary Index)                               (refs/edio/active/*)
                             │
                             ▼ (Deleted immediately)
                   Primary .git/index stays clean
```

1. **Isolated Index (`GIT_INDEX_FILE`):** Instead of running `git add` (which would overwrite your primary `.git/index`), `edio` points the `GIT_INDEX_FILE` environment variable to a temporary scratchpad. It stages files there, creates a tree object using `git write-tree`, and deletes the temporary index file immediately.
2. **Shadow Commit DAG (`refs/edio/*`):** Each turn is committed with `git commit-tree` and linked to the previous turn under a custom reference namespace (`refs/edio/active/<session_id>/<turn>`). Your active branch pointer and `HEAD` never move.
3. **Atomic Promotion on Accept:** When you run `edio accept`, `edio` reads the tree SHA from the latest turn, creates a single commit pointing to `HEAD` as its parent, and advances your active branch.
4. **Automated Garbage Collection:** Stale sessions older than 10 days have their reference pointers pruned automatically, allowing Git's native object pruner (`git prune`) to reclaim disk space.

---

## Installation

### With Homebrew (macOS & Linux)

```bash
brew install devxdh/tap/edio
```

### Quick Install (Linux & macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/devxdh/edio/main/install.sh | bash
```

### With Go

```bash
go install github.com/devxdh/edio/cmd/edio@latest
```

### Pre-built Binaries

Pre-compiled binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/devxdh/edio/releases) page.

---

## 2-Minute Quickstart

Here is how you can use `edio` in any Git repository:

### 1. Initialize your project

```bash
cd /path/to/your/project
edio init
```

This sets up `.git/edio/` and automatically configures MCP servers and lifecycle hooks for your installed AI tools.

### 2. Record Turn Snapshots

As you or your agent modify files, record snapshots after each logical step:

```bash
# Edit code
echo "func ValidateToken() {}" >> auth.go

# Take a shadow snapshot
edio snapshot -m "added token validation"
```

Your regular `git status` and commit history remain completely untouched.

### 3. Inspect Turn History

```bash
edio log
```

Output:
```text
Session sess_1787756569_0babd925 (3 turns)

* [Turn 1] (7c836f4) added HandleAuth function
* [Turn 2] (7e7dbee) added token validation
* [Turn 3] (868d16e) added auth unit tests
```

### 4. Open the Interactive UI

```bash
edio ui
```

* Navigate turns with `j` / `k` (or arrow keys).
* Inspect syntax-highlighted diffs on the right panel.
* Press `r` on any turn to instantly revert your workspace to that state.
* Press `Tab` to scroll the diff viewport.
* Press `q` to quit.

### 5. Squash and Commit

When you are satisfied with the final result:

```bash
edio accept "feat: implement user authentication"
```

All turns from the session are squashed into a single clean commit on your active branch.

---

## Connecting with AI Agents

Running `edio init` automatically configures your repository for all major agent environments:

### 1. Universal MCP Server (Cursor, Gemini CLI, Antigravity, VS Code, Windsurf)
`edio init` generates MCP configuration files for your IDEs:
* **Cursor:** `.cursor/mcp.json`
* **Gemini CLI / Antigravity:** `.gemini/settings.json`
* **VS Code / Cline / Roo Code:** `.vscode/mcp.json`

```json
{
  "mcpServers": {
    "edio": {
      "command": "edio",
      "args": ["mcp"]
    }
  }
}
```

**Exposed MCP Tools:**
* `edio_snapshot`: Allows the model to take snapshots with descriptive summaries.
* `edio_log`: Allows the model to inspect past turns in the session.
* `edio_restore`: Allows the model or user to roll back the full workspace or a single file (`-f`).

---

### 2. Lifecycle Hooks (Claude Code CLI)
`edio init` writes a `Stop` event hook to `.claude/settings.json`:
```json
{
  "hooks": {
    "Stop": [
      { "type": "command", "command": "edio snapshot -m \"prompt turn completed\"" }
    ]
  }
}
```
Every time Claude completes a turn or tool call, a snapshot is recorded automatically in the background.

---

### 3. Process Execution Wrapper (Aider, Python Scripts, CLI Loops)
For CLI agents or custom scripts without hook support, prefix the execution command with `edio run`:
```bash
# Wraps Python agent scripts
edio run python agent.py "refactor database schema"

# Wraps Aider
edio run aider --message "add unit tests"
```
`edio run` forwards terminal I/O interactively and captures a snapshot as soon as the child process exits.

> `edio init` also creates `EDIO.md` in the project root to instruct scanning LLMs (ChatGPT, Grok, DeepSeek) to use `edio snapshot` instead of running raw `git commit` commands.

---

## Storage & Garbage Collection

`edio` manages disk storage safely at the session boundary:

* **Automatic Background Cleanup:** Whenever you run `edio accept`, `edio` checks for and prunes shadow sessions older than **10 days**.
* **Manual Pruning:** You can trigger cleanup manually at any time:
  ```bash
  edio gc           # Prune sessions older than 10 days (default)
  edio gc --days 3  # Prune sessions older than 3 days
  ```
* **Active Session Safety:** `edio gc` strictly protects your active session from deletion, only pruning completed or abandoned sessions.

---

## Command Reference

| Command | Description |
| :--- | :--- |
| `edio init` | Configure shadow storage, agent hooks, and MCP servers in the repo |
| `edio snapshot -m "<msg>"` | Record an isolated turn snapshot of current workspace state |
| `edio run <command> [args...]` | Run an agent command and auto-snapshot on process exit |
| `edio log [-p]` | Display turn history for the active session (`-p` for patch diffs) |
| `edio diff [turn] [-f file]` | Show colorized diff for a specific turn |
| `edio restore <turn> [-f file]` | Roll back workspace (or single file) to a specific turn |
| `edio accept "<commit_msg>"` | Squash all session turns into a clean commit on current branch |
| `edio gc [-d <days>]` | Clean up shadow sessions older than retention limit (default: 10 days) |
| `edio ui` *(alias: `tui`)* | Launch the interactive split-pane terminal dashboard |
| `edio mcp` | Start the stdio Model Context Protocol (MCP) JSON-RPC server |

---

## Building from Source

```bash
git clone https://github.com/devxdh/edio.git
cd edio
make build
make test
```

The compiled binary will be placed at `./bin/edio`.

For an in-depth explanation of the codebase architecture and internal packages, see [ARCHITECTURE.md](ARCHITECTURE.md).

---

## License

MIT License. See [LICENSE](LICENSE) for details.
