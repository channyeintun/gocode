package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	configpkg "github.com/channyeintun/nami/internal/config"
	mcppkg "github.com/channyeintun/nami/internal/mcp"
)

var mcpServerNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type mcpAddOptions struct {
	scope     string
	transport string
	env       []string
	headers   []string
	trust     bool
	disabled  bool
	startupMS int
}

func newMCPCommand() *cobra.Command {
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Manage MCP server configuration",
	}

	mcpCmd.AddCommand(newMCPAddCommand())
	mcpCmd.AddCommand(newMCPAddJSONCommand())
	mcpCmd.AddCommand(newMCPListCommand())
	mcpCmd.AddCommand(newMCPGetCommand())
	mcpCmd.AddCommand(newMCPRemoveCommand())

	return mcpCmd
}

func newMCPAddCommand() *cobra.Command {
	options := mcpAddOptions{}
	cmd := &cobra.Command{
		Use:   "add <name> <command-or-url> [args...]",
		Short: "Add an MCP server",
		Long: "Add an MCP server to Nami's MCP configuration.\n\n" +
			"Examples:\n" +
			"  nami mcp add my-server -- npx my-mcp-server\n" +
			"  nami mcp add --transport http sentry https://mcp.sentry.dev/mcp\n" +
			"  nami mcp add --scope user --transport sse relay https://example.com/sse --header 'Authorization: Bearer token'",
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPAdd(cmd, args, options)
		},
	}
	cmd.Flags().StringVarP(&options.scope, "scope", "s", string(configpkg.MCPScopeProject), "Configuration scope (project or user)")
	cmd.Flags().StringVarP(&options.transport, "transport", "t", "", "Transport type (stdio, http, sse, or ws). Defaults to stdio")
	cmd.Flags().StringArrayVarP(&options.env, "env", "e", nil, "Set an environment variable for stdio transport (KEY=value)")
	cmd.Flags().StringArrayVarP(&options.headers, "header", "H", nil, "Set a request header for http/sse/ws transports (Key: Value)")
	cmd.Flags().BoolVar(&options.trust, "trust", false, "Mark the server as trusted so configured tool permissions apply")
	cmd.Flags().BoolVar(&options.disabled, "disabled", false, "Add the server in a disabled state")
	cmd.Flags().IntVar(&options.startupMS, "startup-timeout-ms", 0, "Override stdio startup timeout in milliseconds")
	return cmd
}

func newMCPAddJSONCommand() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "add-json <name> <json>",
		Short: "Add an MCP server from raw JSON",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			parsedScope, err := configpkg.ParseMCPScope(scope)
			if err != nil {
				return err
			}
			name := strings.TrimSpace(args[0])
			if err := validateMCPServerName(name); err != nil {
				return err
			}
			var server configpkg.MCPServerConfig
			if err := json.Unmarshal([]byte(args[1]), &server); err != nil {
				return fmt.Errorf("parse server JSON: %w", err)
			}
			return addMCPServer(cmd, cwd, parsedScope, name, server, "")
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", string(configpkg.MCPScopeProject), "Configuration scope (project or user)")
	return cmd
}

func newMCPListCommand() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured MCP servers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMCPList(cmd, scope)
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "", "Optional scope filter (project or user)")
	return cmd
}

func newMCPGetCommand() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Show MCP server details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPGet(cmd, args[0], scope)
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "", "Optional scope filter (project or user)")
	return cmd
}

func newMCPRemoveCommand() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPRemove(cmd, args[0], scope)
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "", "Optional scope filter (project or user)")
	return cmd
}

func runMCPAdd(cmd *cobra.Command, args []string, options mcpAddOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	name := strings.TrimSpace(args[0])
	if err := validateMCPServerName(name); err != nil {
		return err
	}
	if options.startupMS < 0 {
		return fmt.Errorf("startup-timeout-ms must be greater than or equal to 0")
	}
	scope, err := configpkg.ParseMCPScope(options.scope)
	if err != nil {
		return err
	}
	transport, err := parseMCPTransport(options.transport)
	if err != nil {
		return err
	}
	server, warning, summary, err := buildMCPServerConfig(transport, strings.TrimSpace(args[1]), args[2:], options)
	if err != nil {
		return err
	}
	if warning != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), warning)
	}
	return addMCPServer(cmd, cwd, scope, name, server, summary)
}

