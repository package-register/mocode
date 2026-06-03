package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/package-register/mocode/internal/app"
	"github.com/package-register/mocode/internal/permission"
	"github.com/package-register/mocode/internal/pubsub"
	"github.com/package-register/mocode/internal/session"
	"github.com/package-register/mocode/internal/session/message"
	"github.com/package-register/mocode/internal/workspace"
)

func (s *Server) handleWebSocketStream(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	if sessionID == "" {
		http.Error(w, "missing session_id", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("web: ws upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	slog.Info("web: ws connected", "session", sessionID)

	appWS, ok := s.workspace.(*workspace.AppWorkspace)
	if !ok {
		http.Error(w, "web stream only supports local AppWorkspace for now", http.StatusNotImplemented)
		return
	}
	appInstance := appWS.App()

	// Initial session status.
	sendJSON(conn, map[string]any{
		"jsonrpc": "2.0",
		"method":  "session_status",
		"params": map[string]any{
			"session_id": sessionID,
			"state":      "idle",
			"seq":        0,
			"updated_at": now,
		},
	})

	// Replay persisted history.
	replayHistory(conn, sessionID)

	// Frontend expects top-level history_complete.
	sendJSON(conn, map[string]any{
		"jsonrpc": "2.0",
		"method":  "history_complete",
		"id":      sessionID + "-history-complete",
	})

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	msgCh := appInstance.Messages.Subscribe(ctx)
	permReqCh := appInstance.Permissions.Subscribe(ctx)
	permNotifCh := appInstance.Permissions.SubscribeNotifications(ctx)

	// Event bridge goroutine.
	done := make(chan struct{})
	go func() {
		defer close(done)
		seq := 1
		for {
			select {
			case ev, ok := <-msgCh:
				if !ok {
					return
				}
				if ev.Payload.SessionID != sessionID {
					continue
				}
				s.bridgeMessageEvent(conn, ev, &seq, appInstance)
			case ev, ok := <-permReqCh:
				if !ok {
					return
				}
				s.bridgePermissionRequest(conn, ev)
			case ev, ok := <-permNotifCh:
				if !ok {
					return
				}
				s.bridgePermissionNotification(conn, ev)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Read loop: prompt / cancel / set_plan_mode.
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			cancel()
			<-done
			slog.Debug("web: ws closed", "session", sessionID, "error", err)
			break
		}

		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		method, _ := msg["method"].(string)

		// Handle JSON-RPC responses (no method field, only id+result).
		// The frontend sends these for approval and question responses.
		if method == "" {
			if result, ok := msg["result"].(map[string]any); ok {
				if reqID, ok := result["request_id"].(string); ok {
					perm := permission.PermissionRequest{ID: reqID}
					// Approval response: {request_id, response: "approve"|"approve_for_session"|"reject"}
					if resp, ok := result["response"].(string); ok {
						switch resp {
						case "approve":
							s.workspace.PermissionGrant(perm)
						case "approve_for_session":
							s.workspace.PermissionGrantPersistent(perm)
						case "reject":
							s.workspace.PermissionDeny(perm)
						default:
							slog.Warn("web: unknown approval response", "response", resp, "request_id", reqID)
						}
						sendJSON(conn, map[string]any{
							"jsonrpc": "2.0",
							"id":      msg["id"],
							"result":  map[string]any{"status": "ok"},
						})
						continue
					}
					// Question response: {request_id, answers: {...}}
					if _, ok := result["answers"]; ok {
						// TODO: wire up question response mechanism when AskUserQuestion tool is implemented.
						sendJSON(conn, map[string]any{
							"jsonrpc": "2.0",
							"id":      msg["id"],
							"result":  map[string]any{"status": "ok"},
						})
						continue
					}
				}
			}
		}

		switch method {
		case "initialize":
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result": map[string]any{
					"status":         "ok",
					"slash_commands": []any{},
				},
			})
			continue
		case "cancel":
			s.workspace.AgentCancel(sessionID)
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result":  map[string]any{},
			})
			// Notify frontend that the step was interrupted.
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"method":  "event",
				"params": map[string]any{
					"type":    "StepInterrupted",
					"payload": map[string]any{},
				},
			})
			continue
		case "set_plan_mode":
			// Store plan mode state per session and broadcast via StatusUpdate.
			params, _ := msg["params"].(map[string]any)
			enabled := false
			if v, ok := params["enabled"].(bool); ok {
				enabled = v
			}
			s.setPlanMode(sessionID, enabled)

			// Notify frontend of plan mode status.
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"method":  "event",
				"params": map[string]any{
					"type": "StatusUpdate",
					"payload": map[string]any{
						"plan_mode": enabled,
					},
				},
			})

			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result": map[string]any{
					"status":         "ok",
					"slash_commands": []any{},
				},
			})
			continue
		case "prompt":
			// Explicitly handle prompt method.
		}

		params, _ := msg["params"].(map[string]any)
		// Support both string and ContentPart array for user_input
		var userInput string
		if ui, ok := params["user_input"].(string); ok {
			userInput = ui
		} else if uiArr, ok := params["user_input"].([]any); ok {
			// Extract text from ContentPart array
			var parts []string
			for _, item := range uiArr {
				if part, ok := item.(map[string]any); ok {
					if text, ok := part["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
			userInput = strings.Join(parts, "\n")
		}
		if userInput == "" {
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"error": map[string]any{
					"code":    -32602,
					"message": "user_input must be a string in current implementation",
				},
			})
			continue
		}

		s.setRunning(sessionID, true)
		sendJSON(conn, map[string]any{
			"jsonrpc": "2.0",
			"method":  "session_status",
			"params": map[string]any{
				"session_id": sessionID,
				"state":      "busy",
				"seq":        1,
				"updated_at": time.Now().UTC().Format(time.RFC3339),
			},
		})

		err = s.workspace.AgentRun(r.Context(), sessionID, userInput)
		if err != nil {
			slog.Warn("web: agent run failed", "session", sessionID, "error", err)
			s.setRunning(sessionID, false)
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"error":   map[string]any{"code": -32000, "message": err.Error()},
			})
		} else {
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result": map[string]any{
					"status":         "ok",
					"slash_commands": getSlashCommands(),
				},
			})
		}
	}
}

