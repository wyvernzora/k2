package vm

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"golang.org/x/term"
)

// consoleEscape is the byte sent by Ctrl-] (ASCII GS, group separator),
// the same escape character telnet has used since 1983. Operators are
// likely to remember it without a cheat-sheet.
const consoleEscape = 0x1d

// attachConsole opens an interactive bidirectional bridge to the VM's
// serial console socket. The host stdin is put into raw mode for the
// duration so keystrokes flow through unchanged; Ctrl-] detaches and
// returns. Output is mirrored to the host stdout.
//
// Persistent logging is handled by QEMU's chardev `logfile=` attribute,
// not by this function — the host can `tail -f <ConsoleLog>` from
// another window without interfering.
func (r Runner) attachConsole(meta Metadata) error {
	if err := preflightConsole(meta); err != nil {
		return err
	}

	conn, err := net.Dial("unix", meta.ConsoleSocket)
	if err != nil {
		return fmt.Errorf("dial console socket %s: %w (was the VM started under an older k2-tools? stop and start it again)", meta.ConsoleSocket, err)
	}
	defer conn.Close()

	stdin := os.Stdin
	rawTTY, restore, err := enterRawTTY(stdin)
	if err != nil {
		return err
	}
	defer restore()

	var (
		wg    sync.WaitGroup
		once  sync.Once
		errCh = make(chan error, 2)
	)

	stop := func() {
		once.Do(func() {
			_ = conn.Close() // wakes the VM→stdout goroutine
			// Closing os.Stdin here would also wake the stdin reader,
			// but it's a process-global fd; safer to let the reader
			// hit EOF naturally on the next byte.
		})
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		pumpVMToStdout(conn, errCh, stop)
	}()
	go func() {
		defer wg.Done()
		pumpStdinToVM(stdin, conn, rawTTY, errCh, stop)
	}()

	wg.Wait()
	if rawTTY {
		fmt.Fprint(os.Stderr, "\r\nDetached.\r\n")
	}

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// preflightConsole checks the metadata invariants attachConsole relies
// on before any I/O — surfaces a clear error if the VM was created
// pre-console-socket support or simply isn't running.
func preflightConsole(meta Metadata) error {
	if meta.ConsoleSocket == "" {
		return errors.New("VM metadata has no console socket; re-create the VM after upgrading k2-tools")
	}
	if !isRunning(meta) {
		return fmt.Errorf("%s is not running", meta.Name)
	}
	return nil
}

// enterRawTTY puts stdin into raw mode when it's a terminal so
// keystrokes flow through to QEMU unchanged. Returns whether stdin
// was a TTY and a restore func that's safe to call unconditionally
// (no-op when stdin wasn't a TTY).
func enterRawTTY(stdin *os.File) (rawTTY bool, restore func(), err error) {
	fd := int(stdin.Fd())
	if !term.IsTerminal(fd) {
		return false, func() {}, nil
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return false, func() {}, fmt.Errorf("set stdin raw mode: %w", err)
	}
	fmt.Fprint(os.Stderr, "Connected to console. Press Ctrl-] to detach.\r\n")
	return true, func() { _ = term.Restore(fd, oldState) }, nil
}

// pumpVMToStdout copies QEMU console output to host stdout until the
// socket closes or errors. Net-closed / EOF unwind silently — they
// just mean the operator detached.
func pumpVMToStdout(conn net.Conn, errCh chan<- error, stop func()) {
	_, err := io.Copy(os.Stdout, conn)
	if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
		errCh <- fmt.Errorf("vm→stdout: %w", err)
	}
	stop()
}

// pumpStdinToVM forwards host stdin to the console socket. When stdin
// is a TTY, the Ctrl-] escape detaches the session: any pre-escape
// bytes flush first, then the loop exits.
func pumpStdinToVM(stdin io.Reader, conn net.Conn, rawTTY bool, errCh chan<- error, stop func()) {
	buf := make([]byte, 1024)
	for {
		n, err := stdin.Read(buf)
		if n > 0 {
			if done := writeStdinChunk(buf[:n], conn, rawTTY, errCh); done {
				stop()
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				errCh <- fmt.Errorf("stdin: %w", err)
			}
			stop()
			return
		}
	}
}

// writeStdinChunk forwards one Read's worth of bytes to the VM. It
// returns true when the caller should stop pumping — either because
// the operator hit Ctrl-] (TTY mode) or because conn rejected the
// write. Pre-escape bytes are flushed before signalling stop.
func writeStdinChunk(chunk []byte, conn net.Conn, rawTTY bool, errCh chan<- error) bool {
	if rawTTY {
		if idx := bytes.IndexByte(chunk, consoleEscape); idx >= 0 {
			if idx > 0 {
				if _, werr := conn.Write(chunk[:idx]); werr != nil {
					errCh <- fmt.Errorf("stdin→vm: %w", werr)
				}
			}
			return true
		}
	}
	if _, werr := conn.Write(chunk); werr != nil {
		if !errors.Is(werr, net.ErrClosed) {
			errCh <- fmt.Errorf("stdin→vm: %w", werr)
		}
		return true
	}
	return false
}