func addMCPServer(cmd *cobra.Command, cwd string, scope configpkg.MCPScope, name string, server configpkg.MCPServerConfig, summary string) error {
	if err := validateMCPServerConfig(cwd, name, server); err != nil {
		return err
	}
	existing, _, err := configpkg.LoadMCPConfigForScope(cwd, scope)
	if err != nil {
		return err
	}
	if existing.Servers == nil {
		existing.Servers = make(map[string]configpkg.MCPServerConfig)
	}
	if _, ok := existing.Servers[name]; ok {
		return fmt.Errorf("MCP server %q already exists in %s config", name, scope)
	}
	existing.Servers[name] = server
	path, err := configpkg.SaveMCPConfigForScope(cwd, scope, existing)
	if err != nil {
		return err
	}
	transport := effectiveTransportLabel(server)
	if summary == "" {
		summary = renderServerSummary(server)
	}
	if strings.TrimSpace(summary) != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Added %s MCP server %s to %s config: %s\n", transport, name, scope, summary)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Added %s MCP server %s to %s config\n", transport, name, scope)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Config path: %s\n", path)
	return nil
}

func runMCPList(cmd *cobra.Command, scopeRaw string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	configToList, sources, err := loadMCPConfigForListing(cwd, scopeRaw)
	if err != nil {
		return err
	}
	if len(configToList.Servers) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No MCP servers configured.")
		return nil
	}

	statuses, closeErr := loadMCPStatuses(cwd, configToList)
	if closeErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: close MCP manager: %v\n", closeErr)
	}

	for _, status := range statuses {
		summary := renderStatusSummary(status)
		source := sources[status.Name]
		server := configToList.Servers[status.Name]
		location := renderServerSummary(server)
		if source != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s [%s] - %s\n", status.Name, location, source, summary)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s - %s\n", status.Name, location, summary)
	}
	return nil
}

func runMCPGet(cmd *cobra.Command, rawName, scopeRaw string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	name := strings.TrimSpace(rawName)
	if err := validateMCPServerName(name); err != nil {
		return err
	}

	targetScope, server, configuredScopes, err := resolveMCPServerForGet(cwd, name, scopeRaw)
	if err != nil {
		return err
	}
	status, closeErr, err := loadSingleMCPStatus(cwd, name, server)
	if closeErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: close MCP manager: %v\n", closeErr)
	}
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Scope: %s\n", targetScope)
	if len(configuredScopes) > 1 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Configured in: %s\n", strings.Join(configuredScopes, ", "))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  Transport: %s\n", effectiveTransportLabel(server))
	fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", renderStatusSummary(status))

	if server.Enabled != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Enabled: %t\n", *server.Enabled)
	}
	if server.Trust != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Trusted: %t\n", *server.Trust)
	}
	if server.StartupTimeoutMS != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Startup timeout: %dms\n", *server.StartupTimeoutMS)
	}

	if server.Command != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Command: %s\n", *server.Command)
	}
	if len(server.Args) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Args: %s\n", strings.Join(server.Args, " "))
	}
	if len(server.Env) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  Environment:")
		printSortedKeyValue(cmd, server.Env, "    %s=%s\n")
	}
	if server.URL != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  URL: %s\n", *server.URL)
	}
	if len(server.Headers) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  Headers:")
		printSortedKeyValue(cmd, server.Headers, "    %s: %s\n")
	}
	if len(server.IncludeTools) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Include tools: %s\n", strings.Join(server.IncludeTools, ", "))
	}
	if len(server.ExcludeTools) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Exclude tools: %s\n", strings.Join(server.ExcludeTools, ", "))
	}
	if len(server.ToolPermissions) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  Tool permissions:")
		toolNames := make([]string, 0, len(server.ToolPermissions))
		for toolName := range server.ToolPermissions {
			toolNames = append(toolNames, toolName)
		}
		sort.Strings(toolNames)
		for _, toolName := range toolNames {
			fmt.Fprintf(cmd.OutOrStdout(), "    %s: %s\n", toolName, server.ToolPermissions[toolName])
		}
	}
	return nil
}

