# Installation Guide

This guide covers installation, configuration, and troubleshooting for AF (Adversarial Proof Framework).

## System Requirements

### Go Version

AF requires **Go 1.22 or later**. The project uses Go modules and modern language features.

To check your Go version:

```bash
go version
```

### Operating Systems

AF is a pure Go application with no CGO dependencies, making it portable across:

- **Linux**: Any modern distribution (Ubuntu, Debian, Fedora, Arch, etc.)
- **macOS**: 10.15 (Catalina) or later
- **Windows**: Windows 10/11 with Go installed

### Hardware Requirements

- **Disk Space**: ~15 MB for the compiled binary; proof workspaces vary based on proof size (typically 1-100 MB)
- **Memory**: Minimal requirements; AF uses filesystem-based storage rather than in-memory databases
- **CPU**: Any modern processor

## Installation Methods

### Method 1: Build from Source (Recommended)

Building from source gives you the latest features and allows you to verify the code.

```bash
# Clone the repository
git clone https://github.com/tobias/vibefeld.git

# Enter the directory
cd vibefeld

# Build the binary
go build ./cmd/af

# Verify the build
./af --version
```

This creates an `af` binary in the current directory.

### Method 2: go install

Install directly using Go's package manager:

```bash
go install github.com/tobias/vibefeld/cmd/af@latest
```

This installs the binary to `$GOPATH/bin` or `$HOME/go/bin` by default.

### Adding to PATH

After installation, ensure the binary is accessible from anywhere:

**Linux/macOS:**

```bash
# If built from source, move to a directory in your PATH
sudo mv af /usr/local/bin/

# Or add the build directory to PATH (add to ~/.bashrc or ~/.zshrc)
export PATH="$PATH:/path/to/vibefeld"

# If using go install, ensure GOPATH/bin is in PATH
export PATH="$PATH:$(go env GOPATH)/bin"
```

**Windows (PowerShell):**

```powershell
# Move binary to a directory in your PATH, or add to PATH via System Properties
# Typically: C:\Users\<username>\go\bin (if using go install)
```

## Verification

After installation, verify AF is working correctly:

### Check Version

```bash
af --version
```

Expected output:

```
af version 0.1.0
```

Or with build information:

```
af version dev
  Commit:  <commit-hash>
  Built:   <timestamp>
  Go:      go1.22.x
```

### Test Initialization

Create a test proof to verify full functionality:

```bash
# Create a temporary directory
mkdir /tmp/af-test
cd /tmp/af-test

# Initialize a proof
af init --claim "For all n >= 1, the sum of first n natural numbers equals n(n+1)/2"

# Check status
af status

# Clean up
cd ..
rm -rf /tmp/af-test
```

If these commands complete without errors, AF is installed correctly.

### Verify Help System

```bash
af --help
af init --help
```

## Configuration

### Proof Configuration

Each proof workspace stores its configuration in `meta.json`. This is created automatically by `af init` with sensible defaults:

| Setting | Default | Description |
|---------|---------|-------------|
| `lock_timeout` | 5m | Maximum duration a node can be claimed |
| `max_depth` | 20 | Maximum proof tree depth |
| `max_children` | 10 | Maximum children per node |
| `warn_depth` | 3 | Depth at which depth warnings appear |
| `auto_correct_threshold` | 0.8 | Fuzzy match threshold for command correction |

### Environment Variables

AF recognizes the following environment variables:

| Variable | Description |
|----------|-------------|
| `AF_AGENT_ID` | Identifies the agent when claiming nodes (optional) |

Hook execution provides additional environment variables:

| Variable | Description |
|----------|-------------|
| `AF_EVENT_TYPE` | The type of event that triggered the hook |
| `AF_NODE_ID` | The affected node ID |
| `AF_CHALLENGE_ID` | The challenge ID (if applicable) |
| `AF_TIMESTAMP` | When the event occurred |

### Global Flags

All AF commands support these global flags:

```bash
--verbose    Enable verbose output for debugging
--dry-run    Preview changes without making them
```

## Upgrading

### From Source

```bash
cd vibefeld
git pull
go build ./cmd/af
```

### Using go install

```bash
go install github.com/tobias/vibefeld/cmd/af@latest
```

### Verifying Upgrade

After upgrading, verify the new version:

```bash
af --version
```

