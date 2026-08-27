# edio Architecture & Codebase Guide

This document explains the internal architecture, design decisions, and data flow of `edio`. It is written for developers and contributors who want to understand or extend the codebase.

---

## 1. High-Level Concept

`edio` is an isolated shadow version control engine designed for AI coding agents.

### The Problem it Solves
Standard Git couples two distinct concepts:
1. **Working Directory State** (the files you are editing).
2. **Staging & Commit History** (the official Git branch and `.git/index`).

When AI agents work iteratively, they perform dozens of micro-edits. Recording these directly with `git commit` pollutes branch history with messy trial-and-error commits. Conversely, not recording them leaves no safety net if an agent breaks working code.

### The Solution: Shadow DAG
`edio` creates an invisible, branchless Git commit DAG stored entirely within custom Git namespaces (`refs/edio/*`) and a dedicated `.git/edio/` metadata store.

```
Main Git Branch (Clean)
● Commit 1 ─────────────────────────────────────────────► ● Commit 2 (Accepted)
                                                              ▲
Shadow DAG (Isolated from git log)                            │
  ● Turn 1 ──► ● Turn 2 ──► ● Turn 3 ──► ... ──► ● Turn N ────┘
  (refs/edio/active/sess_12345/*)
```

---

## 2. Directory & Package Structure

```text
.
├── cmd/
│   └── edio/               # CLI subcommands & entrypoint (Cobra)
│       ├── main.go         # Application main
│       ├── root.go         # Root command definition & bare execution launcher
│       ├── init.go         # 'edio init': configures hooks & EDIO.md
│       ├── snapshot.go     # 'edio snapshot': captures isolated turn
│       ├── run.go          # 'edio run': wraps command & auto-snapshots
│       ├── log.go          # 'edio log': displays turn history
│       ├── diff.go         # 'edio diff': CLI turn diff viewer
│       ├── restore.go      # 'edio restore': workspace & single-file rollback
│       ├── accept.go       # 'edio accept': squashes turns into branch commit
│       ├── gc.go           # 'edio gc': 10-day TTL session garbage collection
│       ├── ui.go           # 'edio ui': interactive split-pane TUI
│       └── mcp.go          # 'edio mcp': Model Context Protocol JSON-RPC server
│
├── pkg/
│   ├── gitengine/          # Low-level Git plumbing operations
│   │   ├── operations.go   # Git command runner, commit-tree, update-ref
│   │   └── tree.go         # Isolated tree snapshotting via GIT_INDEX_FILE
│   │
│   ├── session/            # Session lifecycle & DAG management
│   │   ├── session.go      # Session struct & RecordTurn logic
│   │   ├── store.go        # JSON persistence (.git/edio/active_session.json)
│   │   ├── history.go      # Turn history query & metadata parser
│   │   ├── archive.go      # Session archiving logic
│   │   └── gc.go           # Session-level TTL pruning engine
│   │
│   ├── tui/                # Interactive Terminal User Interface
│   │   ├── tui.go          # tview Application layout, focus & event routing
│   │   └── diff.go         # GitHub-style diff parser & Chroma syntax highlighter
│   │
│   └── ui/                 # CLI terminal formatting & color helpers
│       └── style.go        # ANSI terminal styles & status badges
```

---

## 3. Core Engine Mechanics

### A. Zero-Pollution Snapshotting (`pkg/gitengine/tree.go`)

Standard `git add` modifies `.git/index`. If a developer already had files staged for their own commit, running an agent snapshot would overwrite their staging area.

To avoid this, `edio` isolates the staging index:

1. **Creates a unique temporary index:**
   ```text
   .git/edio_index_<timestamp>_<random>.tmp
   ```
2. **Injects `GIT_INDEX_FILE` environment variable:**
   Forces all Git commands for that snapshot to read/write *only* to the temporary file.
3. **Reads current HEAD:**
   `git read-tree HEAD` populates the temporary index with base repository structure.
4. **Stages changes:**
   `git add -A` captures all additions, edits, and deletions in the working tree.
5. **Writes tree object:**
   `git write-tree` writes a pure Git tree object into `.git/objects/` and returns its SHA.
6. **Cleans up scratchpad:**
   The temporary index file is deleted immediately.

```
User Workspace ──► Staged in Temporary Index ──► git write-tree ──► Tree Object (SHA)
                       (GIT_INDEX_FILE)                                (In .git/objects)
                             │
                             ▼ (Deleted immediately)
                   Primary .git/index untouched!
```

---

### B. Shadow DAG Chaining (`pkg/session/session.go`)

Once a Tree SHA is generated, `session.RecordTurn()` creates a Git commit object:

1. **Uses `git commit-tree` (not `git commit`):**
   * Creates a raw commit object in `.git/objects/`.
   * Sets the previous turn's SHA as its parent commit (`-p <parent_sha>`).
   * Turn 1 links to the base repository HEAD commit (or universal empty tree SHA).
