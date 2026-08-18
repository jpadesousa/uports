# uports

A CLI tool for displaying **listening TCP ports grouped by user**, together with the owning process ID and command.

`uports` uses [`gopsutil`](https://github.com/shirou/gopsutil) to inspect network connections and processes, [Cobra](https://github.com/spf13/cobra) for CLI argument handling, and [Lip Gloss](https://github.com/charmbracelet/lipgloss) for terminal styling.

## Features

* Lists TCP ports currently in the `LISTEN` state
* Groups listening ports by username
* Displays the owning process ID (PID)
* Displays the process command line
* Counts sockets belonging to the same PID/port combination
* Filter results by:

  * Port
  * PID
  * Command
  * Username or UID

* Highlights values matching active filters
* Supports full, unwrapped command output
* Sorts ports numerically and users alphabetically
* Colorized terminal output for easier scanning

## Installation

### From source

Requires a recent version of Go.

```bash
go install <your-module-path>/uports@latest
```

Replace `<your-module-path>` with the module path used by this project.

### Build locally

```bash
git clone <repository-url>
cd uports

go build -o bin/uports .
```

or

```bash
git clone <repository-url>
cd uports

make build
```

Then run:

```bash
./bin/uports
```

You can optionally install it somewhere in your `PATH`:

```bash
install uports ~/.local/bin/uports
```

## Usage

```text
uports [flags]
```

Running `uports` without arguments displays all listening TCP ports that the current process can inspect.

Example:

```bash
uports
```

Example output:

```text
root (0)
  PORT  PID    N   COMMAND
  22    812    1   /usr/sbin/sshd -D

postgres (112)
  PORT  PID    N   COMMAND
  5432  1247   1   /usr/lib/postgresql/.../postgres
```

The exact output depends on the processes and network connections available on the system.

## Options

| Flag        | Short | Description                                               |
| ----------- | ----- | --------------------------------------------------------- |
| `--port`    | `-p`  | Filter by listening TCP port                              |
| `--pid`     | `-i`  | Filter by process ID                                      |
| `--cmd`     | `-c`  | Filter by command using a substring match                 |
| `--user`    | `-u`  | Filter by username or UID; accepts comma-separated values |
| `--unwrap`  | `-l`  | Show the full command output                              |
| `--help`    |       | Show help                                                 |
| `--version` |       | Show the application version                              |

## Filtering

### Filter by port

Show only processes listening on TCP port `8080`:

```bash
uports --port 8080
```

or:

```bash
uports -p 8080
```

Matching ports are highlighted in the output.

### Filter by PID

Show listening sockets owned by a specific process:

```bash
uports --pid 1234
```

or:

```bash
uports -i 1234
```

### Filter by command

Search for a substring in the process command line:

```bash
uports --cmd nginx
```

This is a substring search, so it can match commands containing `nginx` anywhere in their command line.

For example:

```bash
uports -c postgres
```

### Filter by user

Filter by username:

```bash
uports --user postgres
```

Multiple users can be supplied as a comma-separated list:

```bash
uports --user root,postgres
```

You can also filter by UID:

```bash
uports --user 1000
```

Multiple UIDs are supported:

```bash
uports -u 0,1000,1001
```

Usernames and UIDs can be mixed:

```bash
uports -u root,1000,postgres
```

### Combine filters

Filters can be combined to narrow the results.

For example:

```bash
uports --port 8080 --user alice
```

Or:

```bash
uports --cmd nginx --port 443
```

When multiple filters are specified, a connection must satisfy the applicable filters to remain in the displayed results.

## Full Command Output

By default, long commands are constrained to keep the table readable.

Use `--unwrap` (`-l`) to display the full command without the normal command-column width limit:

```bash
uports --unwrap
```

This can be particularly useful when process command lines contain long argument lists.

## Output Format

Each user is displayed as a separate section:

```text
username (UID)
  PORT  PID    N   COMMAND
```

Where:

* **PORT** — Listening TCP port
* **PID** — Process ID owning the socket
* **N** — Number of matching sockets for the same PID and port
* **COMMAND** — Process command line

For example:

```text
www-data (33)
  PORT  PID    N   COMMAND
  80    1520   1   /usr/sbin/nginx -g daemon off;
  443   1520   1   /usr/sbin/nginx -g daemon off;
```

Only TCP connections whose state is `LISTEN` are included.

## Permissions

`uports` relies on operating-system process and network information exposed through `gopsutil`. Depending on the operating system and its security configuration, some processes or command lines may not be accessible to an unprivileged user.

If information is missing, try running it with elevated privileges:

```bash
sudo uports
```

Use elevated privileges only when necessary.