func (s *Server) bridgeMessageEvent(conn *websocket.Conn, ev pubsub.Event[message.Message], seq *int, appInstance *app.App) {
	msg := ev.Payload
	now := time.Now().UTC().Format(time.RFC3339)

	switch ev.Type {
	case pubsub.CreatedEvent:
		if msg.Role == message.User {
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"method":  "event",
				"params": map[string]any{
					"type": "TurnBegin",
					"payload": map[string]any{
						"user_input": msg.Content().Text,
					},
				},
			})
			return
		}
		if msg.Role == message.Assistant {
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"method":  "event",
				"params": map[string]any{
					"type":    "StepBegin",
					"payload": map[string]any{"n": 1},
				},
			})
		}
	}

	if msg.Role == message.Assistant {
		thinking := msg.ReasoningContent().Thinking
		if thinking != "" {
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"method":  "event",
				"params": map[string]any{
					"type": "ContentPart",
					"payload": map[string]any{
						"type":  "think",
						"think": thinking,
					},
				},
			})
		}
		text := msg.Content().Text
		if text != "" {
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"method":  "event",
				"params": map[string]any{
					"type": "ContentPart",
					"payload": map[string]any{
						"type": "text",
						"text": text,
					},
				},
			})
		}
		for _, tc := range msg.ToolCalls() {
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"method":  "event",
				"params": map[string]any{
					"type": "ToolCall",
					"payload": map[string]any{
						"type": "function",
						"id":   tc.ID,
						"function": map[string]any{
							"name":      tc.Name,
							"arguments": tc.Input,
						},
					},
				},
			})
		}
		if msg.FinishPart() != nil {
			s.setRunning(msg.SessionID, false)
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"method":  "session_status",
				"params": map[string]any{
					"session_id": msg.SessionID,
					"state":      "idle",
					"seq":        *seq,
					"updated_at": now,
				},
			})
			*seq++

			// Send StatusUpdate with token usage from session.
			if appInstance != nil {
				if sess, err := appInstance.Sessions.Get(context.Background(), msg.SessionID); err == nil {
					var contextUsage any
					maxCtx := int64(0)
					if cfg := appInstance.Config(); cfg != nil {
						// Use the same model lookup as handleGetConfig.
						if m, ok := cfg.Models["large"]; ok {
							if prov := cfg.GetModel(m.Provider, m.Model); prov != nil {
								maxCtx = prov.ContextWindow
							}
						}
					}
					if maxCtx > 0 {
						total := sess.PromptTokens + sess.CompletionTokens
						contextUsage = float64(total) / float64(maxCtx)
						if contextUsage.(float64) > 1 {
							contextUsage = 1.0
						}
					} else {
						contextUsage = nil
					}
					sendJSON(conn, map[string]any{
						"jsonrpc": "2.0",
						"method":  "event",
						"params": map[string]any{
							"type": "StatusUpdate",
							"payload": map[string]any{
								"context_usage": contextUsage,
								"token_usage":   statusTokenUsage(sess),
							},
						},
					})
				}
			}
		}
		return
	}

	if msg.Role == message.Tool {
		for _, tr := range msg.ToolResults() {
			payload := map[string]any{
				"tool_call_id": tr.ToolCallID,
				"return_value": map[string]any{
					"is_error": tr.IsError,
					"output":   tr.Content,
					"message":  tr.Content,
					"display":  []any{},
				},
			}
			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"method":  "event",
				"params": map[string]any{
					"type":    "ToolResult",
					"payload": payload,
				},
			})
		}
	}
}

