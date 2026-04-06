package main

import (
	"bufio"
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"

	"golang.org/x/term"

	"github.com/adam-stokes/gl1tch-mud/internal/auth"
	"github.com/adam-stokes/gl1tch-mud/internal/busd"
	"github.com/adam-stokes/gl1tch-mud/internal/commands"
	"github.com/adam-stokes/gl1tch-mud/internal/db"
	"github.com/adam-stokes/gl1tch-mud/internal/db/pgq"
	"github.com/adam-stokes/gl1tch-mud/internal/pgdb"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/server"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

//go:embed all:web/dist
var webDist embed.FS

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "useradd":
			runUserAdd(os.Args[2:])
			return
		case "userdel":
			runUserDel(os.Args[2:])
			return
		case "usermod":
			runUserMod(os.Args[2:])
			return
		case "userlist":
			runUserList()
			return
		}
	}

	serveMode := flag.Bool("serve", false, "run LAN server only")
	servePort := flag.Int("port", 8080, "server port")
	servePass := flag.String("passphrase", "", "session passphrase")
	worldFlag := flag.String("world", "", "world to load (skips selection screen)")
	flag.Parse()

	if *serveMode {
		runServe(*servePort, *servePass, *worldFlag)
		return
	}

	worldName := *worldFlag
	if worldName == "" {
		if term.IsTerminal(int(os.Stdin.Fd())) {
			worldName = selectWorld()
		} else {
			worldName = "cyberspace"
		}
	}

	runGame(worldName)
}

// selectWorld prints an interactive numbered world menu and returns the chosen world name.
func selectWorld() string {
	metas := world.ListAvailable()
	if len(metas) == 0 {
		return "cyberspace"
	}
	if len(metas) == 1 {
		return metas[0].Name
	}

	fmt.Print("\n  available worlds:\n\n")
	for i, m := range metas {
		tagline := m.Tagline
		if tagline == "" {
			tagline = "—"
		}
		fmt.Printf("  [%d] %-16s — %s\n", i+1, m.Name, tagline)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n  > ")
		if !scanner.Scan() {
			return metas[0].Name
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if n, err := strconv.Atoi(input); err == nil {
			if n >= 1 && n <= len(metas) {
				return metas[n-1].Name
			}
		}
		for _, m := range metas {
			if strings.EqualFold(input, m.Name) {
				return m.Name
			}
		}
		fmt.Printf("  invalid selection %q — enter a number (1-%d) or world name\n", input, len(metas))
	}
}

// loadAllWorlds loads every available world. When lockedWorld is non-empty, only that world is loaded.
func loadAllWorlds(lockedWorld string) (map[string]*world.World, error) {
	if lockedWorld != "" {
		w, err := world.Load(lockedWorld)
		if err != nil {
			return nil, err
		}
		return map[string]*world.World{lockedWorld: w}, nil
	}
	names := world.Available()
	worlds := make(map[string]*world.World, len(names))
	for _, name := range names {
		w, err := world.Load(name)
		if err != nil {
			return nil, fmt.Errorf("load world %q: %w", name, err)
		}
		worlds[name] = w
	}
	return worlds, nil
}

// runServe starts the HTTP/WebSocket server and blocks until SIGINT/SIGTERM.
func runServe(port int, passphrase, lockedWorld string) {
	sub, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: embed:", err)
		os.Exit(1)
	}
	server.SetFS(sub)

	worlds, err := loadAllWorlds(lockedWorld)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: world:", err)
		os.Exit(1)
	}

	srv := server.New(worlds, lockedWorld)
	if _, err := srv.Start(port, passphrase); err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: serve:", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	srv.Stop()
}

