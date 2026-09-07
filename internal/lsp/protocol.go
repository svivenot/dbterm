package lsp

import "encoding/json"

// LSP standard position (0-indexed line and character)
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// LSP standard range
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// TextDocumentIdentifier identifies a text document
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// VersionedTextDocumentIdentifier identifies a specific version of a document
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// TextDocumentItem represents an open text document
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// TextDocumentPositionParams contains document and position
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// CompletionTriggerKind represents how completion was triggered
type CompletionTriggerKind int

const (
	CompletionTriggerInvoked                         CompletionTriggerKind = 1
	CompletionTriggerCharacter                       CompletionTriggerKind = 2
	CompletionTriggerForIncompleteCompletions        CompletionTriggerKind = 3
)

// CompletionContext provides additional context for completion request
type CompletionContext struct {
	TriggerKind      CompletionTriggerKind `json:"triggerKind"`
	TriggerCharacter string                `json:"triggerCharacter,omitempty"`
}

// CompletionParams parameters for textDocument/completion
type CompletionParams struct {
	TextDocumentPositionParams
	Context *CompletionContext `json:"context,omitempty"`
}

// CompletionItemKind represents the kind of completion item
type CompletionItemKind int

const (
	CompletionItemKindText          CompletionItemKind = 1
	CompletionItemKindMethod        CompletionItemKind = 2
	CompletionItemKindFunction      CompletionItemKind = 3
	CompletionItemKindConstructor   CompletionItemKind = 4
	CompletionItemKindField         CompletionItemKind = 5 // Column
	CompletionItemKindVariable      CompletionItemKind = 6
	CompletionItemKindClass         CompletionItemKind = 7 // Table
	CompletionItemKindInterface     CompletionItemKind = 8 // View
	CompletionItemKindModule        CompletionItemKind = 9 // Schema / DB
	CompletionItemKindProperty      CompletionItemKind = 10
	CompletionItemKindUnit          CompletionItemKind = 11
	CompletionItemKindValue         CompletionItemKind = 12
	CompletionItemKindEnum          CompletionItemKind = 13
	CompletionItemKindKeyword       CompletionItemKind = 14 // SQL Keyword
	CompletionItemKindSnippet       CompletionItemKind = 15 // SQL Snippet
	CompletionItemKindColor         CompletionItemKind = 16
	CompletionItemKindFile          CompletionItemKind = 17
	CompletionItemKindReference     CompletionItemKind = 18
	CompletionItemKindFolder        CompletionItemKind = 19
	CompletionItemKindEnumMember    CompletionItemKind = 20
	CompletionItemKindConstant      CompletionItemKind = 21
	CompletionItemKindStruct        CompletionItemKind = 22
	CompletionItemKindEvent         CompletionItemKind = 23
	CompletionItemKindOperator      CompletionItemKind = 24
	CompletionItemKindTypeParameter CompletionItemKind = 25
)

// TextEdit represents a text edit
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// MarkupKind represents markup format
type MarkupKind string

const (
	PlainText MarkupKind = "plaintext"
	Markdown  MarkupKind = "markdown"
)

// MarkupContent represents documentation content
type MarkupContent struct {
	Kind  MarkupKind `json:"kind"`
	Value string     `json:"value"`
}

// CompletionItem represents an individual completion suggestion
type CompletionItem struct {
	Label            string              `json:"label"`
	Kind             CompletionItemKind  `json:"kind,omitempty"`
	Detail           string              `json:"detail,omitempty"`
	Documentation    *MarkupContent      `json:"documentation,omitempty"`
	SortText         string              `json:"sortText,omitempty"`
	FilterText       string              `json:"filterText,omitempty"`
	InsertText       string              `json:"insertText,omitempty"`
	InsertTextFormat int                 `json:"insertTextFormat,omitempty"`
	TextEdit         *TextEdit           `json:"textEdit,omitempty"`
	Data             any                 `json:"data,omitempty"`
}

// CompletionList represents completion results
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// HoverParams parameters for textDocument/hover
type HoverParams struct {
	TextDocumentPositionParams
}

// Hover represents hover response
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// SignatureHelpParams parameters for textDocument/signatureHelp
type SignatureHelpParams struct {
	TextDocumentPositionParams
}

// ParameterInformation represents a function parameter
type ParameterInformation struct {
	Label         string         `json:"label"`
	Documentation *MarkupContent `json:"documentation,omitempty"`
}

// SignatureInformation represents a function signature
type SignatureInformation struct {
	Label         string                 `json:"label"`
	Documentation *MarkupContent         `json:"documentation,omitempty"`
	Parameters    []ParameterInformation `json:"parameters,omitempty"`
}

// SignatureHelp represents signature help response
type SignatureHelp struct {
	Signatures      []SignatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

// InitializeParams parameters for initialize request
type InitializeParams struct {
	ProcessID int    `json:"processId,omitempty"`
	RootURI   string `json:"rootUri,omitempty"`
}

// CompletionOptions completion server capabilities
type CompletionOptions struct {
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// ServerCapabilities capabilities provided by this LSP server
type ServerCapabilities struct {
	CompletionProvider    *CompletionOptions `json:"completionProvider,omitempty"`
	HoverProvider         bool               `json:"hoverProvider,omitempty"`
	SignatureHelpProvider bool               `json:"signatureHelpProvider,omitempty"`
	TextDocumentSync      int                `json:"textDocumentSync,omitempty"` // 1 = Full
}

// InitializeResult response for initialize request
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

// JSON-RPC 2.0 structures
type RequestMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type ResponseMessage struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id"`
	Result  any            `json:"result,omitempty"`
	Error   *ResponseError `json:"error,omitempty"`
}

type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type NotificationMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}
