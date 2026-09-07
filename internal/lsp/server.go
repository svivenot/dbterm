package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"dbterm/internal/db"
)

// Server implements a full Language Server Protocol (LSP) server for SQL & Database Schema
type Server struct {
	mu        sync.RWMutex
	docs      map[string]string
	catalog   *Catalog
	completer *Completer
}

// NewServer creates a new LSP server instance
func NewServer() *Server {
	catalog := NewCatalog()
	completer := NewCompleter(catalog)

	return &Server{
		docs:      make(map[string]string),
		catalog:   catalog,
		completer: completer,
	}
}

// GetCatalog returns the internal schema catalog
func (s *Server) GetCatalog() *Catalog {
	return s.catalog
}

// GetCompleter returns the completer engine
func (s *Server) GetCompleter() *Completer {
	return s.completer
}

// UpdateSchema updates the internal catalog from an active database driver
func (s *Server) UpdateSchema(ctx context.Context, driver db.Driver) error {
	return s.catalog.UpdateFromDriver(ctx, driver)
}

// SetDocument stores or updates an in-memory document
func (s *Server) SetDocument(uri, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs[uri] = content
}

// GetDocument retrieves the content of an in-memory document
func (s *Server) GetDocument(uri string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	content, ok := s.docs[uri]
	return content, ok
}

// Complete returns completions directly for a given query and position
func (s *Server) Complete(uri string, sql string, line int, char int) []CompletionItem {
	if sql == "" && uri != "" {
		if doc, ok := s.GetDocument(uri); ok {
			sql = doc
		}
	}
	return s.completer.Complete(sql, line, char)
}

// Hover returns hover information directly for a given query and position
func (s *Server) Hover(uri string, sql string, line int, char int) *Hover {
	if sql == "" && uri != "" {
		if doc, ok := s.GetDocument(uri); ok {
			sql = doc
		}
	}
	return s.completer.Hover(sql, line, char)
}

// HandleMessage parses and executes a standard JSON-RPC 2.0 LSP message
func (s *Server) HandleMessage(raw []byte) ([]byte, error) {
	var req RequestMessage
	if err := json.Unmarshal(raw, &req); err != nil {
		return s.formatErrorResponse(nil, -32700, "Parse error")
	}

	switch req.Method {
	case "initialize":
		var params InitializeParams
		_ = json.Unmarshal(req.Params, &params)
		result := InitializeResult{
			Capabilities: ServerCapabilities{
				CompletionProvider: &CompletionOptions{
					ResolveProvider:   false,
					TriggerCharacters: []string{".", " ", "(", "\t"},
				},
				HoverProvider:         true,
				SignatureHelpProvider: true,
				TextDocumentSync:      1, // Full
			},
		}
		return s.formatResultResponse(req.ID, result)

	case "initialized":
		return nil, nil // Notification

	case "textDocument/didOpen":
		var params struct {
			TextDocument TextDocumentItem `json:"textDocument"`
		}
		if err := json.Unmarshal(req.Params, &params); err == nil {
			s.SetDocument(params.TextDocument.URI, params.TextDocument.Text)
		}
		return nil, nil

	case "textDocument/didChange":
		var params struct {
			TextDocument VersionedTextDocumentIdentifier `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		if err := json.Unmarshal(req.Params, &params); err == nil && len(params.ContentChanges) > 0 {
			s.SetDocument(params.TextDocument.URI, params.ContentChanges[len(params.ContentChanges)-1].Text)
		}
		return nil, nil

	case "textDocument/didClose":
		var params struct {
			TextDocument TextDocumentIdentifier `json:"textDocument"`
		}
		if err := json.Unmarshal(req.Params, &params); err == nil {
			s.mu.Lock()
			delete(s.docs, params.TextDocument.URI)
			s.mu.Unlock()
		}
		return nil, nil

	case "textDocument/completion":
		var params CompletionParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.formatErrorResponse(req.ID, -32602, "Invalid params")
		}

		doc, _ := s.GetDocument(params.TextDocument.URI)
		items := s.Complete(params.TextDocument.URI, doc, params.Position.Line, params.Position.Character)

		list := CompletionList{
			IsIncomplete: false,
			Items:        items,
		}
		return s.formatResultResponse(req.ID, list)

	case "textDocument/hover":
		var params HoverParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.formatErrorResponse(req.ID, -32602, "Invalid params")
		}

		doc, _ := s.GetDocument(params.TextDocument.URI)
		hover := s.Hover(params.TextDocument.URI, doc, params.Position.Line, params.Position.Character)
		return s.formatResultResponse(req.ID, hover)

	case "textDocument/signatureHelp":
		var params SignatureHelpParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.formatErrorResponse(req.ID, -32602, "Invalid params")
		}

		doc, _ := s.GetDocument(params.TextDocument.URI)
		lines := strings.Split(doc, "\n")
		var lineText string
		if params.Position.Line < len(lines) {
			lineText = lines[params.Position.Line]
		}

		sigHelp := s.getSignatureHelp(lineText, params.Position.Character)
		return s.formatResultResponse(req.ID, sigHelp)

	case "shutdown":
		return s.formatResultResponse(req.ID, nil)

	case "exit":
		return nil, nil

	default:
		return s.formatErrorResponse(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func (s *Server) getSignatureHelp(lineText string, char int) *SignatureHelp {
	if char > len(lineText) {
		char = len(lineText)
	}
	text := lineText[:char]
	lastOpenParen := strings.LastIndex(text, "(")
	if lastOpenParen == -1 {
		return nil
	}

	beforeParen := strings.TrimRight(text[:lastOpenParen], " \t")
	funcName := s.completer.getWordBeforeCursor(beforeParen)
	if funcName == "" {
		return nil
	}

	for _, fn := range standardFunctions {
		if strings.EqualFold(fn.Name, funcName) {
			return &SignatureHelp{
				Signatures: []SignatureInformation{
					{
						Label: fn.Signature,
						Documentation: &MarkupContent{
							Kind:  Markdown,
							Value: fn.Description,
						},
					},
				},
				ActiveSignature: 0,
				ActiveParameter: 0,
			}
		}
	}

	return nil
}

func (s *Server) formatResultResponse(id any, result any) ([]byte, error) {
	resp := ResponseMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	return json.Marshal(resp)
}

func (s *Server) formatErrorResponse(id any, code int, msg string) ([]byte, error) {
	resp := ResponseMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &ResponseError{
			Code:    code,
			Message: msg,
		},
	}
	return json.Marshal(resp)
}
