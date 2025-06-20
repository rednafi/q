# Release Process

This project uses GitHub Actions and GoReleaser to automatically build and release the CLI tool. The project is built with Go 1.24 and uses modern Go features like `slices` package and improved performance optimizations.

## Prerequisites

### GitHub Secrets

You need to set up the following secrets in your GitHub repository:

1. **`GITHUB_TOKEN`** - Automatically provided by GitHub Actions (no additional setup needed)

### Homebrew Formula

The Homebrew formula is automatically generated and stored in the `Formula/` directory of this repository.

## Release Process

### Automatic Release

1. **Tag a release**: Create and push a tag starting with `v` (e.g., `v1.0.0`)
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **Workflow triggers**: The build workflow will automatically run when:
   - A tag is pushed (immediate release)
   - The Test and Lint workflows complete successfully (after tests pass)

3. **What happens**:
   - GoReleaser builds the CLI for multiple platforms (Linux, macOS, Windows) using Go 1.24
   - Creates a GitHub release with the binaries
   - Updates the Homebrew formula in the `Formula/` directory

### Manual Release

You can also trigger a release manually:
1. Go to the Actions tab in your GitHub repository
2. Select the "Build and Release" workflow
3. Click "Run workflow" and select the branch

## Homebrew Installation

After a release, users can install the CLI via Homebrew:

```bash
# Install directly from this repository
brew install rednafi/canvas/q
```

Or if you prefer to add it as a tap first:
```bash
# Add this repository as a tap
brew tap rednafi/canvas

# Install the CLI
brew install q
```

## Configuration

The release process is configured in:
- `.github/workflows/build.yml` - GitHub Actions workflow
- `.goreleaser.yml` - GoReleaser configuration

### Supported Platforms

The CLI is built for:
- **Linux**: amd64, arm64
- **macOS**: amd64, arm64
- **Windows**: amd64, arm64

### Go 1.24 Features

This project leverages Go 1.24 features:
- **`slices` package**: For efficient slice operations like `slices.Contains` and `slices.Sort`
- **Improved performance**: Better compiler optimizations and runtime performance
- **Enhanced tooling**: Better error messages and debugging support

### Version Information

The CLI includes version information that can be displayed with:
```bash
q version
```

This shows:
- Version number (from git tag)
- Git commit hash
- Build date
