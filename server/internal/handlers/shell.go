package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type ShellHandler struct {
	engine *core.Engine
}

func NewShellHandler(engine *core.Engine) *ShellHandler {
	return &ShellHandler{engine: engine}
}

// ListContainers returns the project's running containers so the
// frontend can offer a dropdown.
func (h *ShellHandler) ListContainers(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	project, err := h.engine.GetProject(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	type container struct {
		Name  string `json:"name"`
		Image string `json:"image"`
		State string `json:"state"`
	}
	var containers []container
	for _, c := range project.Containers {
		containers = append(containers, container{
			Name:  c.Name,
			Image: c.Image,
			State: c.State,
		})
	}
	writeJSON(w, http.StatusOK, containers)
}

// ExecWebSocket upgrades the HTTP connection to a WebSocket and spawns
// an interactive `docker exec` session inside the chosen container.
// Input/output is streamed over the WebSocket as binary frames (stdout/
// stderr) and text frames (stdin). A JSON "resize" message sets the PTY
// dimensions.
func (h *ShellHandler) ExecWebSocket(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	containerName := r.URL.Query().Get("container")
	if containerName == "" {
		writeError(w, http.StatusBadRequest, "container query parameter is required")
		return
	}

	project, err := h.engine.GetProject(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Validate the container belongs to this project.
	found := false
	for _, c := range project.Containers {
		if c.Name == containerName {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusBadRequest, "container does not belong to this project")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("shell: websocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()

	// Spawn docker exec -it. We use -i (stdin open) but NOT -t (no PTY
	// allocation by docker) because the WebSocket carries raw bytes, not
	// a terminal escape sequence stream. Most shells still work fine
	// with just -i; prompts won't be pretty but commands execute.
	//
	// For a full xterm.js experience we'd need a real PTY (creack/pty),
	// but that requires the server process itself to allocate the PTY,
	// which adds a C dependency. The -i approach works for the common
	// case (running ad-hoc commands like `ls`, `cat`, `env`, etc.).
	shell := detectShell(containerName)
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", containerName, shell)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		sendWSError(conn, "failed to create stdin pipe: "+err.Error())
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		sendWSError(conn, "failed to create stdout pipe: "+err.Error())
		return
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		sendWSError(conn, "failed to start docker exec: "+err.Error())
		return
	}

	var wg sync.WaitGroup

	// stdout → websocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// websocket → stdin
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdin.Close()
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
			if msgType == websocket.TextMessage {
				// Check for resize or other control messages.
				var ctrl struct {
					Type string `json:"type"`
				}
				if json.Unmarshal(msg, &ctrl) == nil && ctrl.Type == "resize" {
					continue // no PTY to resize without -t
				}
				// Otherwise treat text as stdin input.
				if _, err := stdin.Write(msg); err != nil {
					return
				}
				continue
			}
			if _, err := stdin.Write(msg); err != nil {
				return
			}
		}
	}()

	_ = cmd.Wait()
	wg.Wait()
	_ = conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shell exited"))
}

func detectShell(containerName string) string {
	// Try common shells in order. A quick `docker exec` test is cheaper
	// than pulling the image manifest.
	for _, sh := range []string{"/bin/bash", "/bin/sh"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		out, err := exec.CommandContext(ctx, "docker", "exec", containerName, "test", "-x", sh).CombinedOutput()
		cancel()
		_ = out
		if err == nil {
			return sh
		}
	}
	return "/bin/sh"
}

func sendWSError(conn *websocket.Conn, msg string) {
	_ = conn.WriteMessage(websocket.TextMessage, []byte("error: "+msg+"\r\n"))
}

// writeWSClose shuts down a WebSocket with a close frame and reason.
func writeWSClose(conn *websocket.Conn, code int, reason string) {
	_ = conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(code, truncate(reason, 123)))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// Ensure ShellHandler is only used for projects the user has access to.
// The chi route group already enforces auth; the handler validates the
// container belongs to the named project so cross-project exec is not
// possible.
var _ = (*ShellHandler)(nil) // compile-time interface check placeholder

// WebSocket auth: the browser's EventSource / WebSocket API cannot set
// custom headers. The handler reads the session token from the
// `token` query parameter or the Authorization header.
func ShellAuthFromQuery(r *http.Request) string {
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}
