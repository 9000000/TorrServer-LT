//go:build !windows && !android
// +build !windows,!android

package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"server"

	"server/log"
	"server/settings"
)

func Preconfig(dkill bool) {
	// Raise RLIMIT_NOFILE soft limit to hard limit to prevent "too many open files" (EMFILE)
	// crashes when managing multiple peer connections, disk cache files, and web streams on Linux/POSIX.
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
		if rLimit.Cur < rLimit.Max {
			oldCur := rLimit.Cur
			rLimit.Cur = rLimit.Max
			if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
				// Fallback to a safe intermediate limit if setting to Max fails
				if rLimit.Max >= 4096 && oldCur < 4096 {
					rLimit.Cur = 4096
					_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
				}
			}
		}
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		for s := range sigc {
			if dkill {
				if settings.BTsets().EnableDebug || s != syscall.SIGPIPE {
					log.TLogln("Signal catched:", s)
					log.TLogln("To stop server, close it from web / api")
				}
				continue
			}

			log.TLogln("Signal catched:", s, "stopping server...")

			done := make(chan struct{})

			go func() {
				server.Stop()
				close(done)
			}()

			select {
			case <-done:
				log.TLogln("Server stopped gracefully")
			case <-time.After(5 * time.Second):
				log.TLogln("Server stop timeout, exiting forcefully")
				os.Exit(1)
			}
		}
	}()
}
