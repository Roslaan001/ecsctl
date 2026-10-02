# Installation

Choose the installation method that matches your machine. The installer downloads a prebuilt binary and verifies it against the release's SHA-256 checksums; `go install` and building from source require Go.

---

## Install with the setup script

=== "macOS / Linux"

    To download and install the latest precompiled release binary automatically, run:

    ```bash
    curl -fsSL https://roslaan001.github.io/ecsctl/install.sh | sh
    ```

    #### Install a specific version
    To install a tagged release instead of the latest release, pass its tag:

    ```bash
    curl -fsSL https://roslaan001.github.io/ecsctl/install.sh | sh -s -- v0.2.1
    ```

    Set `ECSCTL_INSTALL_DIR` to choose an install directory. This is useful for
    CI and isolated installs; for example:

    ```bash
    ECSCTL_INSTALL_DIR="$HOME/.local/bin" sh install.sh
    ```

    #### Install without administrator access
    If the script cannot write to `/usr/local/bin`, it installs the binary in `$HOME/.local/bin`. Add that directory to your `PATH` if it is not already there. For the current shell, run:

    ```bash
    export PATH="$HOME/.local/bin:$PATH"
    ```

=== "Windows"

    To download and install the latest precompiled release binary automatically on Windows, run the following command in **PowerShell**:

    ```powershell
    irm https://roslaan001.github.io/ecsctl/install.ps1 | iex
    ```

    The script automatically:
    * Resolves the latest version of `ecsctl`.
    * Detects your system architecture (AMD64 or ARM64).
    * Downloads and extracts the official `.zip` archive.
    * Copies `ecsctl.exe` to `$HOME/.ecsctl/bin`.
    * Appends the directory to your user `PATH` environment variable.

After installing, open a new terminal if your `PATH` changed and confirm the command is available:

```bash
ecsctl version
```

## Verify release provenance

Starting with v0.2.3, release archives include a GitHub artifact attestation
that links each archive to the repository and workflow that built it. After
downloading an archive, verify its provenance with the GitHub CLI:

```bash
gh attestation verify ecsctl_0.2.3_linux_amd64.tar.gz --repo Roslaan001/ecsctl
```

Replace the example filename with the archive for your operating system and
architecture. The installer also checks the archive's SHA-256 checksum.


---

## 2. Using `go install`

If you have Go installed on your system, you can build and install the latest release directly via Go's package manager:

```bash
go install github.com/roslaan001/ecsctl@latest
```

Ensure your Go binary path (`$GOPATH/bin` or `$HOME/go/bin`) is included in your system's `PATH`.

---

## 3. Building From Source

For development or compiling manually, clone the repository and use the provided `Makefile`:

```bash
# Clone the repository
git clone https://github.com/Roslaan001/ecsctl.git
cd ecsctl

# Build the binary to ./bin/ecsctl
make build

# Install the binary to your Go bin directory
make install
```

---

## Configure AWS access

Installing ecsctl does not require AWS credentials. To run commands that create, inspect, or change ECS resources, configure AWS access for the account and Region you intend to use. Your credentials must have permission for the action; ecsctl cannot grant AWS permissions.

ecsctl uses the standard AWS SDK credential chain. It can use credentials from an AWS profile, environment variables, or an attached IAM role. Follow your organization's approved sign-in method. For a named local profile, select it for the current shell:

```bash
export AWS_PROFILE=development
export AWS_REGION=eu-west-2
ecsctl list clusters
```

You can also select a profile for one command with `--profile development`, and select a Region with `--region eu-west-2`. Avoid putting long-lived AWS secret keys directly in commands or documentation. See the [AWS SDK credential provider chain](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html) for supported credential sources.

### ECS Exec Plugin (For Container Shell Access)
To use `ecsctl exec` (which allows you to run interactive shells inside ECS Fargate or EC2 containers), you must install the **AWS Session Manager Plugin** on your local machine.

* **macOS (via Homebrew)**:
  ```bash
  brew install --cask session-manager-plugin
  ```
* **Linux (Ubuntu/Debian)**:
  ```bash
  curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/ubuntu_64bit/session-manager-plugin.deb" -o "session-manager-plugin.deb"
  sudo dpkg -i session-manager-plugin.deb
  ```
* **Linux (RHEL/CentOS)**:
  ```bash
  curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/linux_64bit/session-manager-plugin.rpm" -o "session-manager-plugin.rpm"
  sudo yum install ./session-manager-plugin.rpm
  ```

---

## Shell Autocompletion

`ecsctl` supports generating autocompletion scripts for Bash, Zsh, Fish, and PowerShell on the fly. 

### Bash

To configure autocomplete for Bash:

```bash
# Set up autocomplete for the current session
source <(ecsctl completion bash)

# Make autocomplete persistent across shell sessions:
# On Linux:
ecsctl completion bash | sudo tee /etc/bash_completion.d/ecsctl > /dev/null

# On macOS:
ecsctl completion bash > /usr/local/etc/bash_completion.d/ecsctl
```

### Zsh

To configure autocomplete for Zsh:

```zsh
# Set up autocomplete for the current session
source <(ecsctl completion zsh)

# Make autocomplete persistent across shell sessions:
mkdir -p ~/.zsh/completion
ecsctl completion zsh > ~/.zsh/completion/_ecsctl

# Then ensure the directory is in your fpath by adding this to ~/.zshrc:
fpath=(~/.zsh/completion $fpath)
autoload -Uz compinit && compinit
```

### Fish

To configure autocomplete for Fish:

```fish
ecsctl completion fish > ~/.config/fish/completions/ecsctl.fish
```

### PowerShell

To configure autocomplete for Windows PowerShell:

```powershell
# Set up autocomplete for the current session
ecsctl completion powershell | Out-String | Invoke-Expression

# Make autocomplete persistent across sessions:
ecsctl completion powershell >> $PROFILE
```

---

To remove ecsctl, see [Uninstallation](uninstallation.md).
