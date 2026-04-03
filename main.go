package main

import (
	"bufio"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/adam-stokes/gl1tch-mud/internal/busd"
	"github.com/adam-stokes/gl1tch-mud/internal/commands"
	"github.com/adam-stokes/gl1tch-mud/internal/db"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/server"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

//go:embed all:web/dist
var webDist embed.FS

func main() {
	// --serve mode: run only the HTTP/WebSocket server (used by /lan).
	serveMode := flag.Bool("serve", false, "run LAN server only")
	servePort := flag.Int("port", 8080, "server port")
	servePass := flag.String("passphrase", "", "session passphrase")
	flag.Parse()

	if *serveMode {
		runServe(*servePort, *servePass)
		return
	}

	runGame()
}

// runServe starts the HTTP/WebSocket server and blocks until SIGINT/SIGTERM.
func runServe(port int, passphrase string) {
	sub, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: embed:", err)
		os.Exit(1)
	}
	server.SetFS(sub)

	w, err := world.Load("cyberspace")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: world:", err)
		os.Exit(1)
	}

	srv := server.New(w)
	if _, err := srv.Start(port, passphrase); err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: serve:", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	srv.Stop()
}

// runGame runs the interactive game loop.
func runGame() {
	database, err := db.OpenForWorld("cyberspace")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: db:", err)
		os.Exit(1)
	}
	defer database.Close()

	w, err := world.Load("cyberspace")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: world:", err)
		os.Exit(1)
	}

	s, err := player.Load(database)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl1tch-mud: player:", err)
		os.Exit(1)
	}

	bus := busd.Connect()
	defer bus.Close()

	bus.Publish("mud.session.started", map[string]any{
		"player":  s.Name,
		"room_id": s.RoomID,
		"world":   s.World,
	})

	// Wire /lan command — it forks gl1tch-mud --serve as a detached process.
	commands.SetBinary(executablePath())

	lanSrv := server.New(w)
	commands.SetLANServer(lanSrv)

	interactive := term.IsTerminal(int(os.Stdin.Fd()))

	if interactive {
		fmt.Println(`
  ██████  ██      ██ ████████  ██████ ██   ██       ███    ███ ██    ██ ██████
 ██       ██      ██    ██    ██      ██   ██       ████  ████ ██    ██ ██   ██
 ██   ███ ██      ██    ██    ██      ███████ █████ ██ ████ ██ ██    ██ ██   ██
 ██    ██ ██      ██    ██    ██      ██   ██       ██  ██  ██ ██    ██ ██   ██
  ██████  ███████ ██    ██     ██████ ██   ██       ██      ██  ██████  ██████

  jack in. ghost the gibson. don't get traced.
  type 'help' for commands. type '/lan' to start a multiplayer session.`)

		res := commands.Look(database, s, w, nil)
		fmt.Println(res.Output)
		if res.Event != nil {
			bus.Publish(res.Event.Topic, res.Event.Payload)
		}
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		if interactive {
			fmt.Print("> ")
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
				fmt.Println("disconnecting from The Gibson.")
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
					lanSrv = server.New(w)
					commands.SetLANServer(lanSrv)
					newState, _ := player.Load(database)
					*s = *newState
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
