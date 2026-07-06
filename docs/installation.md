# Installation

There are several ways to install `ecsctl` depending on your environment and preferences.

---

## 1. Quick Install Script (Recommended)

=== "macOS / Linux"

    To download and install the latest precompiled release binary automatically, run:

    ```bash
    curl -fsSL https://raw.githubusercontent.com/Roslaan001/ecsctl/main/install.sh | sh
    ```

    #### Install a Specific Version
    If you wish to install a specific tag/version, pass it as an argument:

    ```bash
    curl -fsSL https://raw.githubusercontent.com/Roslaan001/ecsctl/main/install.sh | sh -s -- v0.1.0
    ```

    #### Non-Root Installation
    If your user doesn't have root permissions and `sudo` is not available, the script will automatically fallback to installing inside `$HOME/.local/bin`. Make sure to add this path to your shell profile (e.g., `~/.bashrc` or `~/.zshrc`):

    ```bash
    export PATH="$HOME/.local/bin:$PATH"
    ```

=== "Windows"

    To download and install the latest precompiled release binary automatically on Windows, run the following command in **PowerShell**:

    ```powershell
    irm https://raw.githubusercontent.com/Roslaan001/ecsctl/main/install.ps1 | iex
    ```

    This script automatically:
    * Resolves the latest version of `ecsctl`.
    * Detects your system architecture (AMD64 or ARM64).
    * Downloads and extracts the official `.zip` archive.
    * Copies `ecsctl.exe` to `$HOME/.ecsctl/bin`.
    * Appends the directory to your user `PATH` environment variable.


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

# Install the binary globally to $GOPATH/bin
make install
```

---

## Prerequisites

Before running `ecsctl`, make sure you have set up the following prerequisites:

### AWS Credentials
`ecsctl` uses the default AWS SDK credential provider chain. You can configure credentials using:
1. **Environment Variables**:
   ```bash
   export AWS_ACCESS_KEY_ID="AKIA..."
   export AWS_SECRET_ACCESS_KEY="wJalr..."
   export AWS_DEFAULT_REGION="us-east-1"
   ```
2. **Shared Credentials File** (`~/.aws/credentials` and `~/.aws/config`):
   Set up your profiles using the `aws configure` command. You can pass the `--profile` flag to `ecsctl` commands to use a specific profile.

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
  sudo yum install -o "session-manager-plugin.rpm"
  ```