// runGame runs the interactive game loop for the named world.
func runGame(worldName string) {
	database, err := db.OpenForWorld(worldName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: db:", err)
		os.Exit(1)
	}
	defer database.Close()

	w, err := world.Load(worldName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: world:", err)
		os.Exit(1)
	}

	s, err := player.Load(database)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: player:", err)
		os.Exit(1)
	}

	world.SeedCrystalShards(database, s.World)  //nolint:errcheck
	world.SeedStartingItems(database, s.World)  //nolint:errcheck

	bus := busd.Connect()
	defer bus.Close()

	bus.Publish("mud.session.started", map[string]any{
		"player":  s.Name,
		"room_id": s.RoomID,
		"world":   s.World,
	})

	commands.SetBinary(executablePath())

	lanSrv := server.New(map[string]*world.World{w.Name: w}, w.Name)
	commands.SetLANServer(lanSrv)

	interactive := term.IsTerminal(int(os.Stdin.Fd()))

	if interactive {
		if banner := w.UIBanner(); banner != "" {
			fmt.Println(banner)
		}
		if tagline := w.UI.Tagline; tagline != "" {
			fmt.Printf("  %s\n", tagline)
		}
		fmt.Println("  type 'help' for commands. type '/lan' to start a multiplayer session.")

		res := commands.Look(database, s, w, nil)
		fmt.Println(res.Output)
		if res.Event != nil {
			bus.Publish(res.Event.Topic, res.Event.Payload)
		}
	}

	prompt := w.UIPrompt() + " "
	scanner := bufio.NewScanner(os.Stdin)
	for {
		if interactive {
			fmt.Print(prompt)
		}
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "quit" || line == "exit" || line == "q" {
			bus.Publish("mud.session.ended", map[string]any{
				"player":  s.Name,
				"room_id": s.RoomID,
			})
			if interactive {
				fmt.Println("disconnecting.")
			}
			break
		}

		line = strings.TrimPrefix(line, "/")

		verb, args := commands.Parse(line)
		handler, ok := commands.Registry[verb]
		if !ok {
			fmt.Printf("unknown command: %q — type 'help' for a list.\n", verb)
			continue
		}

		result := handler(database, s, w, args)
		fmt.Println(result.Output)
		if result.Event != nil {
			bus.Publish(result.Event.Topic, result.Event.Payload)
		}
		if result.SwitchWorld != "" {
			newDB, swErr := db.OpenForWorld(result.SwitchWorld)
			if swErr != nil {
				fmt.Fprintf(os.Stderr, "world switch: %v\n", swErr)
			} else {
				database.Close()
				database = newDB
				newWorld, swErr := world.Load(result.SwitchWorld)
				if swErr != nil {
					fmt.Fprintf(os.Stderr, "world switch: %v\n", swErr)
				} else {
					w = newWorld
					lanSrv.Stop()
					lanSrv = server.New(map[string]*world.World{w.Name: w}, w.Name)
					commands.SetLANServer(lanSrv)
					prompt = w.UIPrompt() + " "
					newState, _ := player.LoadForWorld(database, result.SwitchWorld, w.StartRoom)
					*s = *newState
					if w.Room(s.RoomID) == nil {
						s.RoomID = w.StartRoom
						s.World = result.SwitchWorld
						player.Save(database, s) //nolint:errcheck
					}
					world.SeedCrystalShards(database, s.World)  //nolint:errcheck
					world.SeedStartingItems(database, s.World)  //nolint:errcheck
					bus.Publish("mud.world.switch", map[string]any{"world": w.Name})
					if banner := w.UIBanner(); banner != "" {
						fmt.Println(banner)
					}
					if tagline := w.UI.Tagline; tagline != "" {
						fmt.Printf("  %s\n", tagline)
					}
					lookResult := commands.Look(database, s, w, nil)
					fmt.Println(lookResult.Output)
				}
			}
		}
	}
}

func executablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "gl1tch-mud"
	}
	return exe
}

// --- CLI account management subcommands ---