func runMCPRemove(cmd *cobra.Command, rawName, scopeRaw string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	name := strings.TrimSpace(rawName)
	if err := validateMCPServerName(name); err != nil {
		return err
	}

	scope, err := resolveMCPRemoveScope(cwd, name, scopeRaw)
	if err != nil {
		return err
	}
	cfg, _, err := configpkg.LoadMCPConfigForScope(cwd, scope)
	if err != nil {
		return err
	}
	if _, ok := cfg.Servers[name]; !ok {
		return fmt.Errorf("No MCP server found with name %q in %s config", name, scope)
	}
	delete(cfg.Servers, name)
	path, err := configpkg.SaveMCPConfigForScope(cwd, scope, cfg)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Removed MCP server %s from %s config\n", name, scope)
	fmt.Fprintf(cmd.OutOrStdout(), "Config path: %s\n", path)
	return nil
}

func validateMCPServerName(name string) error {
	if name == "" {
		return fmt.Errorf("server name cannot be empty")
	}
	if !mcpServerNamePattern.MatchString(name) {
		return fmt.Errorf("invalid server name %q: names can only contain letters, numbers, hyphens, and underscores", name)
	}
	return nil
}

func parseMCPTransport(raw string) (mcppkg.TransportKind, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return mcppkg.TransportStdio, nil
	}
	transport := mcppkg.TransportKind(trimmed)
	switch transport {
	case mcppkg.TransportStdio, mcppkg.TransportHTTP, mcppkg.TransportSSE, mcppkg.TransportWS:
		return transport, nil
	default:
		return "", fmt.Errorf("unsupported MCP transport %q (valid: stdio, http, sse, ws)", strings.TrimSpace(raw))
	}
}

func buildMCPServerConfig(transport mcppkg.TransportKind, commandOrURL string, extraArgs []string, options mcpAddOptions) (configpkg.MCPServerConfig, string, string, error) {
	transportValue := string(transport)
	server := configpkg.MCPServerConfig{Transport: &transportValue}
	if options.disabled {
		enabled := false
		server.Enabled = &enabled
	}
	if options.trust {
		trusted := true
		server.Trust = &trusted
	}
	if options.startupMS > 0 {
		server.StartupTimeoutMS = intPtr(options.startupMS)
	}

	switch transport {
	case mcppkg.TransportStdio:
		if len(options.headers) > 0 {
			return configpkg.MCPServerConfig{}, "", "", fmt.Errorf("stdio transport does not accept headers")
		}
		env, err := parseEnvAssignments(options.env)
		if err != nil {
			return configpkg.MCPServerConfig{}, "", "", err
		}
		server.Command = stringPtr(commandOrURL)
		if len(extraArgs) > 0 {
			server.Args = append([]string(nil), extraArgs...)
		}
		server.Env = env
		warning := ""
		if looksLikeURL(commandOrURL) {
			warning = fmt.Sprintf("warning: %q looks like a URL; if this is an HTTP server, use --transport http, or use --transport sse for SSE", commandOrURL)
		}
		return server, warning, strings.TrimSpace(strings.Join(append([]string{commandOrURL}, extraArgs...), " ")), nil
	case mcppkg.TransportHTTP, mcppkg.TransportSSE, mcppkg.TransportWS:
		if len(extraArgs) > 0 {
			return configpkg.MCPServerConfig{}, "", "", fmt.Errorf("%s transport accepts only a single URL argument", transport)
		}
		if len(options.env) > 0 {
			return configpkg.MCPServerConfig{}, "", "", fmt.Errorf("%s transport does not accept env vars", transport)
		}
		headers, err := parseHeaderAssignments(options.headers)
		if err != nil {
			return configpkg.MCPServerConfig{}, "", "", err
		}
		server.URL = stringPtr(commandOrURL)
		server.Headers = headers
		return server, "", commandOrURL, nil
	default:
		return configpkg.MCPServerConfig{}, "", "", fmt.Errorf("unsupported transport %q", transport)
	}
}

