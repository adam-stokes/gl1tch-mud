package commands

import (
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

const lanPort = 8080

// binary is the path to the gl1tch-mud executable, set by main before the
// game loop starts via SetBinary.
var binary string

// LANServer is the minimal interface that main.go's in-process server must satisfy.
// Using an interface avoids an import cycle (server imports commands).
type LANServer interface {
	Stop()
}

// lanServer is the optional in-process LAN server, set by main via SetLANServer.
var lanServer LANServer

// SetBinary sets the executable path used by /lan to fork the serve process.
func SetBinary(path string) {
	binary = path
}

// SetLANServer registers the in-process LAN server so that world hot-swap
// can stop/restart it when the player switches worlds.
func SetLANServer(srv LANServer) {
	lanServer = srv
}

func init() {
	Registry["lan"] = Lan
}

// Lan handles /lan [stop|status|<passphrase>].
func Lan(db *sql.DB, s *player.State, w *world.World, args []string) Result {
	sub := ""
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
	}

	switch sub {
	case "stop":
		return lanStop()
	case "status":
		return lanStatus()
	case "restart":
		passphrase := ""
		if len(args) > 1 {
			passphrase = args[1]
		}
		lanStop()
		return lanStart(passphrase)
	default:
		passphrase := ""
		if len(args) > 0 {
			passphrase = args[0]
		}
		return lanStart(passphrase)
	}
}

func pidFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "gl1tch-mud", "server.pid")
}

func readPID() (int, error) {
	data, err := os.ReadFile(pidFile())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func serverRunning() bool {
	pid, err := readPID()
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks if the process exists without killing it.
	return proc.Signal(nil) == nil
}

func lanStart(passphrase string) Result {
	if serverRunning() {
		pid, _ := readPID()
		url := fmt.Sprintf("http://%s:%d", lanIP(), lanPort)
		out := fmt.Sprintf("LAN session already active: %s", url)
		_ = pid
		return Result{Output: out}
	}

	bin := binary
	if bin == "" {
		var err error
		bin, err = exec.LookPath("gl1tch-mud")
		if err != nil {
			return Result{Output: "lan: cannot find gl1tch-mud binary"}
		}
	}

	args := []string{"--serve", fmt.Sprintf("--port=%d", lanPort)}
	if passphrase != "" {
		args = append(args, fmt.Sprintf("--passphrase=%s", passphrase))
	}

	cmd := exec.Command(bin, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	// Detach from the current process group so it survives parent exit.
	cmd.SysProcAttr = sysProcDetach()

	if err := cmd.Start(); err != nil {
		return Result{Output: fmt.Sprintf("lan: failed to start server: %v", err)}
	}

	// Write PID file.
	pidPath := pidFile()
	os.MkdirAll(filepath.Dir(pidPath), 0o755) //nolint:errcheck
	os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644) //nolint:errcheck

	// Release so we don't wait on it.
	cmd.Process.Release() //nolint:errcheck

	url := fmt.Sprintf("http://%s:%d", lanIP(), lanPort)
	out := fmt.Sprintf("LAN session started: %s\nShare this URL with your players.", url)
	if passphrase != "" {
		out += fmt.Sprintf("\nPassphrase: %s", passphrase)
	}
	return Result{Output: out}
}

func lanStop() Result {
	pid, err := readPID()
	if err != nil {
		return Result{Output: "no LAN session is active."}
	}
	proc, err := os.FindProcess(pid)
	if err != nil || proc.Signal(nil) != nil {
		os.Remove(pidFile()) //nolint:errcheck
		return Result{Output: "no LAN session is active."}
	}
	proc.Signal(os.Interrupt) //nolint:errcheck
	os.Remove(pidFile())      //nolint:errcheck
	return Result{Output: "LAN session stopped."}
}

func lanStatus() Result {
	if !serverRunning() {
		return Result{Output: "no LAN session is active."}
	}
	url := fmt.Sprintf("http://%s:%d", lanIP(), lanPort)
	return Result{Output: fmt.Sprintf("LAN session: %s", url)}
}

func lanIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		if ip4 := ip.To4(); ip4 != nil {
			s := ip4.String()
			if !strings.HasPrefix(s, "169.254") {
				return s
			}
		}
	}
	return "localhost"
}
