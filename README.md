# q

LLM in your shell.


## Features

- **Multi-model support**: I got tired after trying to make Gemini streaming API work. So it's only OpenAI for now.
But extending for other vendors is easy (maybe).
- **Streaming responses**: Watch responses appear in real-time
- **Interactive chat mode**: Have conversations with your language models
- **One-shot prompts**: Quick questions without starting a chat session
- **Raw output mode**: Get clean, unformatted responses for scripting
- **Stdin support**: Pipe input directly to the model
- **Smart defaults**: Set your preferred model and forget about it
- **Simple setup**: Just add your API key and you're ready to go

## Quick start

### Installation

```sh
# Download the latest release for your platform
# macOS
brew install rednafi/q

# Or download directly from GitHub releases
# https://github.com/rednafi/q/releases
```

### Setup

1. **Get your API key**:
   - [OpenAI API Key](https://platform.openai.com/api-keys)

2. **Configure your key**:
   ```sh
   q keys set -p openai -k sk-your-openai-key
   ```

3. **List the available models**:
   ```sh
   q models list
   ```

4. **Set your default model**:
   ```sh
   q default set -m openai/gpt-4o
   ```

5. **Start chatting**:
   ```sh
   q "What's the weather like today?"
   ```

## Usage

### One-shot prompts

Ask quick questions without starting a chat session:

```sh
# Use default model
q "Explain quantum computing in simple terms"

# Specify a model
q -m openai/gpt-4o "Write a Python function to sort a list"

# Disable streaming (get response all at once)
q --no-stream "What are the benefits of meditation?"

# Get raw output (no formatting)
q -r "Return only a JSON object with name and age"
```

### Reading from stdin

Pipe input directly to the model:

```sh
# Pipe text to the model
echo "Convert this to uppercase" | q -

# Use with raw output for scripting
echo "this [unstructured] data [should] be structured. first word should be \
  key and the [] word should be value. return only json and no other text, not \
  even json fence" | q -r - | jq

# Process files
cat data.txt | q -r -m openai/gpt-4o "Summarize this text in one sentence"
```

### Interactive chat mode

Start a conversation with your AI model:

```sh
# Use default model
q chat

# Disable streaming
q chat --no-stream

# Raw output mode (no "you:" or "model:" prefixes)
q chat -r
```

### Raw output mode

Get clean, unformatted responses perfect for scripting and automation:

```sh
# Regular output
q "What's 2+2?"
# Output: model (openai/gpt-4o): 2+2 equals 4

# Raw output
q -r "What's 2+2?"
# Output: 2+2 equals 4

# Perfect for JSON processing
q -r "Return a JSON object with name: John, age: 30" | jq '.name'
# Output: "John"

# Combine with stdin for powerful workflows
echo "Extract the email addresses from this text: contact@example.com and \
  support@test.org" | q -r - | grep -o '[^@]*@[^@]*'
```

### Available models

See all supported models:

```sh
q models list
```

**OpenAI models:**
- `gpt-3.5-turbo`
- `gpt-3.5-turbo-0613`
- `gpt-4o`
- `gpt-4o-mini`
- `gpt-4.1`
- `gpt-4.1-mini`
- `gpt-4.1-nano`
- `o3-mini`
- `o3`
- `o3-pro`
- `o4-mini`

## Configuration

### Managing API keys

```sh
# List configured providers
q keys list

# Set an API key
q keys set -p openai -k sk-your-key-here

# Show config file location
q keys path
```

### Default model management

```sh
# Show current default
q default list

# Set a new default
q default set -m openai/gpt-4o
```

## Command reference

### Commands
- `q [prompt]`: Send a one-shot prompt
  - `--no-stream`: Disable streaming output
  - `--raw, -r`: Return raw model output (no formatting)
  - `-`: Read prompt from stdin
- `q chat`: Start interactive chat mode
  - `--no-stream`: Disable streaming output
  - `--raw, -r`: Return raw model output (no "you:" or "model:" prefixes)
- `q models list`: List all available models
- `q keys list`: Show configured API keys
- `q keys set -p <name> -k <key>`: Set API key (or `--provider` and `--key`)
- `q keys path`: Show config file location
- `q default list`: Show current default model
- `q default set -m <model>`: Set default model (or `--model`)
- `q version`: Show version information


## Why?

There's no shortage of wrappers that call the language model from your terminal. I wanted to have my own that's written in Go :)