func parseEnvAssignments(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	parsed := make(map[string]string, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid env assignment %q: expected KEY=value", value)
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("invalid env assignment %q: key cannot be empty", value)
		}
		parsed[key] = parts[1]
	}
	return parsed, nil
}

func parseHeaderAssignments(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	parsed := make(map[string]string, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header %q: expected Key: Value", value)
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("invalid header %q: key cannot be empty", value)
		}
		parsed[key] = strings.TrimSpace(parts[1])
	}
	return parsed, nil
}

func looksLikeURL(value string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "ws://") || strings.HasPrefix(trimmed, "wss://")
}

func validateMCPServerConfig(cwd, name string, server configpkg.MCPServerConfig) error {
	resolved := mcppkg.ResolveConfig(cwd, configpkg.MCPConfig{
		Servers: map[string]configpkg.MCPServerConfig{name: server},
	})
	if len(resolved.Problems) > 0 {
		return resolved.Problems[0].Err
	}
	if len(resolved.Servers) != 1 {
		return fmt.Errorf("failed to validate MCP server %q", name)
	}
	return nil
}

func loadMCPConfigForListing(cwd, scopeRaw string) (configpkg.MCPConfig, map[string]string, error) {
	if strings.TrimSpace(scopeRaw) != "" {
		scope, err := configpkg.ParseMCPScope(scopeRaw)
		if err != nil {
			return configpkg.MCPConfig{}, nil, err
		}
		cfg, _, err := configpkg.LoadMCPConfigForScope(cwd, scope)
		if err != nil {
			return configpkg.MCPConfig{}, nil, err
		}
		sources := make(map[string]string, len(cfg.Servers))
		for name := range cfg.Servers {
			sources[name] = scope.String()
		}
		return cfg, sources, nil
	}

	userCfg, _, err := configpkg.LoadMCPConfigForScope(cwd, configpkg.MCPScopeUser)
	if err != nil {
		return configpkg.MCPConfig{}, nil, err
	}
	projectCfg, _, err := configpkg.LoadMCPConfigForScope(cwd, configpkg.MCPScopeProject)
	if err != nil && !errors.Is(err, configpkg.ErrProjectScopeUnavailable) {
		return configpkg.MCPConfig{}, nil, err
	}
	merged := configpkg.MergeMCPConfig(userCfg, projectCfg)
	sources := make(map[string]string, len(merged.Servers))
	for name := range userCfg.Servers {
		sources[name] = configpkg.MCPScopeUser.String()
	}
	for name := range projectCfg.Servers {
		if _, ok := userCfg.Servers[name]; ok {
			sources[name] = fmt.Sprintf("%s (overrides user)", configpkg.MCPScopeProject)
			continue
		}
		sources[name] = configpkg.MCPScopeProject.String()
	}
	return merged, sources, nil
}

