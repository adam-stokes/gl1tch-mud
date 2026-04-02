package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/adam-stokes/gl1tch-mud/internal/busd"
	"github.com/adam-stokes/gl1tch-mud/internal/commands"
	"github.com/adam-stokes/gl1tch-mud/internal/db"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func main() {
	database, err := db.Open()
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

	interactive := term.IsTerminal(int(os.Stdin.Fd()))

	if interactive {
		fmt.Println(`
  ██████  ██      ██ ████████  ██████ ██   ██       ███    ███ ██    ██ ██████
 ██       ██      ██    ██    ██      ██   ██       ████  ████ ██    ██ ██   ██
 ██   ███ ██      ██    ██    ██      ███████ █████ ██ ████ ██ ██    ██ ██   ██
 ██    ██ ██      ██    ██    ██      ██   ██       ██  ██  ██ ██    ██ ██   ██
  ██████  ███████ ██    ██     ██████ ██   ██       ██      ██  ██████  ██████

  jack in. ghost the gibson. don't get traced.
  type 'help' for commands.`)

		// Show the starting room only in interactive mode.
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
	}
}
