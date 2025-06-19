package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"q/internal/config"
	"q/internal/google"
	"q/internal/openai"
	"q/internal/providers"
	"slices"
)

var (
	modelName string
	noStream  bool
)

// rootCmd handles one-shot prompts.
var rootCmd = &cobra.Command{
	Use:   "q [prompt]",
	Short: "Run a one-shot prompt with a model",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			return
		}
		model := modelName
		if model == "" {
			var err error
			model, err = config.GetDefaultModel()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error loading default: %v\n", err)
				os.Exit(1)
			}
			if model == "" {
				fmt.Fprintln(os.Stderr, "no default model set; use 'q default set --model provider/model'")
				os.Exit(1)
			}
		}
		parts := strings.SplitN(model, "/", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "invalid model format; use provider/model\n")
			os.Exit(1)
		}
		provider, mdl := parts[0], parts[1]
		p, ok := providers.Get(provider)
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown provider: %s\n", provider)
			os.Exit(1)
		}
		if !slices.Contains(p.SupportedModels(), mdl) {
			fmt.Fprintf(os.Stderr, "unsupported model for %s: %s\n", provider, mdl)
			os.Exit(1)
		}
		key, err := config.GetAPIKey(provider)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading keys: %v\n", err)
			os.Exit(1)
		}
		if key == "" {
			fmt.Fprintf(os.Stderr, "no API key set for %s; use 'q keys set --provider %s --key KEY'\n", provider, provider)
			os.Exit(1)
		}
		if !noStream {
			if streamer, ok := p.(interface{ Stream(string, string) error }); ok {
				fmt.Printf("model (%s/%s): ", provider, mdl)
				if err := streamer.Stream(mdl, args[0]); err != nil {
					fmt.Fprintf(os.Stderr, "error during prompt: %v\n", err)
					os.Exit(1)
				}
				fmt.Println()
				return
			}
		}
		resp, err := p.Prompt(mdl, args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error during prompt: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("model (%s/%s): %s\n", provider, mdl, resp)
	},
}

// chatCmd starts the interactive REPL.
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start interactive REPL with a model",
	Run: func(cmd *cobra.Command, args []string) {
		model := modelName
		if model == "" {
			var err error
			model, err = config.GetDefaultModel()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error loading default: %v\n", err)
				os.Exit(1)
			}
			if model == "" {
				fmt.Fprintln(os.Stderr, "no default model set; use 'q default set --model provider/model'")
				os.Exit(1)
			}
		}
		parts := strings.SplitN(model, "/", 2)
		provider, mdl := parts[0], parts[1]
		p, ok := providers.Get(provider)
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown provider: %s\n", provider)
			os.Exit(1)
		}
		if !slices.Contains(p.SupportedModels(), mdl) {
			fmt.Fprintf(os.Stderr, "unsupported model for %s: %s\n", provider, mdl)
			os.Exit(1)
		}
		key, err := config.GetAPIKey(provider)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading keys: %v\n", err)
			os.Exit(1)
		}
		if key == "" {
			fmt.Fprintf(os.Stderr, "no API key set for %s; use 'q keys set --provider %s --key KEY'\n", provider, provider)
			os.Exit(1)
		}
		if !noStream {
			reader := bufio.NewReader(os.Stdin)
			for {
				fmt.Print("you: ")
				text, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						return
					}
					fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
					os.Exit(1)
				}
				text = strings.TrimSpace(text)
				if text == "" {
					continue
				}
				fmt.Printf("model (%s/%s): ", provider, mdl)
				if streamer, ok := p.(interface{ Stream(string, string) error }); ok {
					if err := streamer.Stream(mdl, text); err != nil {
						fmt.Fprintf(os.Stderr, "error during chat: %v\n", err)
						os.Exit(1)
					}
					fmt.Println()
				} else {
					resp, err := p.Prompt(mdl, text)
					if err != nil {
						fmt.Fprintf(os.Stderr, "error during chat: %v\n", err)
						os.Exit(1)
					}
					fmt.Printf("%s\n", resp)
				}
			}
		}
		if err := p.Chat(mdl); err != nil {
			fmt.Fprintf(os.Stderr, "error during chat: %v\n", err)
			os.Exit(1)
		}
	},
}

// noun-first command structure to follow GitHub CLI style
var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage models",
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available provider/model combinations",
	Run: func(cmd *cobra.Command, args []string) {
		for _, pr := range providers.Providers() {
			p, _ := providers.Get(pr)
			for _, m := range p.SupportedModels() {
				fmt.Printf("%s/%s\n", pr, m)
			}
		}
	},
}

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage API keys",
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List which providers have keys set",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
			os.Exit(1)
		}
		for _, pr := range providers.Providers() {
			status := "❌"
			if k := cfg.APIKeys[pr]; k != "" {
				status = "✅"
			}
			fmt.Printf("%s: %s\n", pr, status)
		}
	},
}

var keysSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set API key for a provider",
	Run: func(cmd *cobra.Command, args []string) {
		provider, _ := cmd.Flags().GetString("provider")
		key, _ := cmd.Flags().GetString("key")
		if provider == "" || key == "" {
			cmd.Help()
			os.Exit(1)
		}
		if err := config.SetAPIKey(provider, key); err != nil {
			fmt.Fprintf(os.Stderr, "error saving key: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Saved key for %s\n", provider)
	},
}

var keysPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show path to API key config file",
	Run: func(cmd *cobra.Command, args []string) {
		path, err := config.ConfigPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error getting config path: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(path)
	},
}

var defaultCmd = &cobra.Command{
	Use:   "default",
	Short: "Manage default model",
}

var defaultListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show default model",
	Run: func(cmd *cobra.Command, args []string) {
		m, err := config.GetDefaultModel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading default: %v\n", err)
			os.Exit(1)
		}
		if m == "" {
			fmt.Fprintln(os.Stderr, "no default model set")
			os.Exit(1)
		}
		fmt.Println(m)
	},
}

var defaultSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set default model",
	Run: func(cmd *cobra.Command, args []string) {
		m, _ := cmd.Flags().GetString("model")
		if m == "" {
			cmd.Help()
			os.Exit(1)
		}
		if err := config.SetDefaultModel(m); err != nil {
			fmt.Fprintf(os.Stderr, "error saving default: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Saved default model: %s\n", m)
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&noStream, "no-stream", false, "Disable streaming output")
	rootCmd.PersistentFlags().BoolVar(&noStream, "ns", false, "Alias for --no-stream")
	rootCmd.PersistentFlags().StringVarP(&modelName, "model", "m", "", "provider/model")
	chatCmd.Flags().StringVarP(&modelName, "model", "m", "", "provider/model")

	keysSetCmd.Flags().String("provider", "", "provider name")
	keysSetCmd.Flags().String("key", "", "API key")

	defaultSetCmd.Flags().String("model", "", "provider/model")

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