func loadMCPStatuses(cwd string, cfg configpkg.MCPConfig) ([]mcppkg.ServerStatus, error) {
	manager := mcppkg.NewManager(cwd, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	manager.Start(ctx)
	statuses := manager.Statuses()
	return statuses, manager.Close()
}

func resolveMCPServerForGet(cwd, name, scopeRaw string) (string, configpkg.MCPServerConfig, []string, error) {
	entries, err := collectScopedServers(cwd, name)
	if err != nil {
		return "", configpkg.MCPServerConfig{}, nil, err
	}
	if len(entries) == 0 {
		return "", configpkg.MCPServerConfig{}, nil, fmt.Errorf("No MCP server found with name %q", name)
	}

	if strings.TrimSpace(scopeRaw) != "" {
		scope, err := configpkg.ParseMCPScope(scopeRaw)
		if err != nil {
			return "", configpkg.MCPServerConfig{}, nil, err
		}
		server, ok := entries[scope]
		if !ok {
			return "", configpkg.MCPServerConfig{}, nil, fmt.Errorf("No MCP server found with name %q in %s config", name, scope)
		}
		return scope.String(), server, []string{scope.String()}, nil
	}

	configuredScopes := make([]string, 0, len(entries))
	if _, ok := entries[configpkg.MCPScopeUser]; ok {
		configuredScopes = append(configuredScopes, configpkg.MCPScopeUser.String())
	}
	if _, ok := entries[configpkg.MCPScopeProject]; ok {
		configuredScopes = append(configuredScopes, configpkg.MCPScopeProject.String())
	}
	sort.Strings(configuredScopes)
	if server, ok := entries[configpkg.MCPScopeProject]; ok {
		return configpkg.MCPScopeProject.String(), server, configuredScopes, nil
	}
	return configpkg.MCPScopeUser.String(), entries[configpkg.MCPScopeUser], configuredScopes, nil
}

func loadSingleMCPStatus(cwd, name string, server configpkg.MCPServerConfig) (mcppkg.ServerStatus, error, error) {
	manager := mcppkg.NewManager(cwd, configpkg.MCPConfig{
		Servers: map[string]configpkg.MCPServerConfig{name: server},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	manager.Start(ctx)
	statuses := manager.Statuses()
	closeErr := manager.Close()
	if len(statuses) == 0 {
		return mcppkg.ServerStatus{}, closeErr, fmt.Errorf("MCP server %q did not produce a status entry", name)
	}
	return statuses[0], closeErr, nil
}

func resolveMCPRemoveScope(cwd, name, scopeRaw string) (configpkg.MCPScope, error) {
	if strings.TrimSpace(scopeRaw) != "" {
		return configpkg.ParseMCPScope(scopeRaw)
	}
	entries, err := collectScopedServers(cwd, name)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("No MCP server found with name %q", name)
	}
	if len(entries) == 1 {
		for scope := range entries {
			return scope, nil
		}
	}
	return "", fmt.Errorf("MCP server %q exists in multiple scopes; rerun with --scope project or --scope user", name)
}

func collectScopedServers(cwd, name string) (map[configpkg.MCPScope]configpkg.MCPServerConfig, error) {
	entries := make(map[configpkg.MCPScope]configpkg.MCPServerConfig, 2)
	userCfg, _, err := configpkg.LoadMCPConfigForScope(cwd, configpkg.MCPScopeUser)
	if err != nil {
		return nil, err
	}
	if server, ok := userCfg.Servers[name]; ok {
		entries[configpkg.MCPScopeUser] = server
	}
	projectCfg, _, err := configpkg.LoadMCPConfigForScope(cwd, configpkg.MCPScopeProject)
	if err != nil && !errors.Is(err, configpkg.ErrProjectScopeUnavailable) {
		return nil, err
	}
	if server, ok := projectCfg.Servers[name]; ok {
		entries[configpkg.MCPScopeProject] = server
	}
	return entries, nil
}

func renderStatusSummary(status mcppkg.ServerStatus) string {
	if !status.Enabled {
		return "disabled"
	}
	if strings.TrimSpace(status.Error) != "" {
		return "error: " + status.Error
	}
	if !status.Connected {
		return "not connected"
	}
	summary := fmt.Sprintf("connected, %d tools, %d prompts, %d resources, %d templates", status.ToolCount, status.PromptCount, status.ResourceCount, status.ResourceTemplateCount)
	if len(status.Warnings) > 0 {
		summary += " [warnings: " + strings.Join(status.Warnings, "; ") + "]"
	}
	return summary
}

func renderServerSummary(server configpkg.MCPServerConfig) string {
	switch effectiveTransportLabel(server) {
	case string(mcppkg.TransportStdio):
		parts := []string{}
		if server.Command != nil {
			parts = append(parts, *server.Command)
		}
		parts = append(parts, server.Args...)
		return strings.TrimSpace(strings.Join(parts, " "))
	default:
		if server.URL != nil {
			return *server.URL
		}
		return "(" + effectiveTransportLabel(server) + ")"
	}
}

func effectiveTransportLabel(server configpkg.MCPServerConfig) string {
	if server.Transport != nil && strings.TrimSpace(*server.Transport) != "" {
		return strings.TrimSpace(*server.Transport)
	}
	if server.URL != nil {
		return string(mcppkg.TransportHTTP)
	}
	if server.Command != nil {
		return string(mcppkg.TransportStdio)
	}
	return "unknown"
}

func printSortedKeyValue(cmd *cobra.Command, values map[string]string, format string) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(cmd.OutOrStdout(), format, key, values[key])
	}
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}