### Proof Compatibility

AF maintains backward compatibility with proof workspaces. Existing proofs should work with new versions without migration. The `version` field in `meta.json` indicates the schema version (currently "1.0").

## Troubleshooting

### Go Not Found

**Symptom:**
```
command not found: go
```

**Solution:**
Install Go from https://go.dev/dl/ and ensure it is in your PATH:

```bash
# Linux/macOS: Add to ~/.bashrc or ~/.zshrc
export PATH="$PATH:/usr/local/go/bin"
source ~/.bashrc
```

### Go Version Too Old

**Symptom:**
```
go: requires go1.22 or later
```

**Solution:**
Upgrade Go to version 1.22 or later from https://go.dev/dl/

### Module Download Errors

**Symptom:**
```
go: github.com/spf13/cobra@v1.10.2: Get "https://proxy.golang.org/...": dial tcp: lookup proxy.golang.org: no such host
```

**Solutions:**

1. Check internet connectivity
2. If behind a proxy, configure Go proxy settings:
   ```bash
   go env -w GOPROXY=https://proxy.golang.org,direct
   ```
3. For corporate firewalls, you may need to use a private module proxy

### Permission Denied

**Symptom:**
```
permission denied: /usr/local/bin/af
```

**Solution:**
Use `sudo` when installing to system directories:

```bash
sudo mv af /usr/local/bin/
```

Or install to a user-writable location:

```bash
mkdir -p ~/bin
mv af ~/bin/
export PATH="$PATH:$HOME/bin"
```

### Binary Not Found After Installation

**Symptom:**
```
command not found: af
```

**Solutions:**

1. Verify the binary location:
   ```bash
   which af
   ls -la $(go env GOPATH)/bin/af
   ```

2. Add the binary location to PATH (see "Adding to PATH" section)

3. Restart your terminal or source your shell configuration:
   ```bash
   source ~/.bashrc  # or ~/.zshrc
   ```

### Proof Directory Errors

**Symptom:**
```
Error: not a proof directory (or any parent up to filesystem root)
```

**Solution:**
Run `af init` to create a new proof, or navigate to an existing proof directory:

```bash
# Create new proof
af init --claim "Your theorem statement"

# Or find existing proofs
find ~ -name "meta.json" -path "*/.af/*" 2>/dev/null
```

### Lock/Concurrency Issues

**Symptom:**
```
Error: node 1.2 is locked by another process
```

**Solutions:**

1. Wait for the lock to expire (default: 5 minutes)
2. Use `af reap` to clean up stale locks:
   ```bash
   af reap
   ```
3. Check for zombie processes holding locks

### Corrupt Ledger

**Symptom:**
```
Error: ledger corrupted at sequence 42
```

**Solution:**
The ledger is append-only and generally self-healing. Try:

```bash
af replay --verify
af health
```

If corruption persists, you may need to restore from backup or start a new proof.

## Uninstalling

### Remove Binary

**Linux/macOS:**

```bash
# If installed to /usr/local/bin
sudo rm /usr/local/bin/af

# If installed via go install
rm $(go env GOPATH)/bin/af

# If built from source
rm /path/to/vibefeld/af
```

**Windows:**

Delete the `af.exe` file from its installation location.

### Remove Source Code

```bash
rm -rf /path/to/vibefeld
```

### Remove Proof Workspaces

Proof workspaces are stored wherever you created them. Each contains a `.af/` directory with:

- `ledger.jsonl` - Event history
- `meta.json` - Configuration
- `locks/` - Lock files

To find and optionally remove proof directories:

```bash
# Find all proof directories
find ~ -type d -name ".af" 2>/dev/null

# Remove a specific proof (be careful!)
rm -rf /path/to/proof/.af
```

### Clean Go Module Cache (Optional)

If you want to completely remove cached dependencies:

```bash
go clean -modcache
```

Note: This removes cached modules for ALL Go projects, not just AF.

## Getting Help

- Run `af --help` for command overview
- Run `af <command> --help` for command-specific help
- Use `af tutorial` for a step-by-step guide
- Check the project README for workflow examples

## Next Steps

After installation, try:

1. `af init --claim "Your theorem"` - Start a new proof
2. `af status` - View proof state
3. `af tutorial` - Learn the workflow
4. `af --help` - Explore available commands
