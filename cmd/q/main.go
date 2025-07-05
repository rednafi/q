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

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

type CLI struct{ registry *providers.Registry }

func NewCLI() *CLI {
	r := providers.NewRegistry()
	r.Register(openai.NewProvider())
	return &CLI{registry: r}
}

// contextWithInterrupt returns a context that cancels when the user presses Ctrl-C.
func contextWithInterrupt() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		fmt.Fprintln(os.Stderr, "\nReceived Ctrl + C, quitting...")
		cancel()
	}()
	return ctx
}

func promptFromStdin() (string, error) {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	prompt := strings.TrimSpace(string(b))
	if prompt == "" {
		return "", errors.New("no input provided via stdin")
	}
	return prompt, nil
}

func writePrefix(provider, model string) {
	fmt.Printf("model (%s/%s): ", provider, model)
	os.Stdout.Sync()
}

type flags struct {
	model    string
	noStream bool
	raw      bool
}

func parseFlags(cmd *cobra.Command) (flags, error) {
	getStr := func(name string) (string, error) { return cmd.Flags().GetString(name) }
	getBool := func(name string) (bool, error) { return cmd.Flags().GetBool(name) }

	model, err := getStr("model")
	if err != nil {
		return flags{}, err
	}
	noStream, err := getBool("no-stream")
	if err != nil {
		return flags{}, err
	}
	raw, err := getBool("raw")
	if err != nil {
		return flags{}, err
	}
	return flags{model, noStream, raw}, nil
}

func addCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("model", "m", "", "provider/model")
	cmd.Flags().Bool("no-stream", false, "Disable streaming output")
	cmd.Flags().BoolP("raw", "r", false, "Return raw model output")
}

func (cli *CLI) resolve(modelFlag string) (provider, model string, p providers.Provider, err error) {
	model = modelFlag
	if model == "" {
		if model, err = config.GetDefaultModel(); err != nil {
			return
		}
		if model == "" {
			err = errors.New("no default model\n\nSet default: q default set --model provider/model\nOr specify: q --model provider/model")
			return
		}
	}

	parts := strings.SplitN(model, "/", 2)
	if len(parts) != 2 {
		err = errors.New("invalid model format\n\nUse: provider/model (e.g., openai/gpt-4o)")
		return
	}
	provider, model = parts[0], parts[1]

	var ok bool
	if p, ok = cli.registry.Lookup(provider); !ok {
		err = fmt.Errorf("unknown provider: %s\n\nSee available: q models list", provider)
		return
	}
	if !slices.Contains(p.SupportedModels(), model) {
		err = fmt.Errorf("unsupported model '%s' for %s\n\nSee available: q models list", model, provider)
		return
	}

	key, keyErr := config.GetAPIKey(provider)
	switch {
	case keyErr != nil:
		err = fmt.Errorf("failed to read API key for %s: %w", provider, keyErr)
	case key == "":
		err = fmt.Errorf("no API key for %s\n\nSet key: q keys set --provider %s --key KEY", provider, provider)
	}
	return
}

func executePrompt(ctx context.Context, p providers.Provider, provider, model, prompt string, raw, stream bool) error {
	if stream {
		if !raw {
			writePrefix(provider, model)
		}
		if _, err := p.Stream(ctx, model, prompt); err != nil {
			return err
		}
		if !raw {
			fmt.Println()
		}
		return nil
	}

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

func chatLoop(ctx context.Context, p providers.Provider, provider, model string, raw, stream bool) error {
	reader := bufio.NewReader(os.Stdin)
	first := true

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !first && !raw {
			fmt.Println()
		}
		first = false

		if !raw {
			fmt.Print("you: ")
		}
		text, err := reader.ReadString('\n')
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		if !raw {
			writePrefix(provider, model)
		}

		if stream {
			if _, err = p.ChatStream(ctx, model, text); err != nil {
				return err
			}
		} else {
			resp, err := p.ChatPrompt(ctx, model, text)
			if err != nil {
				return err
			}
			if raw {
				fmt.Print(resp)
			} else {
				fmt.Print(resp)
			}
		}
		fmt.Println()
	}
}

func (cli *CLI) rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "q [prompt]",
		Short:        "LLM in the Shell",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			f, err := parseFlags(cmd)
			if err != nil {
				return err
			}

			prompt := args[0]
			if prompt == "-" {
				prompt, err = promptFromStdin()
				if err != nil {
					return err
				}
			}

			provider, model, p, err := cli.resolve(f.model)
			if err != nil {
				return err
			}

			ctx := contextWithInterrupt()
			return executePrompt(ctx, p, provider, model, prompt, f.raw, !f.noStream)
		},
	}
	addCommonFlags(cmd)
	return cmd
}

