package vm

import (
	"io"
	"os"

	"github.com/wyvernzora/k2/kairos/tools/internal/ui"
)

const defaultArtifactBaseURL = "https://io.wyvernzora.k2.images.s3.us-west-2.amazonaws.com"

type Runner struct {
	RepoRoot string
	Reporter *ui.Reporter
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
}

func (r Runner) stdin() io.Reader {
	if r.Stdin != nil {
		return r.Stdin
	}
	return os.Stdin
}

func (r Runner) stdout() io.Writer {
	if r.Stdout != nil {
		return r.Stdout
	}
	return os.Stdout
}

func (r Runner) stderr() io.Writer {
	if r.Stderr != nil {
		return r.Stderr
	}
	return os.Stderr
}

func (r Runner) logf(format string, args ...any) {
	if r.Reporter != nil {
		r.Reporter.Infof(format, args...)
	}
}

func (r Runner) successf(format string, args ...any) {
	if r.Reporter != nil {
		r.Reporter.Successf(format, args...)
	}
}

type CreateOptions struct {
	Preset  string
	ID      string
	RawXZ   string
	SSHPort string
	APIPort string
	Start   bool
	Sudo    bool
}

type StartOptions struct {
	ID   string
	Sudo bool
}

type StopOptions struct {
	ID  string
	All bool
}

type DeleteOptions struct {
	ID    string
	Force bool
	All   bool
}

type Preset struct {
	Description    string         `json:"description"`
	Target         string         `json:"target"`
	MemoryMB       int            `json:"memoryMb"`
	CPUs           int            `json:"cpus"`
	Network        Network        `json:"network"`
	PersistentDisk PersistentDisk `json:"persistentDisk"`
	Forwards       []Forward      `json:"forwards"`
	name           string
}

type Network struct {
	Mode string `json:"mode"`
}

type PersistentDisk struct {
	Enabled bool `json:"enabled"`
	SizeMB  int  `json:"sizeMb"`
}

type Forward struct {
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	HostIP    string `json:"hostIp"`
	HostPort  string `json:"hostPort"`
	GuestPort int    `json:"guestPort"`
}

type Metadata struct {
	Backend         string `json:"backend"`
	ID              string `json:"id"`
	Name            string `json:"name"`
	Preset          string `json:"preset"`
	Target          string `json:"target"`
	RawXZ           string `json:"rawXz"`
	VMDir           string `json:"vmDir"`
	KairosQCOW2     string `json:"kairosQcow2"`
	PersistentQCOW2 string `json:"persistentQcow2"`
	SSHPort         int    `json:"sshPort"`
	APIPort         int    `json:"apiPort"`
	MonitorPort     int    `json:"monitorPort"`
	QGAPort         int    `json:"qgaPort"`
	NetworkMode     string `json:"networkMode"`
	MACAddress      string `json:"macAddress"`
	MemoryMB        int    `json:"memoryMb"`
	CPUs            int    `json:"cpus"`
	PIDFile         string `json:"pidFile"`
	QEMULog         string `json:"qemuLog"`
	ConsoleLog      string `json:"consoleLog"`
	ConsoleSocket   string `json:"consoleSocket"`
}

type artifactManifest struct {
	Git struct {
		SHA string `json:"sha"`
	} `json:"git"`
	S3 struct {
		Prefix string `json:"prefix"`
	} `json:"s3"`
	Compressed artifactFile `json:"compressed"`
	Artifact   struct {
		Compressed artifactFile `json:"compressed"`
	} `json:"artifact"`
}

type artifactFile struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}
