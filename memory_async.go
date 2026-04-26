package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const rememberTimeout = 45 * time.Second

type rememberManager struct {
	wg      sync.WaitGroup
	pending atomic.Int64
}

type rememberResult struct {
	stored int
	err    error
}

var asyncRemember rememberManager
var rememberResults = make(chan rememberResult, 256)

func scheduleRememberTurn(userQuery string, aiResponse string) {
	asyncRemember.wg.Add(1)
	asyncRemember.pending.Add(1)

	go func() {
		defer asyncRemember.wg.Done()
		defer asyncRemember.pending.Add(-1)

		ctx, cancel := context.WithTimeout(context.Background(), rememberTimeout)
		defer cancel()

		stored, err := rememberTurn(ctx, userQuery, aiResponse)
		rememberResults <- rememberResult{stored: stored, err: err}
	}()
}

func flushRememberResults() {
	for {
		select {
		case result := <-rememberResults:
			if result.err != nil {
				printMemoryWarning(result.err)
				continue
			}
			if result.stored > 0 {
				printMemorySaved(result.stored)
			} else {
				printMemoryNoop()
			}
		default:
			return
		}
	}
}

func pendingRememberTasks() int64 {
	return asyncRemember.pending.Load()
}

func waitForRememberTasks(reason string) {
	pending := pendingRememberTasks()
	if pending <= 0 {
		return
	}

	if reason == "" {
		reason = "Exiting"
	}
	printMemoryWait(reason, pending)
	asyncRemember.wg.Wait()
	printMemorySyncComplete()
	flushRememberResults()
}

func installInterruptHandler() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		waitForRememberTasks("Interrupt received")
		os.Exit(130)
	}()
}
