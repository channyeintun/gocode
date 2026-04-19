package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultLSPMaxResults = 100

type LSPTool struct{}

type lspRequest struct {
	Operation          string
	FilePath           string
	Line               int
	Column             int
	Query              string
	IncludeDeclaration bool
	MaxResults         int
	SearchPath         string
}

type lspServerConfig struct {
	Name       string
	Command    string
	Args       []string
	LanguageID string
	Extensions map[string]struct{}
	Markers    []string
}

type lspClient struct {
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       *bufio.Reader
	writeMu      sync.Mutex
	nextID       int64
	workspaceDir string
	server       lspServerConfig
	stderr       bytes.Buffer
}

type lspResponseEnvelope struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *lspRPCError    `json:"error,omitempty"`
}

type lspRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

type lspLocation struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

type lspLocationLink struct {
	TargetURI            string   `json:"targetUri"`
	TargetRange          lspRange `json:"targetRange"`
	TargetSelectionRange lspRange `json:"targetSelectionRange"`
}

type lspMarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type lspMarkedString struct {
	Language string `json:"language"`
	Value    string `json:"value"`
}

type lspHover struct {
	Contents any       `json:"contents"`
	Range    *lspRange `json:"range,omitempty"`
}

type lspDocumentSymbol struct {
	Name           string              `json:"name"`
	Detail         string              `json:"detail,omitempty"`
	Kind           int                 `json:"kind"`
	Range          lspRange            `json:"range"`
	SelectionRange lspRange            `json:"selectionRange"`
	Children       []lspDocumentSymbol `json:"children,omitempty"`
}

type lspSymbolInformation struct {
	Name          string      `json:"name"`
	Kind          int         `json:"kind"`
	Location      lspLocation `json:"location"`
	ContainerName string      `json:"containerName,omitempty"`
}

type lspWorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type lspOutput struct {
	Operation   string         `json:"operation"`
	FilePath    string         `json:"filePath,omitempty"`
	Workspace   string         `json:"workspace,omitempty"`
	ResultCount int            `json:"resultCount"`
	Results     []lspResultRow `json:"results"`
}

