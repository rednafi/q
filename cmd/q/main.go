package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"q/internal/config"
	"q/internal/providers"
	"q/internal/providers/google"
	"q/internal/providers/openai"
	"slices"
)

var (
	modelName string
	noStream  bool
)

// rootCmd handles one-shot prompts.
var rootCmd = &cobra.Command{
	Use:          "q [prompt]",
	Short:        "Run a one-shot prompt with a model",
	Args:         cobra.ArbitraryArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		model := modelName
		if model == "" {
			m, err := config.GetDefaultModel()
			if err != nil {
				return fmt.Errorf("error loading default: %w", err)
			}
			if m == "" {
				return errors.New("no default model set; use 'q default set provider/model'")
			}
			model = m
		}
		parts := strings.SplitN(model, "/", 2)
		if len(parts) != 2 {
			return errors.New("invalid model format; use provider/model")
		}
		provider, mdl := parts[0], parts[1]
		p, ok := providers.Get(provider)
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

// chatCmd starts the interactive REPL.
var chatCmd = &cobra.Command{
	Use:          "chat",
	Short:        "Start interactive REPL with a model",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		model := modelName
		if model == "" {
			m, err := config.GetDefaultModel()
			if err != nil {
				return fmt.Errorf("error loading default: %w", err)
			}
			if m == "" {
				return errors.New("no default model set; use 'q default set provider/model'")
			}
			model = m
		}
		parts := strings.SplitN(model, "/", 2)
		provider, mdl := parts[0], parts[1]
		p, ok := providers.Get(provider)
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

// noun-first command structure to follow GitHub CLI style
var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage models",
}

var modelsListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List available provider/model combinations",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, pr := range providers.Providers() {
			p, _ := providers.Get(pr)
			for _, m := range p.SupportedModels() {
				fmt.Printf("%s/%s\n", pr, m)
			}
		}
		return nil
	},
}

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage API keys",
}

var keysListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List which providers have keys set",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}
		for _, pr := range providers.Providers() {
			status := "❌"
			if k := cfg.APIKeys[pr]; k != "" {
				status = "✅"
			}
			fmt.Printf("%s: %s\n", pr, status)
		}
		return nil
	},
}

var keysSetCmd = &cobra.Command{
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

var keysPathCmd = &cobra.Command{
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

var defaultCmd = &cobra.Command{
	Use:   "default",
	Short: "Manage default model",
}

var defaultListCmd = &cobra.Command{
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

var defaultSetCmd = &cobra.Command{
	Use:          "set [model]",
	Short:        "Set default model",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := args[0]
		if err := config.SetDefaultModel(m); err != nil {
			return fmt.Errorf("error saving default: %w", err)
		}
		fmt.Printf("Saved default model: %s\n", m)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&noStream, "no-stream", false, "Disable streaming output")
	rootCmd.PersistentFlags().BoolVar(&noStream, "ns", false, "Alias for --no-stream")
	rootCmd.PersistentFlags().StringVarP(&modelName, "model", "m", "", "provider/model")
	chatCmd.Flags().StringVarP(&modelName, "model", "m", "", "provider/model")

	keysSetCmd.Flags().String("provider", "", "provider name")
	keysSetCmd.Flags().String("key", "", "API key")

	rootCmd.AddCommand(chatCmd, modelsCmd, keysCmd, defaultCmd)
	modelsCmd.AddCommand(modelsListCmd)
	keysCmd.AddCommand(keysListCmd, keysSetCmd, keysPathCmd)
	defaultCmd.AddCommand(defaultListCmd, defaultSetCmd)
}

func main() {
	// explicit plugin registration
	providers.Register(openai.New())
	providers.Register(google.New())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
