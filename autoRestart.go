// Package reload offers lightweight automatic reloading of running processes.
//
// After initialisation with reload.Watch() any changes to the binary will
// restart the process.
//
// Note that this package won't prevent race conditions (e.g. when assigning to
// a global templates variable). You'll need to use sync.RWMutex yourself.
package autoRestart

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
)

var (
	executablePath string
	targetOp       fsnotify.Op

	initOnce sync.Once
	Log      func(string, ...interface{})
)

func initialize() error {
	if Log == nil {
		Log = func(string, ...interface{}) {}
	}

	executablePath = os.Args[0]
	if !filepath.IsAbs(executablePath) {
		var err error
		executablePath, err = os.Executable()
		if err != nil {
			return errors.Wrapf(
				err,
				"cannot get path to binary %q (launch with absolute path): %v",
				os.Args[0],
				err,
			)
		}
	}

	switch runtime.GOOS {
	case "darwin", "freebsd", "openbsd", "netbsd", "dragonfly":
		targetOp = fsnotify.Create
	case "linux":
		targetOp = fsnotify.Write
	default:
		targetOp = fsnotify.Create
		Log("untested OS(GOOS) %q; auto restart may not work", runtime.GOOS)
	}

	return nil
}

// Auto restart the current process when its binary changes.
//
// The Log function is used to display an informational startup message and
// errors. It works well with e.g. the standard Log package or Logrus.
//
// The error return will only return initialisation errors. Once initialized it
// will use the Log function to print errors, rather than return.
func Watch() (err error) {
	initOnce.Do(func() {
		err = initialize()
	})
	if err != nil {
		return
	}

	var fileWatcher *fsnotify.Watcher
	if watcher, err := fsnotify.NewWatcher(); err != nil {
		return errors.Wrap(err, "cannot setup watcher")
	} else {
		fileWatcher = watcher
	}
	defer func() { _ = fileWatcher.Close() }()

	done := make(chan bool)
	go func() {
		for {
			select {
			case err := <-fileWatcher.Errors:
				// Standard logger doesn't have anything other than Print,
				// Panic, and Fatal :-/ Printf() is probably best.
				Log("can't restart. %v", err)
			case event := <-fileWatcher.Events:
				// Ensure that we use the correct events, as they are not uniform accross
				// platforms. See https://github.com/fsnotify/fsnotify/issues/74
				if targetOp != event.Op&targetOp {
					continue
				}

				if event.Name == executablePath {
					// TODO perfect this
					// Wait for writes to finish.
					time.Sleep(150 * time.Millisecond)
					Exec()
				}
			}
		}
	}()

	// Watch the directory, because a recompile renames the existing
	// file (rather than rewriting it), so we won't get events for that.
	if err := fileWatcher.Add(filepath.Dir(executablePath)); err != nil {
		return errors.Wrapf(err, "can't watch folder %q", filepath.Dir(executablePath))
	}

	cwd, _ := os.Getwd()
	relPath, _ := filepath.Rel(cwd, executablePath)
	Log(
		`watching "./%s", restart when it changes`,
		relPath,
	)
	<-done
	return nil
}

// Exec replaces the current process with a new copy of itself.
func Exec() {
	if err := syscall.Exec(executablePath, append([]string{executablePath}, os.Args[1:]...), os.Environ()); err != nil {
		panic(fmt.Sprintf("cannot restart: %v", err))
	}
}