func (s *Server) bridgePermissionRequest(conn *websocket.Conn, ev pubsub.Event[permission.PermissionRequest]) {
	p := ev.Payload
	sendJSON(conn, map[string]any{
		"jsonrpc": "2.0",
		"method":  "event",
		"params": map[string]any{
			"type": "ApprovalRequest",
			"payload": map[string]any{
				"id":           p.ID,
				"action":       p.ToolName,
				"description":  p.Description,
				"sender":       "mocode",
				"tool_call_id": p.ToolCallID,
			},
		},
	})
}

func (s *Server) bridgePermissionNotification(conn *websocket.Conn, ev pubsub.Event[permission.PermissionNotification]) {
	n := ev.Payload
	response := any(false)
	if n.Granted && !n.Denied {
		response = true
	}
	sendJSON(conn, map[string]any{
		"jsonrpc": "2.0",
		"method":  "event",
		"params": map[string]any{
			"type": "ApprovalRequestResolved",
			"payload": map[string]any{
				"request_id": n.RequestID,
				"response":   response,
			},
		},
	})
}

// bridgeQuestionRequest sends a QuestionRequest event to the frontend.
//
// TODO: When the AskUserQuestion tool is implemented, it should publish
// QuestionRequest events via pubsub. Subscribe to that topic and call
// this function to bridge the events to the WebSocket.
//
// Frontend expects payload: {id, tool_call_id, questions: [{question, header, options, multi_select}]}
//
//lint:ignore U1000 — reserved for future use.
func (s *Server) bridgeQuestionRequest(conn *websocket.Conn, reqID, toolCallID string, questions []any) {
	sendJSON(conn, map[string]any{
		"jsonrpc": "2.0",
		"method":  "event",
		"params": map[string]any{
			"type": "QuestionRequest",
			"payload": map[string]any{
				"id":           reqID,
				"tool_call_id": toolCallID,
				"questions":    questions,
			},
		},
	})
}

// bridgeSubagentEvent sends a SubagentEvent wrapping an inner event from a
// sub-agent (Agent tool) to the frontend.
//
// TODO: When the Agent tool produces sub-agent events via pubsub, subscribe
// to that topic and call this function to forward nested events.
//
// Frontend expects payload: {parent_tool_call_id, agent_id?, subagent_type?, event: {type, payload}}
//
//lint:ignore U1000 — reserved for future use.
func (s *Server) bridgeSubagentEvent(conn *websocket.Conn, parentToolCallID, agentID, subagentType string, innerType string, innerPayload any) {
	sendJSON(conn, map[string]any{
		"jsonrpc": "2.0",
		"method":  "event",
		"params": map[string]any{
			"type": "SubagentEvent",
			"payload": map[string]any{
				"parent_tool_call_id": parentToolCallID,
				"agent_id":            agentID,
				"subagent_type":       subagentType,
				"event": map[string]any{
					"type":    innerType,
					"payload": innerPayload,
				},
			},
		},
	})
}

func replayHistory(conn *websocket.Conn, sessionID string) {
	workDir := getStartupDir()
	candidates := []string{
		filepath.Join(workDir, ".mocode", "sessions", sessionID, "wire.jsonl"),
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var record struct {
				Message json.RawMessage `json:"message"`
				Type    string          `json:"type"`
			}
			if err := json.Unmarshal([]byte(line), &record); err != nil {
				continue
			}
			if record.Type == "metadata" {
				continue
			}

			sendJSON(conn, map[string]any{
				"jsonrpc": "2.0",
				"method":  "event",
				"params":  json.RawMessage(record.Message),
			})
		}
		break
	}
}

func statusTokenUsage(sess session.Session) map[string]any {
	inputOther := sess.PromptTokens - sess.CacheReadTokens - sess.CacheCreationTokens
	if inputOther < 0 {
		inputOther = 0
	}
	return map[string]any{
		"input_other":          inputOther,
		"output":               sess.CompletionTokens,
		"input_cache_read":     sess.CacheReadTokens,
		"input_cache_creation": sess.CacheCreationTokens,
	}
}

// getSlashCommands returns the list of available slash commands for the frontend.
func getSlashCommands() []map[string]any {
	return []map[string]any{
		{"name": "help", "description": "Show help", "aliases": []string{"?"}},
		{"name": "clear", "description": "Clear the conversation", "aliases": []string{"reset"}},
		{"name": "compact", "description": "Compact/summarize the conversation", "aliases": []string{}},
	}
}

func sendJSON(conn *websocket.Conn, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, data)
}
