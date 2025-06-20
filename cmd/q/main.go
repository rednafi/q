package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"q/internal/config"
	"q/internal/providers"
	"q/internal/providers/anthropic"
	"q/internal/providers/google"
	"q/internal/providers/openai"
	"slices"

	"github.com/spf13/cobra"
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
		openai.New(),
		google.New(),
		anthropic.New(),
	)

	return &CLI{
		registry: registry,
		version:  version,
		commit:   commit,
		date:     date,
	}
}

// createRootCmd creates the root command with dependencies injected
func (cli *CLI) createRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "q [prompt]",
		Short:        "A fast CLI for chatting with your favorite language models.",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// read flags from command context
			model, err := cmd.Flags().GetString("model")
			if err != nil {
				return fmt.Errorf("failed to parse --model flag: %w", err)
			}
			noStream, err := cmd.Flags().GetBool("no-stream")
			if err != nil {
				return fmt.Errorf("failed to parse --no-stream flag: %w", err)
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			if model == "" {
				m, err := config.GetDefaultModel()
				if err != nil {
					return fmt.Errorf("error loading default: %w", err)
				}
				if m == "" {
					return errors.New("no default model set; use 'q default set --model provider/model'")
				}
				model = m
			}
			parts := strings.SplitN(model, "/", 2)
			if len(parts) != 2 {
				return errors.New("invalid model format; use provider/model")
			}
			provider, mdl := parts[0], parts[1]
			p, ok := cli.registry.Lookup(provider)
			if !ok {
				return fmt.Errorf("unknown provider: %s", provider)
			}
			if !slices.Contains(p.SupportedModels(), mdl) {
				return fmt.Errorf("unsupported model for %s: %s", provider, mdl)
			}
			key, err := config.GetAPIKey(provider)
			if err != nil {
				return fmt.Errorf("error reading keys: %w", err)
			}
			if key == "" {
				return fmt.Errorf("no API key set for %s; use 'q keys set --provider %s --key KEY'", provider, provider)
			}
			if !noStream {
				if streamer, ok := p.(interface{ Stream(string, string) error }); ok {
					fmt.Printf("model (%s/%s): ", provider, mdl)
					if err := streamer.Stream(mdl, args[0]); err != nil {
						return fmt.Errorf("error during prompt: %w", err)
					}
					fmt.Println()
					return nil
				}
			}
			resp, err := p.Prompt(mdl, args[0])
			if err != nil {
				return fmt.Errorf("error during prompt: %w", err)
			}
			fmt.Printf("model (%s/%s): %s\n", provider, mdl, resp)
			return nil
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
			// read CLI flags
			model, err := cmd.Flags().GetString("model")
			if err != nil {
				return fmt.Errorf("failed to parse --model flag: %w", err)
			}
			noStream, err := cmd.Flags().GetBool("no-stream")
			if err != nil {
				return fmt.Errorf("failed to parse --no-stream flag: %w", err)
			}
			if model == "" {
				m, err := config.GetDefaultModel()
				if err != nil {
					return fmt.Errorf("error loading default: %w", err)
				}
				if m == "" {
					return errors.New("no default model set; use 'q default set --model provider/model'")
				}
				model = m
			}
			parts := strings.SplitN(model, "/", 2)
			provider, mdl := parts[0], parts[1]
			p, ok := cli.registry.Lookup(provider)
			if !ok {
				return fmt.Errorf("unknown provider: %s", provider)
			}
			if !slices.Contains(p.SupportedModels(), mdl) {
				return fmt.Errorf("unsupported model for %s: %s", provider, mdl)
			}
			key, err := config.GetAPIKey(provider)
			if err != nil {
				return fmt.Errorf("error reading keys: %w", err)
			}
			if key == "" {
				return fmt.Errorf("no API key set for %s; use 'q keys set --provider %s --key KEY'", provider, provider)
			}
			if !noStream {
				reader := bufio.NewReader(os.Stdin)
				for {
					fmt.Print("you: ")
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
					fmt.Printf("model (%s/%s): ", provider, mdl)
					if streamer, ok := p.(interface{ Stream(string, string) error }); ok {
						if err := streamer.Stream(mdl, text); err != nil {
							return fmt.Errorf("error during chat: %w", err)
						}
						fmt.Println()
					} else {
						resp, err := p.Prompt(mdl, text)
						if err != nil {
							return fmt.Errorf("error during chat: %w", err)
						}
						fmt.Printf("%s\n", resp)
					}
				}
			}
			if err := p.Chat(mdl); err != nil {
				return fmt.Errorf("error during chat: %w", err)
			}
			return nil
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
			if provider == "" || key == "" {
				_ = cmd.Help()
				return errors.New("provider and key must be provided")
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

// CreateRootCommand creates the complete command tree with all dependencies
func (cli *CLI) CreateRootCommand() *cobra.Command {
	rootCmd := cli.createRootCmd()

	// flag wiring
	rootCmd.Flags().StringP("model", "m", "", "provider/model")
	rootCmd.Flags().Bool("no-stream", false, "Disable streaming output")

	// Create subcommands
	chatCmd := cli.createChatCmd()
	chatCmd.Flags().StringP("model", "m", "", "provider/model")
	chatCmd.Flags().Bool("no-stream", false, "Disable streaming output")

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
