# lazyCommit CLI Guide (v0.1)

A quick reference for managing the lazyCommit daemon and its automated Git behaviors.

## Command Reference

| Command | Feature | Description |
| :--- | :--- | :--- |
| `lazycommit start` | **Daemon Lifecycle** | Launches the daemon in the background and initializes repository watchers. |
| `lazycommit stop` | **Daemon Lifecycle** | Safely terminates the running background daemon using the PID file. |
| `lazycommit status` | **Monitoring** | Checks if the daemon is active and displays the current process ID (PID). |
| `lazycommit daemon` | **Debug/Service** | Runs the daemon in the foreground (internal use / container entrypoint). |
| `lazycommit scan [dir]` | **Discovery** | Scans for Git repositories downwards from the specified directory. |
| `lazycommit scan-all [dir]` | **Discovery** | Scans for Git repositories including parent directories of the specified path. |
| `git commit ...` | **Lazy Push** | (Trigger) Making a manual commit triggers the auto-push sequence. |

---

## Feature Deep Dive

### 🚀 Lazy Push (v0.1 Core)
The primary feature of the current version. It bridges the gap between `git commit` and `git push`.

*   **Trigger:** Any change to `.git/refs/heads/` (new commits, branch switches).
*   **Debouncing:** If multiple commits are made within the delay window, the timer resets. Only one push is executed for the entire batch.
*   **Default Delay:** 5 seconds (Configurable via `push_delay_seconds`).
*   **Authentication:** Automatically detects and uses SSH keys from `~/.ssh/` if an SSH agent is not present.

### 🛠 Configuration (`~/.lazycommit/config.toml`)
Control which repositories are watched:

```toml
[[repos]]
path = "/path/to/your/repo"
enabled = true
lazy_push = true
```

---

## Troubleshooting v0.1

| Issue | Solution |
| :--- | :--- |
| **Daemon won't start** | Check `~/.lazycommit/logs/daemon.stderr` for config parsing errors. |
| **Push fails (Auth)** | Ensure your SSH key is at `~/.ssh/id_ed25519` or `~/.ssh/id_rsa`. |
| **Changes not detected** | Ensure the `path` in `config.toml` is an absolute path. |
| **Stale PID file** | If the daemon crashed, run `rm ~/.lazycommit/daemon.pid` then `start`. |
