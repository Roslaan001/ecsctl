# Uninstallation

If you need to remove `ecsctl` from your machine, follow the instructions below for your operating system.

---

## macOS / Linux

To remove the `ecsctl` binary and configuration files:

```bash
# If installed globally (default):
sudo rm -f /usr/local/bin/ecsctl

# If installed in local user directory:
rm -f $HOME/.local/bin/ecsctl
```

---

## Windows

Run the following command in **PowerShell** to delete the `ecsctl` installation folder and binary:

```powershell
# Remove installation directory and binary
Remove-Item -Recurse -Force $HOME\.ecsctl
```

---

## Cleaning up Shell Completions (Optional)

If you generated autocompletions for your shell, you can remove them as well:

* **Bash**: `sudo rm -f /etc/bash_completion.d/ecsctl`
* **Zsh**: `rm -f ~/.zsh/completion/_ecsctl`
* **Fish**: `rm -f ~/.config/fish/completions/ecsctl.fish`
