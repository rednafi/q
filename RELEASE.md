# Release process

This project uses GitHub Actions and GoReleaser to automatically build and release
the CLI tool.
The project is built with Go 1.24 and uses modern Go features like the `slices` package
and improved performance optimizations.

## Prerequisites

### GitHub secrets

You need to set up the following secrets in your GitHub repository:

1. **`GITHUB_TOKEN`** - Automatically provided by GitHub Actions
   (no additional setup needed)

### Homebrew formula

The Homebrew formula is automatically generated and stored in the `Formula/` directory
of this repository.

## Release process

### Automatic release

1. **Tag a release**: Create and push a tag starting with `v` (e.g., `v1.0.0`)
   ```sh
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **Workflow triggers**: The build workflow will automatically run when:
   - A tag is pushed (immediate release)
   - The Test and Lint workflows complete successfully (after tests pass)

3. **What happens**:
   - GoReleaser builds the CLI for multiple platforms (Linux, macOS, Windows)
     using Go 1.24
   - Creates a GitHub release with the binaries
   - Updates the Homebrew formula in the `Formula/` directory

### Manual release

You can also trigger a release manually:
1. Go to the Actions tab in your GitHub repository
2. Select the "Build and Release" workflow
3. Click "Run workflow" and select the branch
