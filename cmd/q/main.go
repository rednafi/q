package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"q/internal/config"
	"q/internal/providers"
	"q/internal/providers/openai"
)

// Version information set during build
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// CLI holds all the dependencies for the CLI application
type CLI struct {
	registry *providers.Registry
	version  string
	commit   string
	date     string
}

// NewCLI creates a new CLI instance with all dependencies
func NewCLI() *CLI {
	registry := providers.NewRegistry()
	registry.Register(
		openai.NewProvider(),
	)

	return &CLI{
		registry: registry,
		version:  version,
		commit:   commit,
		date:     date,
	}
}

// createCancellableContext creates a context that gets cancelled on Ctrl+C
func createCancellableContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	// Set up signal handling for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, cancelling...")
		cancel()
	}()

	return ctx
}

// CommandFlags holds the common flags used across commands
type CommandFlags struct {
	Model    string
	NoStream bool
	Raw      bool
}

// parseCommonFlags extracts common flags from a command
func (cli *CLI) parseCommonFlags(cmd *cobra.Command) (*CommandFlags, error) {
	model, err := cmd.Flags().GetString("model")
	if err != nil {
		return nil, fmt.Errorf("failed to parse --model flag: %w", err)
	}
	noStream, err := cmd.Flags().GetBool("no-stream")
	if err != nil {
		return nil, fmt.Errorf("failed to parse --no-stream flag: %w", err)
	}
	raw, err := cmd.Flags().GetBool("raw")
	if err != nil {
		return nil, fmt.Errorf("failed to parse --raw flag: %w", err)
	}

	return &CommandFlags{
		Model:    model,
		NoStream: noStream,
		Raw:      raw,
	}, nil
}

// setupModelAndProvider handles the common logic for model and provider setup
func (cli *CLI) setupModelAndProvider(model string) (string, string, providers.Provider, error) {
	if model == "" {
		m, err := config.GetDefaultModel()
		if err != nil {
			return "", "", nil, fmt.Errorf("failed to load default model: %w", err)
		}
		if m == "" {
			return "", "", nil, errors.New("no default model\n\nSet default: q default set --model provider/model\nOr specify: q --model provider/model")
		}
		model = m
	}

	parts := strings.SplitN(model, "/", 2)
	if len(parts) != 2 {
		return "", "", nil, errors.New("invalid model format\n\nUse: provider/model (e.g., openai/gpt-4o)")
	}
	provider, mdl := parts[0], parts[1]

	p, ok := cli.registry.Lookup(provider)
	if !ok {
		return "", "", nil, fmt.Errorf("unknown provider: %s\n\nSee available: q models list", provider)
	}
	if !slices.Contains(p.SupportedModels(), mdl) {
		return "", "", nil, fmt.Errorf("unsupported model '%s' for %s\n\nSee available: q models list", mdl, provider)
	}

	key, err := config.GetAPIKey(provider)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to read API key for %s: %w", provider, err)
	}
	if key == "" {
		return "", "", nil, fmt.Errorf("no API key for %s\n\nSet key: q keys set --provider %s --key KEY", provider, provider)
	}

	return provider, mdl, p, nil
}

// readPromptFromStdin reads prompt from stdin when "-" is provided
func readPromptFromStdin() (string, error) {
	bytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("error reading from stdin: %w", err)
	}
	prompt := strings.TrimSpace(string(bytes))
	if prompt == "" {
		return "", errors.New("no input provided via stdin")
	}
	return prompt, nil
}

// handleStreamingResponse handles streaming response with proper formatting
func (cli *CLI) handleStreamingResponse(ctx context.Context, provider, model string, prompt string, raw bool, p providers.Provider) error {
	if !raw {
		fmt.Printf("model (%s/%s): ", provider, model)
	}
	_, err := p.Stream(ctx, model, prompt)
	if err != nil {
		return err
	}
	// Only add newline if not raw mode
	if !raw {
		fmt.Println()
	}
	return nil
}

// handleNonStreamingResponse handles non-streaming response with proper formatting
func (cli *CLI) handleNonStreamingResponse(ctx context.Context, provider, model string, prompt string, raw bool, p providers.Provider) error {
	resp, err := p.Prompt(ctx, model, prompt)
	if err != nil {
		return err
	}
	if raw {
		fmt.Print(resp)
	} else {
		fmt.Printf("model (%s/%s): %s\n", provider, model, resp)
	}
	return nil
}