2. **Updates custom references:**
   * Writes `refs/edio/active/<session_id>/<turn_number>` pointing to the new commit.
   * Updates floating head `refs/edio/active/<session_id>/current`.
3. **Updates session metadata:**
   * Increments `TurnCount` and updates `LatestSHA`.
   * Atomically persists state to `.git/edio/active_session.json`.

---

### C. Atomic Squashing on Accept (`cmd/edio/accept.go`)

When `edio accept "<commit_message>"` is called:

1. **Extracts latest tree:**
   Retrieves the Tree SHA from `sess.LatestSHA^{tree}`.
2. **Creates official commit:**
   Runs `git commit-tree` with the latest tree and the active branch `HEAD` as its parent.
3. **Advances active branch:**
   Updates `refs/heads/<current_branch>` to point to the new commit.
4. **Syncs staging index:**
   Runs `git read-tree HEAD` so the user's working tree and staging index match the new commit.
5. **Archives session:**
   Moves active pointers to `refs/edio/archive/<timestamp>_<session_id>` and removes `.git/edio/active_session.json`.

```
                    [Turn 1] ──► [Turn 2] ──► [Turn 3] (Tree: abc1234)
                                                    │
                                      Extract Tree  │
                                                    ▼
[Branch HEAD] ─────────────────────────────► [Official Commit]
```

---

### D. 10-Day Session-Level Garbage Collection (`pkg/session/gc.go`)

To prevent shadow sessions from accumulating unbounded Git objects over months:

1. **Evaluates at the Session Boundary:**
   Never prunes individual turns from within an active session (which would break DAG parent chains).
2. **Protects Active Session:**
   The session currently registered in `.git/edio/active_session.json` is never deleted.
3. **10-Day TTL:**
   Scans `refs/edio/*`. If a session's latest turn commit timestamp is older than 10 days, all its reference pointers are deleted (`git update-ref -d`).
4. **Reclaims Disk Space:**
   Invokes `git pack-refs --all --prune` and `git prune --expire=<cutoff>` to let Git physically reclaim unreferenced storage.
5. **Automated Trigger:**
   Runs automatically in the background on every `edio accept` execution.

---

## 4. User Interfaces

### A. Terminal UI (`pkg/tui/`)
Built with **`rivo/tview`** and **`gdamore/tcell/v2`**:
* **2D Grid Layout:** Left Timeline (35% width), Right Diff Viewport (65% width), and Bottom Status Bar.
* **Diff Engine (`diff.go`):**
  * Strips raw Git plumbing headers (`diff --git`, `index`, `new file mode`).
  * Parses language syntax via **`chroma/v2`** with `github-dark` theme.
  * Padds additions (`+`) and deletions (`-`) with full-width background tints (`#16331c` / `#3d1a11`).
  * Binary file detection and 200KB payload freeze guardrail.
* **4-Way Scrolling:** `j`/`k`/`↑`/`↓` for vertical line navigation, `h`/`l`/`←`/`→` for horizontal column navigation.

### B. MCP Server (`cmd/edio/mcp.go`)
Implements JSON-RPC 2.0 over `stdin`/`stdout` conforming to the Model Context Protocol:
* `edio_snapshot`: AI agents take turn snapshots.
* `edio_log`: AI agents inspect past turn history.
* `edio_restore`: AI agents or developers rollback workspace to any turn.

---

## 5. Storage Layout on Disk

When `edio` is active in a repository, the `.git/` folder contains:

```text
.git/
├── edio/
│   ├── active_session.json    # JSON metadata for the currently active session
│   └── ...                    # Internal session scratchpads
│
├── refs/
│   └── edio/
│       ├── active/
│       │   └── <session_id>/
│       │       ├── 1          # Commit SHA for Turn 1
│       │       ├── 2          # Commit SHA for Turn 2
│       │       └── current    # Floating pointer to latest turn
│       │
│       └── archive/
│           └── <timestamp>_<session_id>  # Archived session tip commit
│
└── objects/                   # Standard Git blob, tree, and commit storage
```

---

## 6. Development & Testing Principles

* **Unit Isolation (`pkg/testutil`):** All unit tests execute in temporary Git repositories created via `mktemp -d`. Tests never touch your real repository or home directory.
* **Invariants Tested:**
  * `TestZeroPollution`: Verifies that taking 100 snapshots leaves `git status` and `.git/index` 100% clean.
  * `TestBuildIsolatedTree`: Verifies tree generation matches workspace files.
  * `TestSessionLifeCycle`: Verifies DAG parent chaining across turns.
  * `TestPruneExpiredSessions`: Verifies 10-day TTL pruning while protecting active sessions.
  * `TestFormatDiff_*`: Verifies binary detection and large payload truncation.

Run all tests:
```bash
make test
```