func runUserAdd(args []string) {
	fs := flag.NewFlagSet("useradd", flag.ExitOnError)
	username := fs.String("username", "", "account username")
	password := fs.String("password", "", "account password")
	role := fs.String("role", "player", "account role (admin|player)")
	fs.Parse(args) //nolint:errcheck

	if err := auth.ValidateNewAccount(*username, *password, *role); err != nil {
		fmt.Fprintln(os.Stderr, "useradd:", err)
		os.Exit(1)
	}

	hash, err := auth.HashPassword(*password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "useradd:", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgdb.Connect(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "useradd:", err)
		os.Exit(1)
	}
	defer pool.Close()

	row, err := pgq.New(pool).CreateAccount(ctx, pgq.CreateAccountParams{
		Username:     *username,
		PasswordHash: hash,
		Role:         *role,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "useradd:", err)
		os.Exit(1)
	}

	uid := formatUUID(row.ID.Bytes)
	fmt.Printf("created account %q (id: %s, role: %s)\n", row.Username, uid, row.Role)
}

func runUserDel(args []string) {
	fs := flag.NewFlagSet("userdel", flag.ExitOnError)
	username := fs.String("username", "", "account username")
	fs.Parse(args) //nolint:errcheck

	if *username == "" {
		fmt.Fprintln(os.Stderr, "userdel: --username required")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgdb.Connect(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "userdel:", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pgq.New(pool).DeleteAccount(ctx, *username); err != nil {
		fmt.Fprintln(os.Stderr, "userdel:", err)
		os.Exit(1)
	}

	fmt.Printf("deleted account %q\n", *username)
}

func runUserMod(args []string) {
	fs := flag.NewFlagSet("usermod", flag.ExitOnError)
	username := fs.String("username", "", "account username")
	password := fs.String("password", "", "new password")
	ban := fs.Bool("ban", false, "ban the account")
	unban := fs.Bool("unban", false, "unban the account")
	fs.Parse(args) //nolint:errcheck

	if *username == "" {
		fmt.Fprintln(os.Stderr, "usermod: --username required")
		os.Exit(1)
	}
	if *password == "" && !*ban && !*unban {
		fmt.Fprintln(os.Stderr, "usermod: specify --password, --ban, or --unban")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgdb.Connect(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "usermod:", err)
		os.Exit(1)
	}
	defer pool.Close()

	q := pgq.New(pool)

	if *password != "" {
		hash, err := auth.HashPassword(*password)
		if err != nil {
			fmt.Fprintln(os.Stderr, "usermod:", err)
			os.Exit(1)
		}
		if err := q.UpdatePassword(ctx, pgq.UpdatePasswordParams{
			PasswordHash: hash,
			Username:     *username,
		}); err != nil {
			fmt.Fprintln(os.Stderr, "usermod:", err)
			os.Exit(1)
		}
		fmt.Printf("updated password for %q\n", *username)
	}

	if *ban {
		if err := q.SetBanned(ctx, pgq.SetBannedParams{
			Banned:   true,
			Username: *username,
		}); err != nil {
			fmt.Fprintln(os.Stderr, "usermod:", err)
			os.Exit(1)
		}
		fmt.Printf("banned %q\n", *username)
	}

	if *unban {
		if err := q.SetBanned(ctx, pgq.SetBannedParams{
			Banned:   false,
			Username: *username,
		}); err != nil {
			fmt.Fprintln(os.Stderr, "usermod:", err)
			os.Exit(1)
		}
		fmt.Printf("unbanned %q\n", *username)
	}
}

func runUserList() {
	ctx := context.Background()
	pool, err := pgdb.Connect(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "userlist:", err)
		os.Exit(1)
	}
	defer pool.Close()

	accounts, err := pgq.New(pool).ListAccounts(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "userlist:", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "USERNAME\tROLE\tBANNED\tCREATED")
	for _, a := range accounts {
		created := "—"
		if a.CreatedAt.Valid {
			created = a.CreatedAt.Time.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(w, "%s\t%s\t%t\t%s\n", a.Username, a.Role, a.Banned, created)
	}
	w.Flush()
}

// formatUUID converts a [16]byte UUID to its string representation.
func formatUUID(b [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

