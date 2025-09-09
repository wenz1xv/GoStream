package main

import (
	"embed"

	"context"
	"log"

	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/pkg/config"
	"github.com/fatedier/frp/pkg/config/v1/validation"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed all:resources/bin
var binFS embed.FS

// RunFrpc loads the frpc configuration and starts the frpc service
func RunFrpc(ctx context.Context, cfgFile string) error {
	cfg, proxyCfgs, visitorCfgs, _, err := config.LoadClientConfig(cfgFile, true)
	if err != nil {
		return err
	}

	warning, err := validation.ValidateAllClientConfig(cfg, proxyCfgs, visitorCfgs)
	if warning != nil {
		log.Printf("WARNING: %v", warning)
	}
	if err != nil {
		return err
	}

	service, err := client.NewService(client.ServiceOptions{
		Common:         cfg,
		ProxyCfgs:      proxyCfgs,
		VisitorCfgs:    visitorCfgs,
		ConfigFilePath: cfgFile,
	})
	if err != nil {
		return err
	}

	return service.Run(ctx)
}

//go:embed all:resources/web
var webFS embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "GoStream",
		Width:     1024,
		Height:    768,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.beforeClose,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			Theme:                windows.Dark,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
		CSSDragProperty: "--wails-draggable",
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
