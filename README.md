[![GoDoc](https://godoc.org/github.com/teamwork/reload?status.svg)](https://godoc.org/github.com/heawercher/autoRestart)

Lightweight automatic reloading of Go processes.

After initialization with `reload.Do()` any changes to the binary (and *only*
the binary) will restart the process.

This is an alternative to the "restart binary after any `*.go` file
changed"-strategy that some other projects – such as
[gin](https://github.com/codegangsta/gin) or
[go-watcher](https://github.com/canthefason/go-watcher) – take.

The advantage of this project's approach is that you have a more control over when
the process restarts, and it only watches a single directory for changes which
has some performance benefits, especially when used over NFS or Docker with a
large number of files.

It also means you won't start a whole bunch of builds if you update 20 files in
a quick succession. On a desktop this probably isn't a huge deal, but on a
laptop it'll save some battery power.

Because it's in-process you can also do things like reloading just templates
instead of recompiling/restarting everything.

this repo is [customized](https://github.com/Teamwork/reload)
