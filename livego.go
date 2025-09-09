package main

import (
	"context"
	"net"

	"github.com/gwuhaolin/livego/configure"
	"github.com/gwuhaolin/livego/protocol/api"
	"github.com/gwuhaolin/livego/protocol/hls"
	"github.com/gwuhaolin/livego/protocol/httpflv"
	"github.com/gwuhaolin/livego/protocol/rtmp"

	log "github.com/sirupsen/logrus"
)

func (a *App) startHls(ctx context.Context) (*hls.Server, error) {
	hlsAddr := configure.Config.GetString("hls_addr")
	hlsListen, err := net.Listen("tcp", hlsAddr)
	if err != nil {
		return nil, err
	}
	a.livegoListeners = append(a.livegoListeners, hlsListen)

	hlsServer := hls.NewServer()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("HLS server panic: ", r)
			}
		}()
		log.Info("HLS listen On ", hlsAddr)
		hlsServer.Serve(hlsListen)
	}()
	return hlsServer, nil
}

func (a *App) startRtmp(ctx context.Context, stream *rtmp.RtmpStream, hlsServer *hls.Server) error {
	rtmpAddr := configure.Config.GetString("rtmp_addr")

	rtmpListen, err := net.Listen("tcp", rtmpAddr)
	if err != nil {
		return err
	}
	a.livegoListeners = append(a.livegoListeners, rtmpListen)

	rtmpServer := rtmp.NewRtmpServer(stream, hlsServer)

	log.Info("RTMP Listen On ", rtmpAddr)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("RTMP server panic: ", r)
			}
		}()
		rtmpServer.Serve(rtmpListen)
	}()
	return nil
}

func (a *App) startHTTPFlv(ctx context.Context, stream *rtmp.RtmpStream) error {
	httpflvAddr := configure.Config.GetString("httpflv_addr")

	flvListen, err := net.Listen("tcp", httpflvAddr)
	if err != nil {
		return err
	}
	a.livegoListeners = append(a.livegoListeners, flvListen)

	hdlServer := httpflv.NewServer(stream)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("HTTP-FLV server panic: ", r)
			}
		}()
		log.Info("HTTP-FLV listen On ", httpflvAddr)
		hdlServer.Serve(flvListen)
	}()
	return nil
}

func (a *App) startAPI(ctx context.Context, stream *rtmp.RtmpStream) error {
	apiAddr := configure.Config.GetString("api_addr")
	rtmpAddr := configure.Config.GetString("rtmp_addr")

	if apiAddr != "" {
		opListen, err := net.Listen("tcp", apiAddr)
		if err != nil {
			return err
		}
		a.livegoListeners = append(a.livegoListeners, opListen)
		opServer := api.NewServer(stream, rtmpAddr)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("HTTP-API server panic: ", r)
				}
			}()
			log.Info("HTTP-API listen On ", apiAddr)
			opServer.Serve(opListen)
		}()
	}
	return nil
}
func (a *App) runLivego(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("livego panic: ", r)
		}
	}()

	// Use hardcoded default configuration instead of a file
	configure.Config.Set("rtmp_addr", ":1935")
	configure.Config.Set("httpflv_addr", ":7001")
	configure.Config.Set("hls_addr", ":7002")
	configure.Config.Set("api_addr", ":8090")

	apps := configure.Applications{
		{
			Appname: "live",
			Live:    true,
			Hls:     true,
			Api:     true,
			Flv:     true,
		},
	}

	for _, app := range apps {
		stream := rtmp.NewRtmpStream()
		var hlsServer *hls.Server
		var err error

		if app.Hls {
			if hlsServer, err = a.startHls(ctx); err != nil {
				log.Errorf("startHls failed: %v", err)
				return
			}
		}
		if app.Flv {
			if err = a.startHTTPFlv(ctx, stream); err != nil {
				log.Errorf("startHTTPFlv failed: %v", err)
				return
			}
		}
		if app.Api {
			if err = a.startAPI(ctx, stream); err != nil {
				log.Errorf("startAPI failed: %v", err)
				return
			}
		}

		if err = a.startRtmp(ctx, stream, hlsServer); err != nil {
			log.Errorf("startRtmp failed: %v", err)
			return
		}
	}

	// Wait for the context to be cancelled
	<-ctx.Done()
	log.Info("LiveGo services are shutting down...")
}
