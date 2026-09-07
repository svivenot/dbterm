package lsp

import (
	"context"

	"dbterm/internal/db"
)

// Client provides a high-level API for editor autocompletion and hover features
type Client struct {
	server *Server
}

// NewClient creates a new LSP client connected to an embedded LSP server
func NewClient(server *Server) *Client {
	if server == nil {
		server = NewServer()
	}
	return &Client{server: server}
}

// GetServer returns the underlying LSP server
func (c *Client) GetServer() *Server {
	return c.server
}

// UpdateSchema triggers catalog update from the active database driver
func (c *Client) UpdateSchema(ctx context.Context, driver db.Driver) error {
	return c.server.UpdateSchema(ctx, driver)
}

// GetCompletions fetches autocompletion suggestions for the specified position
func (c *Client) GetCompletions(uri, sql string, line, char int) []CompletionItem {
	return c.server.Complete(uri, sql, line, char)
}

// GetHover fetches hover documentation for the token at the specified position
func (c *Client) GetHover(uri, sql string, line, char int) *Hover {
	return c.server.Hover(uri, sql, line, char)
}

// SetDocument syncs document text
func (c *Client) SetDocument(uri, text string) {
	c.server.SetDocument(uri, text)
}
