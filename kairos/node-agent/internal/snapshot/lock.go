package snapshot

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

const (
	lockPollInterval = 100 * time.Millisecond
	lockLogInterval  = 10 * time.Second
)

type fileLock struct {
	file *os.File
}

func acquireFileLock(path string, logf func(string, ...any)) (*fileLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}

	var nextLog time.Time
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return &fileLock{file: file}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			closeErr := file.Close()
			return nil, errors.Join(
				fmt.Errorf("acquire lock %s: %w", path, err),
				wrapCloseError(path, closeErr),
			)
		}

		now := time.Now()
		if nextLog.IsZero() || !now.Before(nextLog) {
			logf("waiting for lock path=%s", path)
			nextLog = now.Add(lockLogInterval)
		}
		time.Sleep(lockPollInterval)
	}
}

func (l *fileLock) Close() error {
	path := l.file.Name()
	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	return errors.Join(
		wrapUnlockError(path, unlockErr),
		wrapCloseError(path, closeErr),
	)
}

func wrapUnlockError(path string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("unlock %s: %w", path, err)
}

func wrapCloseError(path string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("close lock file %s: %w", path, err)
}
