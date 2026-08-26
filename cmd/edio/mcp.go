package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/devxdh/edio/pkg/gitengine"
	"github.com/devxdh/edio/pkg/session"
	"github.com/spf13/cobra"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type JSONRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

// MCP Protocol Data Types

type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type ToolCallResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the Model Context Protocol (MCP) server over stdio",
	Long: `Runs an MCP JSON-RPC 2.0 server over standard input and output.
Allows AI clients (Cursor, Claude Desktop, Windsurf) to natively invoke edio tools.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPServer()
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func runMCPServer() error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("mcp read error: %w", err)
		}

		if len(line) == 0 || (len(line) == 1 && line[0] == '\n') {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendRPCError(writer, nil, -32700, "Parse error")
			continue
		}

		handleRPCRequest(writer, req)
	}
}

func handleRPCRequest(w io.Writer, req JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		sendRPCResult(w, req.ID, map[string]any{
			"protocolVersion": "2024-11=05",
			"serverInfo": map[string]string{
				"name":    "edio-mcp",
				"version": "1.0.0",
			},
			"capabilities": map[string]any{
				"tools": map[string]bool{"listChanged": false},
			},
		})

	case "notifications/initialized":

	case "tools/list":
		sendRPCResult(w, req.ID, map[string]any{
			"tools": getToolDefinitions(),
		})

	case "tools/calls":
		var params ToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sendRPCError(w, req.ID, -32602, "Invalid parameters")
			return
		}

		result, isErr := executeTool(params.Name, params.Arguments)
		sendRPCResult(w, req.ID, ToolCallResult{
			Content: []ToolContent{{Type: "text", Text: result}},
			IsError: isErr,
		})

	default:
		sendRPCError(w, req.ID, -32601, fmt.Sprintf("Method '%s' not found", req.Method))
	}
}

func getToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "edio_snapshot",
			Description: "Take an isolated turn snapshot of all workspace modifications without touching git staging/commits.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"message": map[string]any{
						"type":        "string",
						"description": "Short summary of what code was modified/added in this turn",
					},
				},
				"required": []string{"message"},
			},
		},
		{
			Name:        "edio_log",
			Description: "Retrieve the chronological history of turns in the active edio session.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "edio_restore",
			Description: "Restore the working directory or a single file to a specific turn state.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"turn": map[string]any{
						"type":        "integer",
						"description": "The turn number to revert back to",
					},
					"file": map[string]any{
						"type":        "string",
						"description": "Optional: restore only this specific file path instead of full workspace",
					},
				},
				"required": []string{"turn"},
			},
		},
	}
}

func executeTool(name string, args map[string]any) (string, bool) {
	if err := gitengine.EnsureGitRepo(); err != nil {
		return fmt.Sprintf("Error: not inside git repo: %v", err), true
	}

	switch name {
	case "edio_snapshot":
		msg, _ := args["message"].(string)
		if msg == "" {
			msg = "mcp agent turn"
		}

		sess, err := session.LoadActiveSession()
		if err != nil {
			return fmt.Sprintf("Error loading session: %v", err), true
		}

		treeSHA, err := gitengine.BuildIsolatedTree()
		if err != nil {
			return fmt.Sprintf("Error building tree: %v", err), true
		}

		commitSHA, err := sess.RecordTurn(treeSHA, msg)
		if err != nil {
			return fmt.Sprintf("Error recording turn: %v", err), true
		}

		if err := session.SaveActiveSession(sess); err != nil {
			return fmt.Sprintf("Error saving session: %v", err), true
		}

		return fmt.Sprintf("[Turn %d] (%s) snapshot recorded successfully", sess.TurnCount, commitSHA[:7]), false

	case "edio_log":
		sess, err := session.LoadActiveSession()
		if err != nil {
			return fmt.Sprintf("Error loading session: %v", err), true
		}

		history, err := sess.GetTurnHistory()
		if err != nil {
			return fmt.Sprintf("Error querying history: %v", err), true
		}

		if len(history) == 0 {
			return "No turns recorded in active session.", false
		}

		out := fmt.Sprintf("Session: %s (%d turns)\n", sess.ID, sess.TurnCount)
		for _, rec := range history {
			out += fmt.Sprintf("* [Turn %d] (%s) %s\n", rec.Turn, rec.SHA[:7], rec.Message)
		}
		return out, false

	case "edio_restore":
		rawTurn, ok := args["turn"]
		if !ok {
			return "Error: missing required argument 'turn'", true
		}

		targetTurn := 0
		switch v := rawTurn.(type) {
		case float64:
			targetTurn = int(v)
		case int:
			targetTurn = v
		}

		sess, err := session.LoadActiveSession()
		if err != nil {
			return fmt.Sprintf("Error loading session: %v", err), true
		}

		if targetTurn < 1 || targetTurn > sess.TurnCount {
			return fmt.Sprintf("Error: turn %d does not exist (total turns: %d)", targetTurn, sess.TurnCount), true
		}

		targetRef := sess.ActiveRef(targetTurn)
		targetSHA, err := gitengine.GetRef(targetRef)
		if err != nil || targetSHA == "" {
			return fmt.Sprintf("Error: failed to resolve turn ref: %v", err), true
		}

		filePath, _ := args["file"].(string)
		if filePath != "" {
			_, err = gitengine.RunGit("checkout", targetSHA, "--", filePath)
			if err != nil {
				return fmt.Sprintf("Error restoring file: %v", err), true
			}
			return fmt.Sprintf("Restored file %s from [Turn %d] (%s)", filePath, targetTurn, targetSHA[:7]), false
		}

		_, err = gitengine.RunGit("checkout", targetSHA, "--", ".")
		if err != nil {
			return fmt.Sprintf("Error restoring workspace: %v", err), true
		}
		return fmt.Sprintf("Restored workspace to [Turn %d] (%s)", targetTurn, targetSHA[:7]), false

	default:
		return fmt.Sprintf("Unknown tool '%s'", name), true
	}
}

func sendRPCResult(w io.Writer, id any, result any) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	bytes, _ := json.Marshal(resp)
	_, _ = fmt.Fprintf(w, "%s\n", bytes)
}

func sendRPCError(w io.Writer, id any, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	bytes, _ := json.Marshal(resp)
	_, _ = fmt.Fprintf(w, "%s\n", bytes)
}
