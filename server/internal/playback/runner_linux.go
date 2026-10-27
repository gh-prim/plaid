package playback

import (
	"os/exec"
	"runtime"
	"syscall"
)

// procAttr puts FFmpeg in its own process group and has Linux kill it
// when the thread that forked it dies (R28).
func procAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}

// spawns carries process starts to the spawner thread.
var spawns = make(chan spawnRequest)

type spawnRequest struct {
	cmd  *exec.Cmd
	done chan error
}

func init() {
	// Pdeathsig fires when the forking *thread* exits, and Go moves
	// goroutines between threads and retires idle ones. Every FFmpeg is
	// started from this goroutine, locked to its thread, which never
	// returns: the signal then only fires when PLAID itself dies.
	go func() {
		runtime.LockOSThread()
		for req := range spawns {
			req.done <- req.cmd.Start()
		}
	}()
}

func spawn(cmd *exec.Cmd) error {
	req := spawnRequest{cmd: cmd, done: make(chan error, 1)}
	spawns <- req
	return <-req.done
}
