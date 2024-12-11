package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"go.lsp.dev/jsonrpc2"

	lspenv "github.com/gnolang/gnopls/internal/env"
	"github.com/gnolang/gnopls/internal/lsp"
	"github.com/gnolang/gnopls/internal/version"
)

type buffer struct {
	*io.PipeWriter
	*io.PipeReader
}

func (b buffer) Close() error {
	b.PipeReader.Close()
	b.PipeWriter.Close()
	return nil
}

// TestScripts runs a gnopls server instance and uses the buffer type to
// communicate with it. Then it executes all the txtar files found in the
// testdata directory.
func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		UpdateScripts: os.Getenv("TXTAR_UPDATE") != "",
		Setup: func(env *testscript.Env) error {
			var (
				clientRead, serverWrite = io.Pipe()
				serverRead, clientWrite = io.Pipe()
				serverBuf               = buffer{
					PipeWriter: serverWrite,
					PipeReader: serverRead,
				}
				clientBuf = buffer{
					PipeWriter: clientWrite,
					PipeReader: clientRead,
				}
				serverConn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverBuf))
				procEnv    = &lspenv.Env{
					GNOROOT: os.Getenv("GNOROOT"),
					GNOHOME: lspenv.GnoHome(),
				}
				serverHandler = jsonrpc2.HandlerServer(lsp.BuildServerHandler(serverConn, procEnv))
				clientConn    = jsonrpc2.NewConn(jsonrpc2.NewStream(clientBuf))
			)
			env.Values["conn"] = clientConn

			// Start LSP server
			ctx := context.Background()
			go func() {
				if err := serverHandler.ServeStream(ctx, serverConn); !errors.Is(err, io.ErrClosedPipe) {
					env.T().Fatal("Server error", err)
				}
			}()
			// Listen to server notifications
			// All servers notifications are expected to be find in the txtar working
			// directory, using the following format:
			// $WORK/output/notify{i}.json
			// where {i} is a counter starting at 1.
			var notifyNum atomic.Uint32
			clientConn.Go(ctx, func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
				filename := fmt.Sprintf("notify%d.json", notifyNum.Add(1))
				return writeJSON(env, filename, req)
			})

			// Stop LSP server at the end of test
			env.Defer(func() {
				clientConn.Close()
				serverConn.Close()
				<-clientConn.Done()
				<-serverConn.Done()
			})

			// GNOPLS_WD is used in txtar files to know the location of the gnopls
			// working dir.
			env.Setenv("GNOPLS_WD", filepath.Join(lspenv.GnoHome(), "gnopls", "tmp"))
			// GNOPLS_VERSION is used in txtar files to know the server version.
			// This required because it changes over git status (can be "local" or
			// "v0.1.0-local"...)
			env.Setenv("GNOPLS_VERSION", version.GetVersion(ctx))

			return nil
		},
		Dir: "testdata",
		Cmds: map[string]func(*testscript.TestScript, bool, []string){
			// "lsp" is a txtar command that sends a lsp command to the server with
			// the following arguments:
			// - the method name
			// - the path to the file that contains the method parameters
			//
			// The server's response is encoded into the $WORK/output directory, with
			// filename equals to the parameter filename.
			// For example, the response of the following command:
			//    lsp initialized input/initialized.json
			// will be available in $WORK/output/initialized.json
			"lsp": func(ts *testscript.TestScript, neg bool, args []string) { //nolint:unparam
				if len(args) != 2 {
					ts.Fatalf("usage: lsp <method> <param_file>")
				}
				var (
					method     = args[0]
					paramsFile = args[1]
				)
				call(ts, method, paramsFile)
			},
		},
	})
}

// call decodes paramFile and send it to the server using method.
func call(ts *testscript.TestScript, method string, paramFile string) {
	paramStr := ts.ReadFile(paramFile)
	// Replace $WORK with real path
	paramStr = os.Expand(paramStr, func(key string) string {
		if strings.HasPrefix(key, "FILE_") {
			// ${FILE_filename} is a convenient directive in txtar files used to be
			// replaced by the content of the file "filename".
			// Since the prefix is "FILE_", key[5:] holds the filename.
			fileContent := ts.ReadFile(key[5:])
			// Escape fileContent for JSON format
			bz, err := json.Marshal(fileContent)
			if err != nil {
				ts.Fatalf("encode key %s %q: %v", key, fileContent, err)
			}
			return string(bz[1 : len(bz)-1]) // remove quote wrapping
		}
		return ts.Getenv(key)
	})
	var params any
	if err := json.Unmarshal([]byte(paramStr), &params); err != nil {
		ts.Fatalf("decode param file %s: %v", paramFile, err)
	}
	var (
		conn     = ts.Value("conn").(jsonrpc2.Conn) //nolint:errcheck
		response any
	)
	_, err := conn.Call(context.Background(), method, params, &response)
	if err != nil {
		response = map[string]any{"error": err}
	}
	if err := writeJSON(ts, filepath.Base(paramFile), response); err != nil {
		ts.Fatalf("writeJSON: %v", err)
	}
}

// writeJSON writes x to $WORK/output/filename
func writeJSON(ts interface{ Getenv(string) string }, filename string, x any) error {
	workDir := ts.Getenv("WORK")
	filename = filepath.Join(workDir, "output", filename)
	err := os.MkdirAll(filepath.Dir(filename), os.ModePerm)
	if err != nil {
		return err
	}
	bz, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		return err
	}
	bz = append(bz, '\n') // txtar files always have a final newline
	return os.WriteFile(filename, bz, os.ModePerm)
}
