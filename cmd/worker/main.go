// Command worker runs background jobs: media processing (image thumbnails,
// video transcode via ffmpeg), notification fan-out (FCM), and feed precompute.
//
// This is a placeholder loop. Wire it to a Redis-backed queue (asynq) once the
// notification/media modules define their tasks.
package main

import (
	"os"
	"os/signal"
	"syscall"

	"ruammit-backend/internal/platform/config"
	"ruammit-backend/internal/platform/logging"
)

func main() {
	cfg := config.Load()
	log := logging.New(cfg.Env)

	log.Info("worker started", "env", cfg.Env)

	// TODO: start asynq server and register task handlers, e.g.
	//   mux.HandleFunc(notification.TypePushFanout, notification.HandlePushFanout)
	//   mux.HandleFunc(media.TypeTranscodeVideo, media.HandleTranscodeVideo)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	sig := <-stop
	log.Info("worker stopping", "signal", sig.String())
}