// createRootCmd creates the root command with dependencies injected
func (cli *CLI) createRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "q [prompt]",
		Short:        "A fast CLI for chatting with your favorite language models.",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			flags, err := cli.parseCommonFlags(cmd)
			if err != nil {
				return err
			}

			// Handle stdin reading with "-"
			prompt := args[0]
			if prompt == "-" {
				prompt, err = readPromptFromStdin()
				if err != nil {
					return err
				}
			}

			provider, model, p, err := cli.setupModelAndProvider(flags.Model)
			if err != nil {
				return err
			}

			ctx := createCancellableContext()

			if !flags.NoStream {
				return cli.handleStreamingResponse(ctx, provider, model, prompt, flags.Raw, p)
			}
			return cli.handleNonStreamingResponse(ctx, provider, model, prompt, flags.Raw, p)
		},
	}
}

// createChatCmd creates the chat command with dependencies injected
func (cli *CLI) createChatCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "chat",
		Short:        "Start interactive REPL with a model",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags, err := cli.parseCommonFlags(cmd)
			if err != nil {
				return err
			}

			provider, model, p, err := cli.setupModelAndProvider(flags.Model)
			if err != nil {
				return err
			}

			ctx := createCancellableContext()

			if !flags.NoStream {
				reader := bufio.NewReader(os.Stdin)
				first := true
				for {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
						// Continue with normal flow
					}

					if !first && !flags.Raw {
						fmt.Println()
					}
					first = false
					if !flags.Raw {
						fmt.Print("you: ")
					}
					text, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							return nil
						}
						return fmt.Errorf("error reading input: %w", err)
					}
					text = strings.TrimSpace(text)
					if text == "" {
						continue
					}
					if !flags.Raw {
						fmt.Printf("model (%s/%s): ", provider, model)
					}
					_, err = p.ChatStream(ctx, model, text)
					if err != nil {
						return err
					}
					if !flags.Raw {
						fmt.Println()
					}
				}
			} else {
				// Non-streaming chat mode
				reader := bufio.NewReader(os.Stdin)
				first := true
				for {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
						// Continue with normal flow
					}

					if !first && !flags.Raw {
						fmt.Println()
					}
					first = false
					if !flags.Raw {
						fmt.Print("you: ")
					}
					text, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							return nil
						}
						return fmt.Errorf("error reading input: %w", err)
					}
					text = strings.TrimSpace(text)
					if text == "" {
						continue
					}
					resp, err := p.ChatPrompt(ctx, model, text)
					if err != nil {
						return err
					}
					if flags.Raw {
						fmt.Print(resp)
					} else {
						fmt.Printf("model (%s/%s): %s\n", provider, model, resp)
					}
				}
			}
		},
	}
}

// createModelsCmd creates the models command with dependencies injected
func (cli *CLI) createModelsCmd() *cobra.Command {
	modelsCmd := &cobra.Command{
		Use:   "models",
		Short: "Manage models",
	}

	modelsListCmd := &cobra.Command{
		Use:          "list",
		Short:        "List available provider/model combinations",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, pr := range cli.registry.Names() {
				p, _ := cli.registry.Lookup(pr)
				for _, m := range p.SupportedModels() {
					fmt.Printf("%s/%s\n", pr, m)
				}
			}
			return nil
		},
	}

	modelsCmd.AddCommand(modelsListCmd)
	return modelsCmd
}