type lspResultRow struct {
	Kind          string `json:"kind"`
	Name          string `json:"name,omitempty"`
	Detail        string `json:"detail,omitempty"`
	SymbolKind    string `json:"symbolKind,omitempty"`
	FilePath      string `json:"filePath,omitempty"`
	Line          int    `json:"line,omitempty"`
	Column        int    `json:"column,omitempty"`
	EndLine       int    `json:"endLine,omitempty"`
	EndColumn     int    `json:"endColumn,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	Contents      string `json:"contents,omitempty"`
	Signature     string `json:"signature,omitempty"`
}

var lspServerConfigs = []lspServerConfig{
	{
		Name:       "gopls",
		Command:    "gopls",
		LanguageID: "go",
		Extensions: map[string]struct{}{".go": {}},
		Markers:    []string{"go.work", "go.mod", ".git"},
	},
	{
		Name:       "typescript-language-server",
		Command:    "typescript-language-server",
		Args:       []string{"--stdio"},
		LanguageID: "typescript",
		Extensions: map[string]struct{}{".ts": {}, ".tsx": {}, ".js": {}, ".jsx": {}, ".mjs": {}, ".cjs": {}},
		Markers:    []string{"tsconfig.json", "jsconfig.json", "package.json", ".git"},
	},
}

func NewLSPTool() *LSPTool {
	return &LSPTool{}
}

func (t *LSPTool) Name() string {
	return "lsp"
}

func (t *LSPTool) Description() string {
	return "Use a local Language Server Protocol server for semantic code intelligence including definitions, references, hover, document symbols, workspace symbols, and implementations."
}

func (t *LSPTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type": "string",
				"enum": []string{
					"go_to_definition",
					"find_references",
					"hover",
					"document_symbols",
					"workspace_symbols",
					"go_to_implementation",
				},
				"description": "The LSP operation to perform.",
			},
			"filePath": map[string]any{
				"type":        "string",
				"description": "Absolute or workspace-relative path for file-based operations.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Optional workspace path hint for workspace_symbols.",
			},
			"line": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "1-based line number for position-based operations.",
			},
			"column": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "1-based column number for position-based operations.",
			},
			"character": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Compatibility alias for column.",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Workspace symbol query for workspace_symbols.",
			},
			"includeDeclaration": map[string]any{
				"type":        "boolean",
				"description": "Whether declaration sites are included in find_references.",
			},
			"include_declaration": map[string]any{
				"type":        "boolean",
				"description": "Snake_case alias for includeDeclaration.",
			},
			"maxResults": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Optional maximum number of results to return.",
			},
			"max_results": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Snake_case alias for maxResults.",
			},
		},
		"required": []string{"operation"},
	}
}

func (t *LSPTool) Permission() PermissionLevel {
	return PermissionReadOnly
}

func (t *LSPTool) Concurrency(input ToolInput) ConcurrencyDecision {
	return ConcurrencyParallel
}

func (t *LSPTool) Validate(input ToolInput) error {
	_, err := parseLSPRequest(input.Params)
	return err
}

func (t *LSPTool) Execute(ctx context.Context, input ToolInput) (ToolOutput, error) {
	request, err := parseLSPRequest(input.Params)
	if err != nil {
		return ToolOutput{}, err
	}
	client, err := newLSPClient(ctx, request)
	if err != nil {
		return ToolOutput{}, err
	}
	defer client.Close()

	rows, err := client.Run(ctx, request)
	if err != nil {
		return ToolOutput{}, err
	}

	output := lspOutput{
		Operation:   request.Operation,
		FilePath:    request.FilePath,
		Workspace:   client.workspaceDir,
		ResultCount: len(rows),
		Results:     rows,
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return ToolOutput{}, fmt.Errorf("marshal lsp output: %w", err)
	}
	return ToolOutput{Output: string(encoded)}, nil
}

func parseLSPRequest(params map[string]any) (lspRequest, error) {
	operation, ok := firstStringParam(params, "operation")
	if !ok || strings.TrimSpace(operation) == "" {
		return lspRequest{}, fmt.Errorf("lsp requires operation")
	}
	request := lspRequest{
		Operation:          normalizeLSPOperation(operation),
		MaxResults:         firstPositiveIntOrDefault(params, defaultLSPMaxResults, "maxResults", "max_results"),
		IncludeDeclaration: firstBoolParam(params, "includeDeclaration", "include_declaration"),
	}
	if filePath, ok := firstStringParam(params, "filePath", "path"); ok && strings.TrimSpace(filePath) != "" {
		request.FilePath = strings.TrimSpace(filePath)
	}
	if searchPath, ok := stringParam(params, "path"); ok && strings.TrimSpace(searchPath) != "" {
		request.SearchPath = strings.TrimSpace(searchPath)
	}
	if query, ok := stringParam(params, "query"); ok {
		request.Query = strings.TrimSpace(query)
	}
	if line, ok := firstIntParam(params, "line"); ok {
		request.Line = line
	}
	if column, ok := firstIntParam(params, "column", "character"); ok {
		request.Column = column
	}

	switch request.Operation {
	case "go_to_definition", "find_references", "hover", "go_to_implementation":
		if request.FilePath == "" {
			return lspRequest{}, fmt.Errorf("lsp %s requires filePath", request.Operation)
		}
		if request.Line < 1 || request.Column < 1 {
			return lspRequest{}, fmt.Errorf("lsp %s requires positive line and column", request.Operation)
		}
	case "document_symbols":
		if request.FilePath == "" {
			return lspRequest{}, fmt.Errorf("lsp document_symbols requires filePath")
		}
	case "workspace_symbols":
		if request.Query == "" {
			return lspRequest{}, fmt.Errorf("lsp workspace_symbols requires query")
		}
	default:
		return lspRequest{}, fmt.Errorf("unsupported lsp operation %q", operation)
	}

	if request.FilePath != "" {
		resolvedPath, err := resolveToolPath(request.FilePath)
		if err != nil {
			return lspRequest{}, err
		}
		info, err := os.Stat(resolvedPath)
		if err != nil {
			return lspRequest{}, fmt.Errorf("stat file %q: %w", resolvedPath, err)
		}
		if info.IsDir() {
			return lspRequest{}, fmt.Errorf("%q is a directory", resolvedPath)
		}
		request.FilePath = resolvedPath
	}
	if request.SearchPath != "" {
		resolvedSearchPath, err := resolveToolPath(request.SearchPath)
		if err != nil {
			return lspRequest{}, err
		}
		request.SearchPath = resolvedSearchPath
	}
	return request, nil
}

func normalizeLSPOperation(value string) string {
	switch strings.TrimSpace(value) {
	case "go_to_definition", "goToDefinition":
		return "go_to_definition"
	case "find_references", "findReferences":
		return "find_references"
	case "hover":
		return "hover"
	case "document_symbols", "documentSymbol":
		return "document_symbols"
	case "workspace_symbols", "workspaceSymbol":
		return "workspace_symbols"
	case "go_to_implementation", "goToImplementation":
		return "go_to_implementation"
	default:
		return strings.TrimSpace(value)
	}
}

func newLSPClient(ctx context.Context, request lspRequest) (*lspClient, error) {
	server, workspaceDir, err := resolveLSPServerConfig(request)
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath(server.Command); err != nil {
		return nil, fmt.Errorf("lsp server %q is not installed or not on PATH", server.Command)
	}

	cmd := exec.CommandContext(ctx, server.Command, server.Args...)
	cmd.Dir = workspaceDir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open stdin for %s: %w", server.Name, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open stdout for %s: %w", server.Name, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("open stderr for %s: %w", server.Name, err)
	}

	client := &lspClient{
		cmd:          cmd,
		stdin:        stdin,
		stdout:       bufio.NewReader(stdout),
		workspaceDir: workspaceDir,
		server:       server,
	}
	go func() {
		_, _ = io.Copy(&client.stderr, stderr)
	}()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", server.Name, err)
	}
	if err := client.initialize(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *lspClient) Run(ctx context.Context, request lspRequest) ([]lspResultRow, error) {
	if request.FilePath != "" {
		if err := c.didOpen(ctx, request.FilePath); err != nil {
			return nil, err
		}
	}

	switch request.Operation {
	case "go_to_definition":
		var rawResult any
		if err := c.request(ctx, "textDocument/definition", c.textDocumentPositionParams(request), &rawResult); err != nil {
			return nil, err
		}
		return limitLSPResults(locationRowsFromAny(rawResult, "definition"), request.MaxResults), nil
	case "find_references":
		var locations []lspLocation
		params := c.textDocumentPositionParams(request)
		params["context"] = map[string]any{"includeDeclaration": request.IncludeDeclaration}
		if err := c.request(ctx, "textDocument/references", params, &locations); err != nil {
			return nil, err
		}
		return limitLSPResults(rowsFromLocations(locations, "reference"), request.MaxResults), nil
	case "hover":
		var hover lspHover
		if err := c.request(ctx, "textDocument/hover", c.textDocumentPositionParams(request), &hover); err != nil {
			return nil, err
		}
		rows := make([]lspResultRow, 0, 1)
		contents := strings.TrimSpace(extractHoverContents(hover.Contents))
		if contents != "" {
			row := lspResultRow{Kind: "hover", Contents: contents}
			if hover.Range != nil {
				applyRange(&row, hover.Range)
			}
			rows = append(rows, row)
		}
		return rows, nil
	case "document_symbols":
		var rawResult json.RawMessage
		if err := c.request(ctx, "textDocument/documentSymbol", map[string]any{
			"textDocument": map[string]any{"uri": pathToFileURI(request.FilePath)},
		}, &rawResult); err != nil {
			return nil, err
		}
		return limitLSPResults(documentSymbolRows(rawResult, request.FilePath), request.MaxResults), nil
	case "workspace_symbols":
		var symbols []lspSymbolInformation
		if err := c.request(ctx, "workspace/symbol", map[string]any{"query": request.Query}, &symbols); err != nil {
			return nil, err
		}
		return limitLSPResults(rowsFromWorkspaceSymbols(symbols), request.MaxResults), nil
	case "go_to_implementation":
		var rawResult any
		if err := c.request(ctx, "textDocument/implementation", c.textDocumentPositionParams(request), &rawResult); err != nil {
			return nil, err
		}
		return limitLSPResults(locationRowsFromAny(rawResult, "implementation"), request.MaxResults), nil
	default:
		return nil, fmt.Errorf("unsupported lsp operation %q", request.Operation)
	}
}

func (c *lspClient) initialize(ctx context.Context) error {
	var result map[string]any
	params := map[string]any{
		"processId": os.Getpid(),
		"rootUri":   pathToFileURI(c.workspaceDir),
		"rootPath":  c.workspaceDir,
		"clientInfo": map[string]any{
			"name": "nami",
		},
		"workspaceFolders": []lspWorkspaceFolder{{
			URI:  pathToFileURI(c.workspaceDir),
			Name: filepath.Base(c.workspaceDir),
		}},
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"hover":          map[string]any{"contentFormat": []string{"markdown", "plaintext"}},
				"definition":     map[string]any{"linkSupport": true},
				"implementation": map[string]any{"linkSupport": true},
				"references":     map[string]any{},
				"documentSymbol": map[string]any{"hierarchicalDocumentSymbolSupport": true},
			},
			"workspace": map[string]any{
				"workspaceFolders": true,
			},
		},
	}
	if err := c.request(ctx, "initialize", params, &result); err != nil {
		return fmt.Errorf("initialize %s: %w", c.server.Name, err)
	}
	if err := c.notify("initialized", map[string]any{}); err != nil {
		return fmt.Errorf("notify initialized: %w", err)
	}
	return nil
}

func (c *lspClient) didOpen(ctx context.Context, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file for didOpen %q: %w", filePath, err)
	}
	params := map[string]any{
		"textDocument": map[string]any{
			"uri":        pathToFileURI(filePath),
			"languageId": languageIDForPath(filePath, c.server),
			"version":    1,
			"text":       string(content),
		},
	}
	if err := c.notify("textDocument/didOpen", params); err != nil {
		return fmt.Errorf("notify didOpen: %w", err)
	}
	return nil
}

func (c *lspClient) textDocumentPositionParams(request lspRequest) map[string]any {
	return map[string]any{
		"textDocument": map[string]any{"uri": pathToFileURI(request.FilePath)},
		"position": map[string]any{
			"line":      request.Line - 1,
			"character": request.Column - 1,
		},
	}
}

func (c *lspClient) request(ctx context.Context, method string, params any, result any) error {
	id := c.nextRequestID()
	message := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	if err := c.writeMessage(message); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		envelope, err := c.readMessage()
		if err != nil {
			stderr := strings.TrimSpace(c.stderr.String())
			if stderr != "" {
				return fmt.Errorf("read lsp response for %s: %w (%s)", method, err, stderr)
			}
			return fmt.Errorf("read lsp response for %s: %w", method, err)
		}
		if envelope.Method != "" {
			continue
		}
		responseID, ok := parseLSPResponseID(envelope.ID)
		if !ok || responseID != id {
			continue
		}
		if envelope.Error != nil {
			return fmt.Errorf("lsp %s failed: %s", method, envelope.Error.Message)
		}
		if result == nil || len(envelope.Result) == 0 || string(envelope.Result) == "null" {
			return nil
		}
		if err := json.Unmarshal(envelope.Result, result); err != nil {
			return fmt.Errorf("decode lsp result for %s: %w", method, err)
		}
		return nil
	}
}

func (c *lspClient) notify(method string, params any) error {
	message := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	return c.writeMessage(message)
}

func (c *lspClient) writeMessage(message any) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode lsp message: %w", err)
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := fmt.Fprintf(c.stdin, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	if _, err := c.stdin.Write(data); err != nil {
		return err
	}
	return nil
}

func (c *lspClient) readMessage() (lspResponseEnvelope, error) {
	contentLength := 0
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return lspResponseEnvelope{}, err
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
			value := strings.TrimSpace(parts[1])
			length, err := strconv.Atoi(value)
			if err != nil {
				return lspResponseEnvelope{}, fmt.Errorf("parse content length %q: %w", value, err)
			}
			contentLength = length
		}
	}
	if contentLength <= 0 {
		return lspResponseEnvelope{}, fmt.Errorf("missing content length in lsp message")
	}
	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(c.stdout, payload); err != nil {
		return lspResponseEnvelope{}, err
	}
	var envelope lspResponseEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return lspResponseEnvelope{}, fmt.Errorf("decode lsp message: %w", err)
	}
	return envelope, nil
}

func (c *lspClient) nextRequestID() int64 {
	c.nextID++
	return c.nextID
}

func (c *lspClient) Close() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.request(shutdownCtx, "shutdown", map[string]any{}, nil)
	_ = c.notify("exit", map[string]any{})
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.cmd.Wait()
	return nil
}

func resolveLSPServerConfig(request lspRequest) (lspServerConfig, string, error) {
	if request.FilePath != "" {
		extension := strings.ToLower(filepath.Ext(request.FilePath))
		for _, server := range lspServerConfigs {
			if _, ok := server.Extensions[extension]; ok {
				workspaceDir := detectWorkspaceRoot(filepath.Dir(request.FilePath), server.Markers)
				return server, workspaceDir, nil
			}
		}
		return lspServerConfig{}, "", fmt.Errorf("no LSP server configured for files with extension %q", extension)
	}

	searchPath := request.SearchPath
	if searchPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return lspServerConfig{}, "", fmt.Errorf("get working directory: %w", err)
		}
		searchPath = cwd
	}
	for _, server := range lspServerConfigs {
		workspaceDir := detectWorkspaceRoot(searchPath, server.Markers)
		if containsWorkspaceMarker(workspaceDir, server.Markers) {
			return server, workspaceDir, nil
		}
	}
	return lspServerConfig{}, "", fmt.Errorf("unable to determine an LSP server for workspace_symbols; provide filePath or use a workspace with known markers")
}

func detectWorkspaceRoot(start string, markers []string) string {
	current := start
	for {
		if containsWorkspaceMarker(current, markers) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return start
		}
		current = parent
	}
}

func containsWorkspaceMarker(dir string, markers []string) bool {
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

func pathToFileURI(path string) string {
	resolved := filepath.Clean(path)
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(resolved)}).String()
}

func fileURIToPath(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "file" {
		return value
	}
	return filepath.Clean(parsed.Path)
}

func parseLSPResponseID(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var numeric int64
	if err := json.Unmarshal(raw, &numeric); err == nil {
		return numeric, true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		parsed, err := strconv.ParseInt(text, 10, 64)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func limitLSPResults(rows []lspResultRow, maxResults int) []lspResultRow {
	if maxResults <= 0 || len(rows) <= maxResults {
		return rows
	}
	return rows[:maxResults]
}

func locationRowsFromAny(raw any, kind string) []lspResultRow {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var links []lspLocationLink
	if err := json.Unmarshal(data, &links); err == nil && len(links) > 0 {
		rows := make([]lspResultRow, 0, len(links))
		for _, link := range links {
			row := lspResultRow{Kind: kind, FilePath: fileURIToPath(link.TargetURI)}
			applyRange(&row, &link.TargetSelectionRange)
			if row.Line == 0 {
				applyRange(&row, &link.TargetRange)
			}
			rows = append(rows, row)
		}
		return sortLSPRows(rows)
	}
	var locations []lspLocation
	if err := json.Unmarshal(data, &locations); err == nil && len(locations) > 0 {
		return rowsFromLocations(locations, kind)
	}
	var location lspLocation
	if err := json.Unmarshal(data, &location); err == nil && location.URI != "" {
		return rowsFromLocations([]lspLocation{location}, kind)
	}
	var link lspLocationLink
	if err := json.Unmarshal(data, &link); err == nil && link.TargetURI != "" {
		row := lspResultRow{Kind: kind, FilePath: fileURIToPath(link.TargetURI)}
		applyRange(&row, &link.TargetSelectionRange)
		if row.Line == 0 {
			applyRange(&row, &link.TargetRange)
		}
		return []lspResultRow{row}
	}
	return nil
}

func rowsFromLocations(locations []lspLocation, kind string) []lspResultRow {
	rows := make([]lspResultRow, 0, len(locations))
	for _, location := range locations {
		row := lspResultRow{Kind: kind, FilePath: fileURIToPath(location.URI)}
		applyRange(&row, &location.Range)
		rows = append(rows, row)
	}
	return sortLSPRows(rows)
}

func rowsFromWorkspaceSymbols(symbols []lspSymbolInformation) []lspResultRow {
	rows := make([]lspResultRow, 0, len(symbols))
	for _, symbol := range symbols {
		row := lspResultRow{
			Kind:          "symbol",
			Name:          symbol.Name,
			SymbolKind:    lspSymbolKindName(symbol.Kind),
			FilePath:      fileURIToPath(symbol.Location.URI),
			ContainerName: symbol.ContainerName,
		}
		applyRange(&row, &symbol.Location.Range)
		rows = append(rows, row)
	}
	return sortLSPRows(rows)
}

func documentSymbolRows(raw json.RawMessage, filePath string) []lspResultRow {
	var documentSymbols []lspDocumentSymbol
	if err := json.Unmarshal(raw, &documentSymbols); err == nil && len(documentSymbols) > 0 {
		rows := make([]lspResultRow, 0, len(documentSymbols))
		flattenDocumentSymbols(&rows, documentSymbols, filePath, "")
		return sortLSPRows(rows)
	}
	var symbolInfos []lspSymbolInformation
	if err := json.Unmarshal(raw, &symbolInfos); err == nil && len(symbolInfos) > 0 {
		return rowsFromWorkspaceSymbols(symbolInfos)
	}
	return nil
}

func flattenDocumentSymbols(rows *[]lspResultRow, symbols []lspDocumentSymbol, filePath, container string) {
	for _, symbol := range symbols {
		row := lspResultRow{
			Kind:          "symbol",
			Name:          symbol.Name,
			Detail:        symbol.Detail,
			SymbolKind:    lspSymbolKindName(symbol.Kind),
			FilePath:      filePath,
			ContainerName: container,
		}
		applyRange(&row, &symbol.SelectionRange)
		if row.Line == 0 {
			applyRange(&row, &symbol.Range)
		}
		*rows = append(*rows, row)
		nextContainer := symbol.Name
		if container != "" {
			nextContainer = container + "." + symbol.Name
		}
		flattenDocumentSymbols(rows, symbol.Children, filePath, nextContainer)
	}
}

func sortLSPRows(rows []lspResultRow) []lspResultRow {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].FilePath != rows[j].FilePath {
			return rows[i].FilePath < rows[j].FilePath
		}
		if rows[i].Line != rows[j].Line {
			return rows[i].Line < rows[j].Line
		}
		if rows[i].Column != rows[j].Column {
			return rows[i].Column < rows[j].Column
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func applyRange(row *lspResultRow, source *lspRange) {
	if row == nil || source == nil {
		return
	}
	row.Line = source.Start.Line + 1
	row.Column = source.Start.Character + 1
	row.EndLine = source.End.Line + 1
	row.EndColumn = source.End.Character + 1
}

func extractHoverContents(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]any:
		if raw, ok := typed["value"].(string); ok {
			return raw
		}
		if language, ok := typed["language"].(string); ok {
			if raw, ok := typed["value"].(string); ok && strings.TrimSpace(raw) != "" {
				return language + "\n" + raw
			}
		}
	case []any:
		parts := make([]string, 0, len(typed))
		for _, part := range typed {
			if text := strings.TrimSpace(extractHoverContents(part)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	var markup lspMarkupContent
	if err := json.Unmarshal(data, &markup); err == nil && strings.TrimSpace(markup.Value) != "" {
		return markup.Value
	}
	var marked lspMarkedString
	if err := json.Unmarshal(data, &marked); err == nil && strings.TrimSpace(marked.Value) != "" {
		if strings.TrimSpace(marked.Language) == "" {
			return marked.Value
		}
		return marked.Language + "\n" + marked.Value
	}
	return string(data)
}

func languageIDForPath(path string, server lspServerConfig) string {
	extension := strings.ToLower(filepath.Ext(path))
	if server.Name == "typescript-language-server" {
		switch extension {
		case ".js", ".jsx", ".mjs", ".cjs":
			return "javascript"
		default:
			return "typescript"
		}
	}
	return server.LanguageID
}

func lspSymbolKindName(kind int) string {
	switch kind {
	case 1:
		return "file"
	case 2:
		return "module"
	case 3:
		return "namespace"
	case 4:
		return "package"
	case 5:
		return "class"
	case 6:
		return "method"
	case 7:
		return "property"
	case 8:
		return "field"
	case 9:
		return "constructor"
	case 10:
		return "enum"
	case 11:
		return "interface"
	case 12:
		return "function"
	case 13:
		return "variable"
	case 14:
		return "constant"
	case 15:
		return "string"
	case 16:
		return "number"
	case 17:
		return "boolean"
	case 18:
		return "array"
	case 19:
		return "object"
	case 20:
		return "key"
	case 21:
		return "null"
	case 22:
		return "enum_member"
	case 23:
		return "struct"
	case 24:
		return "event"
	case 25:
		return "operator"
	case 26:
		return "type_parameter"
	default:
		return fmt.Sprintf("kind_%d", kind)
	}
}

func firstPositiveIntOrDefault(params map[string]any, fallback int, keys ...string) int {
	for _, key := range keys {
		if value, ok := intParam(params, key); ok && value > 0 {
			return value
		}
	}
	return fallback
}
