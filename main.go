// spank-claude: approve Claude Code actions by slapping your laptop.
// Reads the Apple Silicon accelerometer via IOKit HID and runs an HTTP
// server that integrates with Claude Code hooks (PermissionRequest).
// Needs sudo.
package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/charmbracelet/fang"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/spf13/cobra"
	"github.com/taigrr/apple-silicon-accelerometer/detector"
	"github.com/taigrr/apple-silicon-accelerometer/sensor"
	"github.com/taigrr/apple-silicon-accelerometer/shm"
)

var version = "dev"

//go:embed audio/pain/*.mp3
var painAudio embed.FS

//go:embed audio/sexy/*.mp3
var sexyAudio embed.FS

//go:embed audio/halo/*.mp3
var haloAudio embed.FS

var (
	port      int
	soundPkgF string // "pain", "sexy", or "halo"
)

// sensorReady is closed once shared memory is created and the sensor
// worker is about to enter the CFRunLoop.
var sensorReady = make(chan struct{})

// sensorErr receives any error from the sensor worker.
var sensorErr = make(chan error, 1)

// ApprovalRequest represents a pending Claude Code permission request.
type ApprovalRequest struct {
	ToolName string
	Response chan string // "allow" or "" (timeout)
}

// pendingApproval is a buffered channel for at most one pending request.
var pendingApproval = make(chan ApprovalRequest, 1)

// awaitingSlap gates shock processing: only handle slaps after a request arrives.
var awaitingSlap atomic.Bool

type soundPack struct {
	name  string
	fs    embed.FS
	dir   string
	files []string
}

func (sp *soundPack) loadFiles() error {
	entries, err := sp.fs.ReadDir(sp.dir)
	if err != nil {
		return err
	}
	sp.files = make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			sp.files = append(sp.files, sp.dir+"/"+e.Name())
		}
	}
	sort.Strings(sp.files)
	return nil
}

func (sp *soundPack) randomFile() string {
	if len(sp.files) == 0 {
		return ""
	}
	return sp.files[rand.Intn(len(sp.files))]
}

func main() {
	cmd := &cobra.Command{
		Use:   "spank",
		Short: "Approve Claude Code actions by slapping your laptop",
		Long: `spank-claude reads the Apple Silicon accelerometer via IOKit HID
and runs an HTTP server for Claude Code PermissionRequest hooks.

When Claude Code requests permission, spank plays a notification sound
and waits for a physical slap on the laptop. A detected slap approves
the action and plays a sound from the selected pack.

Requires sudo (for IOKit HID access to the accelerometer).`,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context())
		},
		SilenceUsage: true,
	}

	cmd.Flags().IntVar(&port, "port", 19222, "HTTP server port")
	cmd.Flags().StringVar(&soundPkgF, "sound", "pain", "Sound pack for approval: pain, sexy, or halo")

	if err := fang.Execute(context.Background(), cmd); err != nil {
		os.Exit(1)
	}
}

// hookRequest is the JSON body Claude Code sends to the hook endpoint.
type hookRequest struct {
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
}

// hookResponse is the JSON body we return to approve an action.
type hookResponse struct {
	HookSpecificOutput hookOutput `json:"hookSpecificOutput"`
}

type hookOutput struct {
	HookEventName string   `json:"hookEventName"`
	Decision      decision `json:"decision"`
}

