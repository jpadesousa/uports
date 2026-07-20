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
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/prometheus/common/version"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Color and column style variables
var (
	usernameStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Bold(true)
	uidStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	columnTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	helpTitleStyle   = lipgloss.NewStyle().Foreground(charmtone.Charple).Bold(true)
	portStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	pidStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	cmdStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)

	portCol     = lipgloss.NewStyle().Width(5)
	pidCol      = lipgloss.NewStyle().Width(6)
	nSocketsCol = lipgloss.NewStyle().Width(3)
	cmdCol      = lipgloss.NewStyle().MaxWidth(61)
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
			helpTitleStyle.Render("DESCRIPTION")),
		RunE: runApp,
	}

	// =============================================
	// CLI Flags
	// =============================================

	// search
	searchFS.Uint32VarP(&cfg.search.port, "port", "p", 0,
		"Filter by listening TCP port",
	)

	searchFS.Int32VarP(&cfg.search.pid, "pid", "i", 0,
		"Filter by process ID",
	)

	searchFS.StringVarP(&cfg.search.cmd, "cmd", "c", "",
		"Filter by command (substring match)",
	)

	searchFS.StringSliceVarP(&cfg.search.user, "user", "u", nil,
		"Filter by username or UID (comma-separated values)",
	)

	cobraCmd.Flags().AddFlagSet(searchFS)

	// =============================================
	// Execute app
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
		port      uint32
		pid       int32
		cmd       string
		nSockets  uint32
		colorPort bool
		colorPid  bool
		colorCmd  bool
	}
	userConn := make(map[string][]userConnection)

	// pidCache serves to avoid having duplicated information for the same PID
	type procInfo struct {
		username string
		cmd      string
	}
	procCache := map[int32]procInfo{}

	// Get all network connections
	// Chosen ConnectionsPid instead of Connections to have a small performance
	// benefit when the user searches by PID
	TCPconns, err := net.ConnectionsPid("tcp", cfg.search.pid)
	if err != nil {
		panic(err)
	}

	// ------------------------------------------------ //
	// Gather info for each sockets
	// ------------------------------------------------ //
	for _, c := range TCPconns {

		// Define the TCP status to display
		if c.Status != "LISTEN" {
			continue
		}

		info, ok := procCache[c.Pid]

		if !ok {

			// Gather PID
			p, err := process.NewProcess(c.Pid)
			if err != nil {
				continue
			}

			// Lookup user and convert UID to string
			u, err := user.LookupId(strconv.FormatUint(uint64(c.Uids[0]), 10))
			if err != nil {
				continue
			}

			// Gather command line arguments
			cmd, err := p.Cmdline()
			if err != nil {
				continue
			}

			// Add to process cache
			info = procInfo{
				username: u.Username,
				cmd:      cmd,
			}
			procCache[c.Pid] = info
		}

		// Check if we've already added this PID and PORT for this user.
		found := false
		for i := range userConn[info.username] {
			if userConn[info.username][i].pid == c.Pid &&
				userConn[info.username][i].port == c.Laddr.Port {

				userConn[info.username][i].nSockets++
				found = true
				break

			}
		}

		if !found {
			userConn[info.username] =
				append(userConn[info.username], userConnection{
					port:     c.Laddr.Port,
					pid:      c.Pid,
					cmd:      info.cmd,
					nSockets: 1,
				})
		}
	}

	// Collect all usernames
	allUsers := slices.Collect(maps.Keys(userConn))

	// Create an empty slice to fill filtered usernames
	var filteredUsers []string

	// The filters below will modify the contents of userConn
	// (map[string][]userConnection) so that if it will only keep the
	// userConnection structs that pass the filter

	// ------------------------------------------------ //
	// Handle ports
	// ------------------------------------------------ //

	if cobraCmd.Flags().Changed("port") {

		for _, username := range allUsers {
			conns := userConn[username]

			filtered := conns[:0]

			for i := range conns {
				if conns[i].port == cfg.search.port {
					conns[i].colorPort = true
					filtered = append(filtered, conns[i])
				}
			}

			userConn[username] = filtered

			if len(filtered) > 0 {
				filteredUsers = append(filteredUsers, username)
			}
		}
	}

	// ------------------------------------------------ //
	// Handle pids
	// ------------------------------------------------ //
	if cobraCmd.Flags().Changed("pid") {

		if filteredUsers == nil {
			filteredUsers = allUsers
		}

		for _, username := range filteredUsers {
			conns := userConn[username]

			filtered := conns[:0]

			for i := range conns {
				if conns[i].pid == cfg.search.pid {
					conns[i].colorPid = true
					filtered = append(filtered, conns[i])
				}
			}

			userConn[username] = filtered

			if len(filtered) > 0 {
				filteredUsers = append(filteredUsers, username)
			}
		}
	}

	// ------------------------------------------------ //
	// Handle cmd
	// ------------------------------------------------ //
	if cfg.search.cmd != "" {

		if filteredUsers == nil {
			filteredUsers = allUsers
		}

		for _, username := range filteredUsers {
			conns := userConn[username]

			filtered := conns[:0]

			for i := range conns {
				if strings.Contains(conns[i].cmd, cfg.search.cmd) {
					conns[i].colorCmd = true
					filtered = append(filtered, conns[i])
				}
			}

			userConn[username] = filtered

			if len(filtered) > 0 {
				filteredUsers = append(filteredUsers, username)
			}
		}
	}

	// ------------------------------------------------ //
	// Handle users
	// ------------------------------------------------ //

	// Process user flag or collect all users
	if cfg.search.user != nil {

		// users in the --users flag takes priority
		filteredUsers = nil

		for _, s := range cfg.search.user {
			username, err := lookupUsername(s)
			if err != nil {
				continue
			}
			filteredUsers = append(filteredUsers, username)
		}
	} else {
		// If no users where filtered just include all users
		if filteredUsers == nil {
			filteredUsers = allUsers
		}
		sort.Strings(filteredUsers)
	}

	// Keep only unique usernames
	var UniqueUsers []string
	seen := make(map[string]struct{})
	for _, user := range filteredUsers {
		if _, ok := seen[user]; !ok {
			seen[user] = struct{}{}
			UniqueUsers = append(UniqueUsers, user)
		}
	}

	userCount := 0
	for _, username := range UniqueUsers {

		conns := userConn[username]
		// If there are no entries in the user's connection because they were
		// filtered out, move to the next user
		if len(conns) == 0 {
			continue
		}

		// Sort this user's connections by port
		slices.SortFunc(conns, func(a, b userConnection) int {
			return cmp.Compare(a.port, b.port)
		})

		u, err := user.Lookup(username)
		if err != nil {
			continue
		}

		// Leave a gap between user tables
		if userCount > 0 {
			fmt.Println()
		}

		// Render username and uid
		fmt.Println(
			usernameStyle.Render(
				fmt.Sprintf("%s", username)),
			uidStyle.Render(
				fmt.Sprintf("(%s)", u.Uid),
			))

		// Print table values for each user
		fmt.Println(columnTitleStyle.Render(
			fmt.Sprintf("  %-5s %-6s %-3s %s", "PORT", "PID", "N", "COMMAND"),
		))

		for _, c := range conns {

			port := strconv.FormatUint(uint64(c.port), 10)
			pid := strconv.FormatUint(uint64(c.pid), 10)
			nSockets := strconv.FormatUint(uint64(c.nSockets), 10)

			if c.colorPort {
				port = portStyle.Render(port)
			}

			if c.colorPid {
				pid = pidStyle.Render(pid)
			}

			if c.colorCmd {
				c.cmd = cmdStyle.Render(c.cmd)
			} else {
				c.cmd = cmdCol.Render(c.cmd)
			}

			fmt.Println(
				"  " +
					portCol.Render(port) + " " +
					pidCol.Render(pid) + " " +
					nSocketsCol.Render(nSockets) + " " +
					c.cmd,
			)
		}

		userCount++
	}

	return nil
}

// lookupUsername converts a string into a integer and search the user based on
// UID. If that fails, it will search the user based on username.
func lookupUsername(s string) (string, error) {
	var u *user.User
	var err error

	if _, err = strconv.Atoi(s); err == nil {
		// It's a UID
		u, err = user.LookupId(s)
	} else {
		// It's a username
		u, err = user.Lookup(s)
	}
	if err != nil {
		return "", err
	}

	return u.Username, nil
}
