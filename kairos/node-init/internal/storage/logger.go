package storage

import (
	"fmt"
	"log/syslog"
	"os"
)

type logger struct {
	prefix string
	syslog *syslog.Writer
}

func newLogger(prefix string) logger {
	writer, _ := syslog.New(syslog.LOG_INFO, prefix)
	return logger{prefix: prefix, syslog: writer}
}

func (l logger) Close() {
	if l.syslog != nil {
		_ = l.syslog.Close()
	}
}

func (l logger) Printf(format string, args ...any) {
	msg := fmt.Sprintf(l.prefix+": "+format, args...)
	fmt.Println(msg)
	if l.syslog != nil {
		_ = l.syslog.Info(msg)
	}
	if f, err := os.OpenFile("/dev/kmsg", os.O_WRONLY, 0); err == nil {
		_, _ = fmt.Fprintln(f, msg)
		_ = f.Close()
	}
}
