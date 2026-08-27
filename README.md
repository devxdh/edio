# edio

Branchless, high-speed shadow version control built for AI coding agents.

<img width="1920" height="1080" alt="demo" src="https://github.com/user-attachments/assets/777a11f4-c4db-4b1c-8814-d07b51a2da0b" />

---

## Why edio?

AI coding agents (Claude Code, Cursor, Windsurf, Aider) work through rapid trial-and-error: generating code, running tests, fixing errors, and iterating across multiple turns.

Standard Git was designed for humans making deliberate, finished commits. When an agent creates dozens of micro-edits, developers face two bad options:
1. **Polluted Git Log:** Commit every turn, resulting in commit history filled with "fix typo", "try again", and "attempt 3".
2. **Lost Progress:** Don't commit, meaning a bad agent edit can overwrite working code with no clean rollback path.

`edio` solves this by recording turn-by-turn snapshots into **isolated shadow Git namespaces** (`refs/edio/*`).

* **Zero Index Pollution:** Your staging area (`.git/index`) and uncommitted files stay completely untouched.
* **Instant Rollbacks:** Jump back to any previous turn in milliseconds.
* **Interactive Terminal UI:** Inspect turn diffs side-by-side with GitHub Dark syntax highlighting.
* **One-Command Squash:** When the agent finishes, `edio accept` squashes all turns into a single clean commit on your branch.
* **Automatic Storage GC:** 10-day retention cleaner automatically reclaims disk space from old shadow sessions.

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

This creates the shadow storage directory in `.git/edio/` and generates local agent hook configurations.

### 2. Record Turn Snapshots

As you or your AI agent edit files, record snapshots after each meaningful change:

```bash
# Make some edits
echo "func HandleAuth() {}" >> auth.go

# Take a snapshot
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

## Agent Integration Guides

Running `edio init` automatically configures your repository for all major AI tools and IDEs:

### 1. Claude Code (Lifecycle Hooks)
`edio init` writes to `.claude/settings.json`:
```json
{
  "hooks": {
    "Stop": [
      { "type": "command", "command": "edio snapshot -m \"prompt turn completed\"" }
    ]
  }
}
```
Claude Code automatically records a snapshot after every prompt turn with zero manual steps.

---

### 2. Cursor, Gemini CLI, Antigravity & VS Code (Universal MCP)
`edio init` automatically creates MCP configuration files for your IDEs:
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
* `edio_snapshot`: AI models record snapshots with clear turn messages.
* `edio_log`: AI models query previous turns in the session.
* `edio_restore`: AI models or developers roll back the full workspace or a single file (`-f`).

---

### 3. Custom Scripts, Non-Hook Agents & Aider (Process Wrapper)
For custom Python agent loops, Aider, or CLI scripts without hook support, prefix with `edio run`:
```bash
# Runs the command and automatically records a snapshot when it exits
edio run python agent.py "refactor database models"
edio run aider --message "add tests"
```

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
| `edio init` | Configure shadow storage and agent hooks in the current repo |
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

## How It Works

`edio` operates directly on low-level Git plumbing objects:

1. **Isolated Index (`BuildIsolatedTree`):** During a snapshot, `edio` points `GIT_INDEX_FILE` to a temporary scratchpad in `.git/`. It runs `git add -A` and `git write-tree` inside this temporary index, keeping `.git/index` untouched.
2. **Shadow DAG (`commit-tree`):** Each turn creates a Git commit object linked to the previous turn, stored under custom reference paths (`refs/edio/active/<session_id>/<turn>`).
3. **Atomic Squash on Accept:** When `edio accept` is called, it extracts the tree SHA from the latest turn, creates a single commit pointing to `HEAD` as parent, advances the active branch pointer, and archives the session.
4. **Clean Disk Reclamation (`gc`):** When stale sessions expire, deleting their `refs/edio/*` pointers allows Git's native object pruning (`git prune`) to reclaim disk space without touching your project repository history.

For an in-depth breakdown of the codebase and internal data structures, see [ARCHITECTURE.md](ARCHITECTURE.md).

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
