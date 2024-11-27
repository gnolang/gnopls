package lsp

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/gnolang/gnopls/internal/env"
	"github.com/gnolang/gnopls/internal/version"
)

type server struct {
	conn jsonrpc2.Conn
	env  *env.Env

	snapshot        *Snapshot
	completionStore *CompletionStore
	cache           *Cache

	initialized atomic.Bool
}

func BuildServerHandler(conn jsonrpc2.Conn, e *env.Env) jsonrpc2.Handler {
	dirs := []string{}
	if e.GNOROOT != "" {
		dirs = append(dirs, filepath.Join(e.GNOROOT, "examples"))
		dirs = append(dirs, filepath.Join(e.GNOROOT, "gnovm/stdlibs"))
	}
	server := &server{
		conn: conn,

		env: e,

		snapshot:        NewSnapshot(),
		completionStore: InitCompletionStore(dirs),
		cache:           NewCache(),
	}
	env.GlobalEnv = e
	return jsonrpc2.ReplyHandler(server.ServerHandler)
}

func (s *server) ServerHandler(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	if req.Method() == protocol.MethodInitialize {
		err := s.Initialize(ctx, reply, req)
		if err != nil {
			return err
		}
		s.initialized.Store(true)
		return nil
	}
	if !s.initialized.Load() {
		return replyErr(ctx, reply, jsonrpc2.NewError(jsonrpc2.ServerNotInitialized, "server not initialized"))
	}
	switch req.Method() {
	case protocol.MethodExit:
		return s.Exit(ctx, reply, req)
	case protocol.MethodInitialized:
		return s.Initialized(ctx, reply, req)
	case protocol.MethodShutdown:
		return s.Shutdown(ctx, reply, req)
	case protocol.MethodTextDocumentDidChange:
		return s.DidChange(ctx, reply, req)
	case protocol.MethodTextDocumentDidClose:
		return s.DidClose(ctx, reply, req)
	case protocol.MethodTextDocumentDidOpen:
		return s.DidOpen(ctx, reply, req)
	case protocol.MethodTextDocumentDidSave:
		return s.DidSave(ctx, reply, req)
	case protocol.MethodTextDocumentFormatting:
		return s.Formatting(ctx, reply, req)
	case protocol.MethodTextDocumentHover:
		return s.Hover(ctx, reply, req)
	case protocol.MethodTextDocumentCompletion:
		return s.Completion(ctx, reply, req)
	case protocol.MethodTextDocumentDefinition:
		return s.Definition(ctx, reply, req)
	default:
		return jsonrpc2.MethodNotFoundHandler(ctx, reply, req)
	}
}

func (s *server) Initialize(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.InitializeParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return sendParseError(ctx, reply, err)
	}

	return reply(ctx, protocol.InitializeResult{
		ServerInfo: &protocol.ServerInfo{
			Name:    "gnopls",
			Version: version.GetVersion(ctx),
		},
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				Change:    protocol.TextDocumentSyncKindFull,
				OpenClose: true,
				Save: &protocol.SaveOptions{
					IncludeText: true,
				},
			},
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{"."},
				ResolveProvider:   false,
			},
			HoverProvider: true,
			ExecuteCommandProvider: &protocol.ExecuteCommandOptions{
				Commands: []string{
					"gnopls.version",
				},
			},
			DefinitionProvider:         true,
			DocumentFormattingProvider: true,
		},
	}, nil)
}

func (s *server) Initialized(ctx context.Context, reply jsonrpc2.Replier, _ jsonrpc2.Request) error {
	slog.Info("initialized")
	return reply(ctx, nil, nil)
}

func (s *server) Shutdown(ctx context.Context, reply jsonrpc2.Replier, _ jsonrpc2.Request) error {
	slog.Info("shutdown")
	return reply(ctx, nil, s.conn.Close())
}

func (s *server) Exit(ctx context.Context, reply jsonrpc2.Replier, _ jsonrpc2.Request) error {
	slog.Info("exit")
	os.Exit(1)
	return nil
}