func (cli *CLI) chatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "chat",
		Short:        "Start interactive REPL with a model",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := parseFlags(cmd)
			if err != nil {
				return err
			}

			provider, model, p, err := cli.resolve(f.model)
			if err != nil {
				return err
			}

			ctx := contextWithInterrupt()
			return chatLoop(ctx, p, provider, model, f.raw, !f.noStream)
		},
	}
	addCommonFlags(cmd)
	return cmd
}

func (cli *CLI) modelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "models",
		Short:        "List available provider/model combinations",
		SilenceUsage: true,
		RunE: func(*cobra.Command, []string) error {
			for _, providerName := range cli.registry.Names() {
				provider, _ := cli.registry.Lookup(providerName)
				for _, model := range provider.SupportedModels() {
					fmt.Printf("%s/%s\n", providerName, model)
				}
			}
			return nil
		},
	}
}

func (cli *CLI) keysCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "keys", Short: "Manage API keys"}

	list := &cobra.Command{
		Use:          "list",
		Short:        "List which providers have keys set",
		SilenceUsage: true,
		RunE: func(*cobra.Command, []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}
			for _, providerName := range cli.registry.Names() {
				status := "❌"
				if cfg.APIKeys[providerName] != "" {
					status = "✅"
				}
				fmt.Printf("%s: %s\n", providerName, status)
			}
			return nil
		},
	}

	set := &cobra.Command{
		Use:          "set",
		Short:        "Set API key for a provider",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			providerName, _ := cmd.Flags().GetString("provider")
			key, _ := cmd.Flags().GetString("key")

			switch {
			case providerName == "":
				_ = cmd.Help()
				return errors.New("provider required")
			case key == "":
				_ = cmd.Help()
				return errors.New("API key required")
			}

			if _, ok := cli.registry.Lookup(providerName); !ok {
				return fmt.Errorf("unknown provider: %s\n\nSee available: q models list", providerName)
			}
			if err := config.SetAPIKey(providerName, key); err != nil {
				return err
			}
			fmt.Printf("Saved key for %s\n", providerName)
			return nil
		},
	}
	set.Flags().StringP("provider", "p", "", "provider name")
	set.Flags().StringP("key", "k", "", "API key")

	path := &cobra.Command{
		Use:          "path",
		Short:        "Show path to API key config file",
		SilenceUsage: true,
		RunE: func(*cobra.Command, []string) error {
			configPath, err := config.ConfigPath()
			if err != nil {
				return err
			}
			fmt.Println(configPath)
			return nil
		},
	}

	cmd.AddCommand(list, set, path)
	return cmd
}

func (cli *CLI) defaultCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "default", Short: "Manage default model"}

	list := &cobra.Command{
		Use:          "list",
		Short:        "Show default model",
		SilenceUsage: true,
		RunE: func(*cobra.Command, []string) error {
			model, err := config.GetDefaultModel()
			if err != nil {
				return err
			}
			if model == "" {
				return errors.New("no default model set")
			}
			fmt.Println(model)
			return nil
		},
	}

	set := &cobra.Command{
		Use:          "set",
		Short:        "Set default model",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			model, _ := cmd.Flags().GetString("model")
			if model == "" {
				_ = cmd.Help()
				return errors.New("model must be provided with --model flag")
			}

			parts := strings.SplitN(model, "/", 2)
			if len(parts) != 2 {
				return errors.New("invalid model format\n\nUse: provider/model (e.g., openai/gpt-4o)")
			}

			providerName, modelName := parts[0], parts[1]
			provider, ok := cli.registry.Lookup(providerName)
			switch {
			case !ok:
				return fmt.Errorf("unknown provider: %s\n\nSee available: q models list", providerName)
			case !slices.Contains(provider.SupportedModels(), modelName):
				return fmt.Errorf("unsupported model '%s' for %s\n\nSee available: q models list", modelName, providerName)
			}

			if err := config.SetDefaultModel(model); err != nil {
				return err
			}
			fmt.Printf("Saved default model: %s\n", model)
			return nil
		},
	}
	set.Flags().StringP("model", "m", "", "provider/model")

	cmd.AddCommand(list, set)
	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "version",
		Short:        "Show version information",
		SilenceUsage: true,
		RunE: func(*cobra.Command, []string) error {
			fmt.Printf("q version %s\ncommit: %s\ndate: %s\n", version, commit, date)
			return nil
		},
	}
}

func (cli *CLI) root() *cobra.Command {
	r := cli.rootCmd()
	r.AddCommand(
		cli.chatCmd(),
		cli.modelsCmd(),
		cli.keysCmd(),
		cli.defaultCmd(),
		versionCmd(),
	)
	return r
}

func run() error { return NewCLI().root().Execute() }

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}