type decision struct {
	Behavior string `json:"behavior"`
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req hookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Bad request — return empty 200 so Claude falls back to terminal prompt.
		w.WriteHeader(http.StatusOK)
		return
	}

	if req.HookEventName != "PermissionRequest" {
		// Not a permission request — return empty 200.
		w.WriteHeader(http.StatusOK)
		return
	}

	fmt.Printf("hook: permission request for tool=%s, waiting for slap...\n", req.ToolName)

	// Play macOS system notification sound.
	go exec.Command("afplay", "/System/Library/Sounds/Ping.aiff").Run()

	// Create approval request.
	respCh := make(chan string, 1)
	approval := ApprovalRequest{
		ToolName: req.ToolName,
		Response: respCh,
	}

	// Try to enqueue — if channel is full (another request pending), fallback.
	select {
	case pendingApproval <- approval:
		// Queued successfully — start listening for slaps.
		awaitingSlap.Store(true)
	default:
		// Already a pending request — return empty 200 (fallback to terminal).
		fmt.Println("hook: another request already pending, falling back to terminal")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Wait for slap or timeout.
	select {
	case result := <-respCh:
		if result == "allow" {
			resp := hookResponse{
				HookSpecificOutput: hookOutput{
					HookEventName: "PermissionRequest",
					Decision:      decision{Behavior: "allow"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			fmt.Printf("hook: approved tool=%s via slap!\n", req.ToolName)
			return
		}
	case <-time.After(30 * time.Second):
		awaitingSlap.Store(false)
		// Drain the pending request so the main loop doesn't hold a stale one.
		select {
		case <-pendingApproval:
		default:
		}
		fmt.Printf("hook: timeout waiting for slap (tool=%s), falling back to terminal\n", req.ToolName)
	}

	// Timeout or empty result — return empty 200.
	w.WriteHeader(http.StatusOK)
}

func run(ctx context.Context) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("spank requires root privileges for accelerometer access, run with: sudo spank")
	}

	var pack *soundPack
	switch soundPkgF {
	case "sexy":
		pack = &soundPack{name: "sexy", fs: sexyAudio, dir: "audio/sexy"}
	case "halo":
		pack = &soundPack{name: "halo", fs: haloAudio, dir: "audio/halo"}
	case "pain":
		pack = &soundPack{name: "pain", fs: painAudio, dir: "audio/pain"}
	default:
		return fmt.Errorf("unknown sound pack %q, choose: pain, sexy, or halo", soundPkgF)
	}

	if err := pack.loadFiles(); err != nil {
		return fmt.Errorf("loading %s audio: %w", pack.name, err)
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create shared memory for accelerometer data.
	accelRing, err := shm.CreateRing(shm.NameAccel)
	if err != nil {
		return fmt.Errorf("creating accel shm: %w", err)
	}
	defer accelRing.Close()
	defer accelRing.Unlink()

	// Start the sensor worker in a background goroutine.
	go func() {
		close(sensorReady)
		err := sensor.Run(sensor.Config{
			AccelRing: accelRing,
			Restarts:  0,
		})
		if err != nil {
			sensorErr <- err
		}
	}()

	// Wait for sensor to be ready.
	select {
	case <-sensorReady:
	case err := <-sensorErr:
		return fmt.Errorf("sensor worker failed: %w", err)
	case <-ctx.Done():
		return nil
	}

	// Give the sensor a moment to start producing data.
	time.Sleep(100 * time.Millisecond)

	// Start HTTP server.
	mux := http.NewServeMux()
	mux.HandleFunc("/hook", hookHandler)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	server := &http.Server{Addr: addr, Handler: mux}

	go func() {
		fmt.Printf("spank: HTTP server listening on %s (sound=%s)\n", addr, pack.name)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
		}
	}()

	// Shut down HTTP server on context cancel.
	go func() {
		<-ctx.Done()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		server.Shutdown(shutCtx)
	}()

	det := detector.New()
	var lastAccelTotal uint64
	var lastEventTime time.Time
	lastYell := time.Time{}
	cooldown := 500 * time.Millisecond
	maxBatch := 200

	fmt.Printf("spank: listening for slaps (sound=%s, port=%d)... ctrl+c to quit\n", pack.name, port)

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nbye!")
			return nil
		case err := <-sensorErr:
			return fmt.Errorf("sensor worker failed: %w", err)
		case <-ticker.C:
		}

		now := time.Now()
		tNow := float64(now.UnixNano()) / 1e9

		samples, newTotal := accelRing.ReadNew(lastAccelTotal, shm.AccelScale)
		lastAccelTotal = newTotal
		if len(samples) > maxBatch {
			samples = samples[len(samples)-maxBatch:]
		}

		nSamples := len(samples)
		for idx, s := range samples {
			tSample := tNow - float64(nSamples-idx-1)/float64(det.FS)
			det.Process(s.X, s.Y, s.Z, tSample)
		}

		newEventIdx := len(det.Events)
		if newEventIdx > 0 {
			ev := det.Events[newEventIdx-1]
			if ev.Time != lastEventTime {
				if time.Since(lastYell) > cooldown {
					if ev.Severity == "CHOC_MAJEUR" || ev.Severity == "CHOC_MOYEN" {
						// Only process slaps when a permission request is pending.
						if !awaitingSlap.Load() {
							lastEventTime = ev.Time
							continue
						}
						lastEventTime = ev.Time
						lastYell = now

						// Check if there's a pending approval request.
						select {
						case approval := <-pendingApproval:
							awaitingSlap.Store(false)
							file := pack.randomFile()
							fmt.Printf("slap detected [%s amp=%.5fg] -> approved! playing %s\n", ev.Severity, ev.Amplitude, file)
							go playEmbedded(pack.fs, file)
							approval.Response <- "allow"
						default:
						}
					}
				}
			}
		}
	}
}

var speakerMu sync.Mutex
var speakerInit bool

func playEmbedded(fs embed.FS, path string) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return
	}

	streamer, format, err := mp3.Decode(io.NopCloser(bytes.NewReader(data)))
	if err != nil {
		return
	}
	defer streamer.Close()

	speakerMu.Lock()
	if !speakerInit {
		speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
		speakerInit = true
	}
	speakerMu.Unlock()

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))
	<-done
}
