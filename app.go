package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"regexp"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:build/web
var webAssets embed.FS

// App struct
type App struct {
	ctx              context.Context
	mu               sync.Mutex
	configOK         bool
	httpServer       *http.Server
	livegoCancel     context.CancelFunc
	frpcCancel       context.CancelFunc
	livegoListeners  []net.Listener
	wg               sync.WaitGroup
	htmlTitle        string
	streamKeyCancel  context.CancelFunc
	backendIsRunning bool
}

// StreamKeyResponse struct for JSON parsing
type StreamKeyResponse struct {
	Data string `json:"data"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		configOK:  false,
		htmlTitle: "GoStream", // Default title
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	a.configOK = true
	runtime.WindowShow(ctx)
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Stop all services
	if a.backendIsRunning {
		if a.streamKeyCancel != nil {
			a.streamKeyCancel()
		}
		a.stopLivego()
		a.stopWebServer()
	}
	a.stopFrpc()

	// Wait for all goroutines to finish
	a.wg.Wait()
}

// beforeClose is called when the user attempts to close the window
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	if !a.configOK {
		return false
	}
	dialog, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:    runtime.QuestionDialog,
		Title:   "退出? / Quit?",
		Message: "确定要退出吗? / Are you sure you want to quit?",
	})

	if err != nil {
		return false
	}

	if dialog == "Yes" {
		log.Print("User chose to quit")
		return false // Allow shutdown
	}

	log.Print("User chose to cancel quitting")
	return true // Prevent shutdown
}

// GetStreamKey fetches the stream key from the LiveGo server.
func (a *App) GetStreamKey(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stream key fetching cancelled.")
			return
		case <-ticker.C:
			resp, err := http.Get("http://127.0.0.1:7180/control/get?room=movie")
			if err != nil {
				log.Printf("Error fetching stream key: %v", err)
				runtime.EventsEmit(a.ctx, "service:log", "system", "Error fetching stream key, retrying...")
				continue // Retry on next tick
			}

			if resp.StatusCode != http.StatusOK {
				log.Printf("Unexpected status code %d while fetching stream key", resp.StatusCode)
				resp.Body.Close()
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close() // Close body immediately after reading
			if err != nil {
				log.Printf("Error reading stream key response: %v", err)
				runtime.EventsEmit(a.ctx, "service:log", "system", "Error reading stream key response: "+err.Error())
				continue
			}

			var streamKeyResp StreamKeyResponse
			if err := json.Unmarshal(body, &streamKeyResp); err != nil {
				log.Printf("Error parsing stream key JSON: %v", err)
				runtime.EventsEmit(a.ctx, "service:log", "system", "Error parsing stream key JSON: "+err.Error())
				continue
			}

			log.Printf("Fetched stream key: %s", streamKeyResp.Data)
			runtime.EventsEmit(a.ctx, "streamkey:update", streamKeyResp.Data)
			return // Success, exit the loop
		}
	}
}

// UpdateHtmlTitle updates the title for the HTML template.
func (a *App) UpdateHtmlTitle(newTitle string) {
	a.mu.Lock()
	a.htmlTitle = newTitle
	a.mu.Unlock()
	log.Printf("HTML title updated to: %s", newTitle)
	runtime.EventsEmit(a.ctx, "service:log", "system", "HTML title updated to: "+newTitle)
	runtime.EventsEmit(a.ctx, "title:updated", newTitle)
}

func (a *App) pipeOutput(name string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("[%s] %s", name, line)
		runtime.EventsEmit(a.ctx, "service:log", name, line)
	}
}

func (a *App) startLivego() {
	if a.livegoCancel != nil {
		log.Println("LiveGo is already running.")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.livegoCancel = cancel

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer func() {
			log.Println("LiveGo process finished.")
			runtime.EventsEmit(a.ctx, "service:status", "livego", "Stopped")
		}()

		log.Println("Starting LiveGo service...")
		runtime.EventsEmit(a.ctx, "service:status", "livego", "Running")

		a.runLivego(ctx)

		<-ctx.Done()
	}()
}

func (a *App) stopLivego() {
	if a.livegoCancel != nil {
		log.Println("Stopping LiveGo service...")
		a.livegoCancel()
		a.livegoCancel = nil // Prevent multiple calls
	}
	// Close all registered listeners to stop the server goroutines
	for _, l := range a.livegoListeners {
		if l != nil {
			l.Close()
		}
	}
	a.livegoListeners = nil // Clear the slice
}

func (a *App) startFrpc() {
	if _, err := os.Stat("frpc.toml"); os.IsNotExist(err) {
		log.Println("frpc.toml not found, frpc will not start.")
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "错误 / Error",
			Message: "frpc.toml 未找到, frpc 将不会启动。 / frpc.toml not found, frpc will not start.",
		})
		return
	}

	dialog, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:    runtime.QuestionDialog,
		Title:   "安全与合规性注意事项",
		Message: "当您使用内网穿透 (FRP)功能时，您的本地直播服务将被暴露在公共互联网上。这意味着任何人都可以通过您的公网地址访问您的直播内容。\n请确保您直播的内容适合公开传播，并注意保护个人隐私和敏感信息。严格遵守您所在国家及服务所在地的法律法规、行业惯例和社会公共道德。严禁发布任何违反法律法规的内容，包括但不限于：\n\t涉及国家秘密或安全的信息。\n\t任何形式的黑客攻击、网络破坏行为。\n\t妨碍互联网运行安全或侵犯他人合法权益的信息。\n您必须为通过本应用发布的所有内容负全部责任。\n确定要继续吗?",
	})
	if err != nil {
		log.Printf("Error showing dialog: %v", err)
		return
	}

	if dialog != "Yes" {
		log.Println("User chose not to start frpc.")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.frpcCancel = cancel

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer func() {
			log.Println("FRPC process finished.")
			runtime.EventsEmit(a.ctx, "service:status", "frpc", "Stopped")
		}()

		log.Println("Starting FRPC service...")
		runtime.EventsEmit(a.ctx, "service:status", "frpc", "Running")

		if err := RunFrpc(ctx, "frpc.toml"); err != nil {
			log.Printf("Failed to start frpc: %v", err)
			runtime.EventsEmit(a.ctx, "service:log", "frpc", "Failed to start: "+err.Error())
		}

		<-ctx.Done()
	}()
}

func (a *App) stopFrpc() {
	if a.frpcCancel != nil {
		log.Println("Stopping FRPC service...")
		a.frpcCancel()
		a.frpcCancel = nil // Prevent multiple calls
	}
}

// StartBackend starts the backend services (LiveGo and Caddy).
func (a *App) StartBackend() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.backendIsRunning {
		log.Println("Backend is already running.")
		return
	}

	// Show privacy warning on first start
	go func() {
		_, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.InfoDialog,
			Title:   "服务正在启动 / Services Starting",
			Message: "服务会启动本地端口，请注意安全。\n\nServices will start on local ports, please be aware of security.",
		})

		if err != nil {
			log.Print(err)
		}
	}()

	// Ensure backend services are stopped before starting
	a.stopLivego()
	a.stopWebServer()

	// Start LiveGo and the web server
	a.startLivego()
	a.startWebServer()
	a.backendIsRunning = true

	// Fetch stream key with retries and cancellation
	runtime.EventsEmit(a.ctx, "streamkey:update", "正在获取... / Fetching...")
	ctx, cancel := context.WithCancel(context.Background())
	a.streamKeyCancel = cancel
	go a.GetStreamKey(ctx)
}

// StopBackend stops the backend services.
func (a *App) StopBackend() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.backendIsRunning {
		log.Println("Backend is not running.")
		return
	}

	if a.streamKeyCancel != nil {
		a.streamKeyCancel()
		a.streamKeyCancel = nil
	}
	a.stopLivego()
	a.stopWebServer()
	a.backendIsRunning = false
	runtime.EventsEmit(a.ctx, "streamkey:update", "N/A")
}

func (a *App) startWebServer() {
	mux := http.NewServeMux()

	// Create a sub-filesystem rooted at "build/web"
	webFS, err := fs.Sub(webAssets, "build/web")
	if err != nil {
		log.Fatalf("Failed to create sub filesystem: %v", err)
	}
	fileServer := http.FileServer(http.FS(webFS))

	titleRegex := regexp.MustCompile(`<title>(.*?)<\/title>`)

	// Custom handler to inject title and serve files
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath := path.Clean(r.URL.Path)
		isHTML := false
		htmlPath := ""

		switch reqPath {
		case "/", "/index.html":
			isHTML = true
			htmlPath = "index.html"
		case "/flv/", "/flv/index.html":
			isHTML = true
			htmlPath = "flv/index.html"
		case "/hls/", "/hls/index.html":
			isHTML = true
			htmlPath = "hls/index.html"
		}

		if isHTML {
			content, err := fs.ReadFile(webFS, htmlPath)
			if err != nil {
				http.Error(w, "File not found", http.StatusNotFound)
				log.Printf("Error reading embedded file %s: %v", htmlPath, err)
				return
			}

			a.mu.Lock()
			title := a.htmlTitle
			a.mu.Unlock()

			newContent := titleRegex.ReplaceAllString(string(content), "<title>"+title+"</title>")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(newContent))
			return
		}

		// For all other files, use the default file server
		fileServer.ServeHTTP(w, r)
	})

	mux.Handle("/", handler)

	// Reverse proxy for the LiveGo API
	livegoURL, _ := url.Parse("http://127.0.0.1:8090")
	proxyAPI := httputil.NewSingleHostReverseProxy(livegoURL)
	mux.Handle("/control/", proxyAPI)

	// Reverse proxy for HLS/M3U8 streams
	livegoHLS, _ := url.Parse("http://127.0.0.1:7002")
	proxyHLS := httputil.NewSingleHostReverseProxy(livegoHLS)
	mux.Handle("/live/movie.m3u8", proxyHLS)
	mux.Handle("/live/movie/", proxyHLS)

	// Reverse proxy for FLV streams
	livegoFLV, _ := url.Parse("http://127.0.0.1:7001")
	proxyFLV := httputil.NewSingleHostReverseProxy(livegoFLV)
	mux.Handle("/live/movie.flv", proxyFLV)

	// CORS middleware
	corsHandler := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if regexp.MustCompile("^/live/").MatchString(r.URL.Path) {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
			h.ServeHTTP(w, r)
		})
	}

	a.httpServer = &http.Server{
		Addr:    ":7180", // Match Caddy's port
		Handler: corsHandler(mux),
	}

	go func() {
		log.Println("Starting web server on :7180")
		if err := a.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()
}

func (a *App) stopWebServer() {
	if a.httpServer != nil {
		log.Println("Stopping web server...")
		// Create a context with a timeout to allow for graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Web server Shutdown error: %v", err)
		} else {
			log.Println("Web server stopped.")
		}
		a.httpServer = nil
	}
}

// ToggleFRPC starts or stops the FRPC service.
func (a *App) ToggleFRPC() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.frpcCancel != nil {
		a.stopFrpc()
	} else {
		a.startFrpc()
	}
}
