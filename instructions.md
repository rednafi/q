* q "prompt" [--no-stream | --ns]

  * runs a one-shot prompt using the default model
  * streams response tokens as they arrive (prefixing with model (model-name):)
  * use `--no-stream` / `--ns` to disable streaming and wait for full response

* q "prompt" --model model-name or -m model-name [--no-stream | --ns]

  * runs a one-shot prompt using the specified model (streaming by default)

* q chat [--no-stream | --ns]

  * starts an interactive REPL with streaming responses using the default model
  * ctrl + c to exit
  * prompt: you: ...
  * response: model (model-name): ...

* q chat --model model-name or -m model-name [--no-stream | --ns]

  * interactive REPL with streaming responses using the specified model

* q models list

  * prints available model identifiers (e.g., openai/gpt-4o)
  * used for reference or scripting

* q keys set --provider provider-name --key key

  * sets and saves an API key for a model provider
  * only allowed outside of q "prompt" or q chat invocations
  * keys stored at \$XDG\_CONFIG\_HOME/q/config.json in JSON format
  * format: { "openai": "..." }


* q keys list

  * lists which providers have keys set
  * ✅ = key present, ❌ = key missing

* q keys path

  * show the filesystem path to the API key config file


* q default set model-name

  * sets the default model for chat or one-shot mode
  * stored at \$XDG\_CONFIG\_HOME/q/default.json

* q default list

  * show current default model

* plugin architecture

  * each model implements a Model interface:

    * Name() string
    * Prompt(prompt string) (string, error)
    * Chat() error
  * models are registered explicitly in main setup (no init functions)
  * model name must be unique and match convention provider/model-name

* http calls

  * providers receive an HTTPClient dependency via constructor for HTTP requests
  * default HTTP client (http.DefaultClient) is used when none is specified
  * enables mocking and testing without global state

* output format

  * all chat and prompt responses are prefixed by: model (model-name):

* behavior

* any command other than `q models list`, `q keys list`, `q keys set`, `q keys path`, and `q default set` requires valid key to be set for target model

* config and key persistence

  * config saved to \$XDG\_CONFIG\_HOME/q/
  * separate JSON files for keys and default model

* testing

  * go test ./...
  * providers and plugin tests inject HTTPClient dependencies instead of patching global state
  * use os/exec to simulate CLI behavior in test scripts
  * aim for full coverage of all commands and plugin integrations

* github ci

  * triggers on push and pull\_request
  * runs `go test ./...`
  * runs `golangci-lint run`

* linter

  * golangci-lint integrated via GitHub Actions
  * enforced on all pushes and PRs

* readme

  * includes install, usage examples, plugin dev guide, and test instructions
  * mentions file locations, supported models, and behavior expectations

* one-shot mode

  * supports q "..." syntax without requiring subcommands
  * follows Unix principle of doing one thing and piping output
  * model must be passed with --model or default must be set

* if no model specified and no default set

  * return clear error: "no default model set; use --model or q default set ..."

* help command

  * show basic usage if no args or help flags passed
  * `q help` or `q --help` returns list of commands and options
