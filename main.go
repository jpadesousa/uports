package main

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"os"
	"os/user"
	"slices"
	"sort"
	"strconv"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/prometheus/common/version"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Color style variables
var (
	usernameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("45")).
			Bold(true)

	uidStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true)

	columnTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))
)

// Message colors
var (
	titleStyle = lipgloss.NewStyle().
		Foreground(charmtone.Charple).
		Bold(true)
)

// Flag variables
type config struct {
	search struct {
		port uint32
		pid  int32
		cmd  string
		user []string
	}
}

var cfg config

func main() {

	// Create new flag sets
	searchFS := &pflag.FlagSet{}

	// CLI Create a new cobra Command
	cobraCmd := &cobra.Command{
		Use: "uports",
		Long: fmt.Sprintf(`%s

Displays listening TCP ports grouped by user,
along with the owning process ID and command.`,
			titleStyle.Render("DESCRIPTION")),
		RunE: runApp,
	}

	// =============================================
	// CLI Flags
	// =============================================

	// search
	searchFS.Uint32VarP(
		&cfg.search.port,
		"port",
		"p",
		0,
		"Filter by listening TCP port",
	)

	searchFS.Int32VarP(
		&cfg.search.pid,
		"pid",
		"i",
		0,
		"Filter by process ID",
	)

	searchFS.StringVarP(
		&cfg.search.cmd,
		"cmd",
		"c",
		"",
		"Filter by command (substring match)",
	)

	searchFS.StringSliceVarP(
		&cfg.search.user,
		"user",
		"u",
		nil,
		"Filter by username or UID (comma-separated values)",
	)

	cobraCmd.Flags().AddFlagSet(searchFS)

	// =============================================
	// Execute command
	// =============================================

	// Set new flag display settings
	cobraCmd.Flags().SetInterspersed(false)
	cobraCmd.Flags().SortFlags = false
	cobraCmd.Flags().PrintDefaults()

	// Remove completion and help commands (--help is kept)
	cobraCmd.CompletionOptions.DisableDefaultCmd = true
	cobraCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	// CLI Version
	cobraCmd.Version = version.Print("uports")

	// Execute cobra command
	if err := fang.Execute(context.Background(), cobraCmd); err != nil {
		os.Exit(1)
	}
}

func runApp(cobraCmd *cobra.Command, args []string) error {

	type userConnection struct {
		port uint32
		pid  int32
		cmd  string
	}

	type procInfo struct {
		username string
		cmd      string
	}

	userConn := make(map[string][]userConnection)

	// procCache serves to avoid having duplicated information for the same PID
	procCache := make(map[int32]procInfo)

	// Get all listening TCP sockets
	conns, err := net.Connections("tcp")
	if err != nil {
		panic(err)
	}

	// ------------------------------------------------ //
	// For each socket
	// ------------------------------------------------ //
	for _, c := range conns {

		// Only LISTEN ports
		if c.Status != "LISTEN" {
			continue
		}

		// Exclude PID 0
		if c.Pid == 0 {
			continue
		}

		_, ok := procCache[c.Pid]

		if !ok {

			// Gather PID
			p, err := process.NewProcess(c.Pid)
			if err != nil {
				continue
			}

			// Gather UIDs associated with PID
			uids, err := p.Uids()
			if err != nil || len(uids) == 0 {
				continue
			}

			// Skip root
			if uids[0] == 0 {
				continue
			}

			// Lookup user and convert UID to string
			u, err := user.LookupId(strconv.FormatUint(uint64(uids[0]), 10))
			if err != nil {
				continue
			}

			// Gather command line arguments
			cmd, err := p.Cmdline()
			if err != nil {
				continue
			}

			// Add to process cache
			procCache[c.Pid] = procInfo{
				username: u.Username,
				cmd:      cmd,
			}

			userConn[u.Username] =
				append(userConn[u.Username],
					userConnection{port: c.Laddr.Port, pid: c.Pid, cmd: cmd})
		}
	}

	// ------------------------------------------------ //
	// Print users alphabetically and with sorted ports
	// ------------------------------------------------ //

	var users []string
	if cfg.search.user != nil {
		users = cfg.search.user
		sort.Strings(users)
	} else {
		users = slices.Collect(maps.Keys(userConn))
		sort.Strings(users)
	}

	for _, username := range users {
		conns := userConn[username]

		// Sort this user's connections by port
		slices.SortFunc(conns, func(a, b userConnection) int {
			return cmp.Compare(a.port, b.port)
		})

		u, err := user.Lookup(username)
		if err != nil {
			continue
		}

		fmt.Println(
			usernameStyle.Render(
				fmt.Sprintf("\n%s", username)),
			uidStyle.Render(
				fmt.Sprintf("(%s)", u.Uid),
			))

		fmt.Println(columnTitleStyle.Render(
			fmt.Sprintf("  %-5s %-6s %s", "PORT", "PID", "COMMAND"),
		))
		for _, c := range conns {
			fmt.Printf("  %-5d %-6d %s\n",
				c.port,
				c.pid,
				truncateChars(c.cmd, 60),
			)
		}
	}

	return nil
}

// truncateChars truncates a string to a maximum number of characters.
func truncateChars(s string, maxChars int) string {
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[:maxChars-3]) + "..."
}
