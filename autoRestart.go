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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
)

var (
	executablePath string

	initOnce sync.Once
	Log      func(string, ...interface{})
)

type dir struct {
	path string
	cb   func()
}

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
	return nil
}

// Auto reload the current process when its binary changes.
//
// The Log function is used to display an informational startup message and
// errors. It works well with e.g. the standard Log package or Logrus.
//
// The error return will only return initialisation errors. Once initialized it
// will use the Log function to print errors, rather than return.
func Watch(additional ...dir) (err error) {
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

	// Watch the directory, because a recompile renames the existing
	// file (rather than rewriting it), so we won't get events for that.
	dirs := make([]string, len(additional)+1)
	dirs[0] = filepath.Dir(executablePath)

	for i := range additional {
		path, err := filepath.Abs(additional[i].path)
		if err != nil {
			return errors.Wrapf(err, "can't get absolute path to %q: %v",
				additional[i].path, err)
		}

		s, err := os.Stat(path)
		if err != nil {
			return errors.Wrap(err, "os.Stat")
		}
		if !s.IsDir() {
			return errors.Errorf("not a directory: %q; can only watch directories",
				additional[i].path)
		}

		additional[i].path = path
		dirs[i+1] = path
	}

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
				var trigger bool
				switch runtime.GOOS {
				case "darwin", "freebsd", "openbsd", "netbsd", "dragonfly":
					trigger = event.Op&fsnotify.Create == fsnotify.Create
				case "linux":
					trigger = event.Op&fsnotify.Write == fsnotify.Write
				default:
					trigger = event.Op&fsnotify.Create == fsnotify.Create
					Log("reload: untested OS(GOOS) %q; this package may not work correctly", runtime.GOOS)
				}

				if !trigger {
					continue
				}

				if event.Name == executablePath {
					// TODO perfect this
					// Wait for writes to finish.
					time.Sleep(150 * time.Millisecond)
					Exec()
				}

				for _, a := range additional {
					if strings.HasPrefix(event.Name, a.path) {
						time.Sleep(100 * time.Millisecond)
						a.cb()
					}
				}
			}
		}
	}()

	for _, d := range dirs {
		if err := fileWatcher.Add(d); err != nil {
			return errors.Wrapf(err, "can't add %q to watcher", d)
		}
	}

	add := ""
	if len(additional) > 0 {
		reldirs := make([]string, len(dirs)-1)
		for i := range dirs[1:] {
			reldirs[i] = calcRelativePath(dirs[i+1])
		}
		add = fmt.Sprintf(" (additional dirs: %s)", strings.Join(reldirs, ", "))
	}
	Log("watching %q %s,restart when it changes", calcRelativePath(executablePath), add)
	<-done
	return nil
}

// Exec replaces the current process with a new copy of itself.
func Exec() {
	if err := syscall.Exec(executablePath, append([]string{executablePath}, os.Args[1:]...), os.Environ()); err != nil {
		panic(fmt.Sprintf("cannot restart: %v", err))
	}
}

//  calculate relative path
func calcRelativePath(p string) string {
	if cwd, err := os.Getwd(); err != nil {
		return p
	} else {
		if strings.HasPrefix(p, cwd) {
			return "./" + strings.TrimLeft(p[len(cwd):], "/")
		}
	}

	return p
}
