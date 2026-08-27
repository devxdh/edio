# edio

Branchless, high-speed shadow version control built for AI coding agents.

<img width="1920" height="1080" alt="demo" src="https://github.com/user-attachments/assets/777a11f4-c4db-4b1c-8814-d07b51a2da0b" />

---

Over the past few months, I found myself using coding agents like Claude Code, Cursor, Windsurf, and Aider for almost every non-trivial refactor. While they write good code, they create a very specific kind of mess in day-to-day development.

### The Problem

When you let an agent work across 10 or 15 turns, things inevitably go off the rails at some point:

1. **Broken workspaces:** When an agent makes a mistake midway through a multi-turn task, you have no quick way to inspect prior turns or undo just the bad edits.
2. **Polluted commit history:** Letting an agent commit after every small step clutters your Git log with throwaway trial-and-error commits ("fix typo", "try again", "attempt 3").
3. **Branch friction:** Creating temporary branches for quick agent tasks causes constant stash conflicts, resets language servers, and triggers slow file watchers.

### What is edio?

`edio` provides an invisible safety net when running coding agents, without needing throwaway branches.

**To be clear:** `edio` is not a new version control system and does not replace Git. It is a single Go binary that operates entirely inside your existing `.git` directory, using low-level Git plumbing to record snapshots of each agent turn in the background.

* **Zero Index Pollution:** Your staging area (`.git/index`), unstaged changes, and active branch stay completely untouched.
* **Instant Rollbacks:** If an agent makes a mistake on Turn 8, open the split-pane TUI (`edio ui`), scrub back to Turn 5, check the diff, and press `r` to instantly roll your files back to that exact working state.
* **One-Command Squash:** When the agent finishes and tests pass, run `edio accept "feat: add feature"` to squash the entire session into one clean commit on your active branch.
* **Automatic Storage GC:** 10-day retention cleaner automatically prunes old shadow sessions.

---

## Installation

### Quick Install (Linux & macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/devxdh/edio/main/install.sh | bash
```

### With Go

```bash
go install github.com/devxdh/edio/cmd/edio@latest
```

### Pre-built Binaries

Download pre-compiled binaries for Linux, macOS, and Windows from the [GitHub Releases](https://github.com/devxdh/edio/releases) page.

---

## First-Time User Walkthrough

Here is how to use `edio` in any Git repository in under 2 minutes:

### 1. Initialize edio in your project

```bash
cd /path/to/your/project
edio init
```

This creates the shadow storage directory in `.git/edio/` and auto-configures hooks & MCP servers for your AI tools.

### 2. Record Turn Snapshots

As you or your AI agent edit files, record snapshots after each meaningful change:

```bash
# Make some edits
echo "func HandleAuth() {}" >> auth.go

# Take an isolated shadow snapshot
edio snapshot -m "added HandleAuth function"
```

You can record as many turns as you want. Your regular Git status and commit history remain completely clean.

### 3. View Turn History

List all turns in your current session:

```bash
edio log
```

Output:
```text
Session sess_1787756569_0babd925 (3 turns)

* [Turn 1] (7c836f4) added HandleAuth function
* [Turn 2] (7e7dbee) added JWT validation logic
* [Turn 3] (868d16e) added unit tests for auth module
```

### 4. Launch the Interactive UI

```bash
edio ui
```

* Use `j` / `k` (or arrow keys / mouse) to browse turns in the timeline.
* View syntax-highlighted diffs on the right pane.
* Press `r` on any turn to instantly roll back your working files to that state.
* Press `Tab` to scroll the diff viewport.
* Press `q` to exit.

### 5. Accept and Commit

When you are satisfied with the agent's work and want to make an official commit:

```bash
edio accept "feat: implement user authentication and unit tests"
```

All session turns are squashed into a single commit on your current branch, and the shadow session is archived.

---

## 3 Ways It Integrates with Agents

Running `edio init` automatically configures your workspace for all major AI tools:

### 1. Universal MCP Server (Cursor, Gemini CLI, Antigravity, VS Code, Windsurf)

`edio init` automatically configures:
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

**Exposed Native MCP Tools:**
* `edio_snapshot`: AI models record snapshots with clear turn messages.
* `edio_log`: AI models query previous turns in the session.
* `edio_restore`: AI models or developers roll back the full workspace or a single file (`-f`).

---

### 2. Auto Lifecycle Hooks (Claude Code CLI)

`edio init` writes a `Stop` hook into `.claude/settings.json`:

```json
{
  "hooks": {
    "Stop": [
      { "type": "command", "command": "edio snapshot -m \"prompt turn completed\"" }
    ]
  }
}
```

Every time Claude finishes a prompt turn or tool call, a snapshot is captured automatically in the background with zero manual steps.

---

### 3. Universal Process Wrapper (Aider, Python Scripts, CLI Loops)

For custom Python agent loops, Aider, or CLI scripts without hook support, prefix your command with `edio run`:

```bash
# Wraps Python agent execution
edio run python agent.py "refactor database models"

# Wraps Aider
edio run aider --message "add unit tests"
```

`edio run` passes through terminal I/O interactively and automatically records a snapshot the moment the child process exits.

> **Universal Prompt Rules:** `edio init` also generates `EDIO.md` in your project root to instruct scanning LLMs (ChatGPT, Grok, DeepSeek) to use `edio snapshot` instead of running raw `git commit` commands.

---

## How It Works Under the Hood

`edio` is designed to be completely invisible to normal Git operations:

1. **Temporary Index Files (`GIT_INDEX_FILE`):** Instead of running `git add` (which would overwrite your real `.git/index`), `edio` points `GIT_INDEX_FILE` to a temporary scratch file in `.git/`. It stages files there, creates a tree object with `git write-tree`, and commits it with `git commit-tree`.
2. **Shadow DAG (`refs/edio/*`):** Each turn commit is saved directly into custom reference paths (`refs/edio/active/<session_id>/<turn>`) instead of a branch. Your actual staging area, your unstaged changes, and your `HEAD` remain completely untouched.
3. **Atomic Squash on Accept:** When you run `edio accept`, `edio` reads the tree SHA of your latest turn, creates a single commit pointing to `HEAD` as parent, advances your active branch pointer, and archives the session.
4. **Clean Disk Reclamation (`gc`):** Stale sessions older than 10 days have their `refs/edio/*` pointers pruned automatically, allowing Git's native garbage collector (`git prune`) to reclaim disk space.

For an in-depth breakdown of data structures and internal plumbing, see [ARCHITECTURE.md](ARCHITECTURE.md).

---

## Garbage Collection & Storage Management

`edio` manages disk space safely at the session boundary:

* **Automatic Cleanup:** Every time you run `edio accept`, `edio` triggers a lightweight background check to prune shadow sessions older than **10 days**.
* **Manual Cleanup:** You can manually run garbage collection at any time:
  ```bash
  # Prune sessions older than 10 days (default)
  edio gc

  # Prune sessions older than 3 days
  edio gc --days 3
  ```
* **Safety Invariant:** `edio gc` strictly protects your currently active session from deletion, and only prunes completed or abandoned sessions that exceed the retention limit.

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

Binary will be generated at `./bin/edio`.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