// createKeysCmd creates the keys command with dependencies injected
func (cli *CLI) createKeysCmd() *cobra.Command {
	keysCmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage API keys",
	}

	keysListCmd := &cobra.Command{
		Use:          "list",
		Short:        "List which providers have keys set",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("error loading config: %w", err)
			}
			for _, pr := range cli.registry.Names() {
				status := "❌"
				if k := cfg.APIKeys[pr]; k != "" {
					status = "✅"
				}
				fmt.Printf("%s: %s\n", pr, status)
			}
			return nil
		},
	}

	keysSetCmd := &cobra.Command{
		Use:          "set",
		Short:        "Set API key for a provider",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, _ := cmd.Flags().GetString("provider")
			key, _ := cmd.Flags().GetString("key")
			if provider == "" {
				_ = cmd.Help()
				return errors.New("provider required")
			}
			if key == "" {
				_ = cmd.Help()
				return errors.New("API key required")
			}

			// Validate that the provider is supported
			if _, ok := cli.registry.Lookup(provider); !ok {
				return fmt.Errorf("unknown provider: %s\n\nSee available: q models list", provider)
			}

			if err := config.SetAPIKey(provider, key); err != nil {
				return fmt.Errorf("error saving key: %w", err)
			}
			fmt.Printf("Saved key for %s\n", provider)
			return nil
		},
	}

	keysPathCmd := &cobra.Command{
		Use:          "path",
		Short:        "Show path to API key config file",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ConfigPath()
			if err != nil {
				return fmt.Errorf("error getting config path: %w", err)
			}
			fmt.Println(path)
			return nil
		},
	}

	// flag wiring for keys commands
	keysSetCmd.Flags().StringP("provider", "p", "", "provider name")
	keysSetCmd.Flags().StringP("key", "k", "", "API key")

	keysCmd.AddCommand(keysListCmd, keysSetCmd, keysPathCmd)
	return keysCmd
}

// createDefaultCmd creates the default command with dependencies injected
func (cli *CLI) createDefaultCmd() *cobra.Command {
	defaultCmd := &cobra.Command{
		Use:   "default",
		Short: "Manage default model",
	}

	defaultListCmd := &cobra.Command{
		Use:          "list",
		Short:        "Show default model",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := config.GetDefaultModel()
			if err != nil {
				return fmt.Errorf("error loading default: %w", err)
			}
			if m == "" {
				return errors.New("no default model set")
			}
			fmt.Println(m)
			return nil
		},
	}

	defaultSetCmd := &cobra.Command{
		Use:          "set",
		Short:        "Set default model",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			model, err := cmd.Flags().GetString("model")
			if err != nil {
				return fmt.Errorf("failed to parse --model flag: %w", err)
			}
			if model == "" {
				_ = cmd.Help()
				return errors.New("model must be provided with --model flag")
			}

			// Validate that the model is supported by a provider
			parts := strings.SplitN(model, "/", 2)
			if len(parts) != 2 {
				return errors.New("invalid model format\n\nUse: provider/model (e.g., openai/gpt-4o)")
			}
			provider, mdl := parts[0], parts[1]

			p, ok := cli.registry.Lookup(provider)
			if !ok {
				return fmt.Errorf("unknown provider: %s\n\nSee available: q models list", provider)
			}
			if !slices.Contains(p.SupportedModels(), mdl) {
				return fmt.Errorf("unsupported model '%s' for %s\n\nSee available: q models list", mdl, provider)
			}

			if err := config.SetDefaultModel(model); err != nil {
				return fmt.Errorf("error saving default: %w", err)
			}
			fmt.Printf("Saved default model: %s\n", model)
			return nil
		},
	}

	// flag wiring for default commands
	defaultSetCmd.Flags().StringP("model", "m", "", "provider/model")

	defaultCmd.AddCommand(defaultListCmd, defaultSetCmd)
	return defaultCmd
}

// createVersionCmd creates the version command with dependencies injected
func (cli *CLI) createVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "version",
		Short:        "Show version information",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("q version %s\n", cli.version)
			fmt.Printf("commit: %s\n", cli.commit)
			fmt.Printf("date: %s\n", cli.date)
			return nil
		},
	}
}

// addCommonFlags adds the common flags to a command
func addCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("model", "m", "", "provider/model")
	cmd.Flags().Bool("no-stream", false, "Disable streaming output")
	cmd.Flags().BoolP("raw", "r", false, "Return raw model output")
}

// CreateRootCommand creates the complete command tree with all dependencies
func (cli *CLI) CreateRootCommand() *cobra.Command {
	rootCmd := cli.createRootCmd()
	addCommonFlags(rootCmd)

	// Create subcommands
	chatCmd := cli.createChatCmd()
	addCommonFlags(chatCmd)

	modelsCmd := cli.createModelsCmd()
	keysCmd := cli.createKeysCmd()
	defaultCmd := cli.createDefaultCmd()
	versionCmd := cli.createVersionCmd()

	// command wiring
	rootCmd.AddCommand(chatCmd, modelsCmd, keysCmd, defaultCmd, versionCmd)

	return rootCmd
}

// run wires providers, flags, and commands explicitly.
func run() error {
	cli := NewCLI()
	rootCmd := cli.CreateRootCommand()

	if err := rootCmd.Execute(); err != nil {
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}
