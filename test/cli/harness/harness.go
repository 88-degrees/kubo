package harness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

// Harness is used within the context of a single test, setting up the test environment, tracking state, and cleaning up.
type Harness struct {
	Dir     string
	IPFSBin string

	// IPFSMountpoint string
	// IPNSMountpoint string
	// IPFSPath       string
	// APIFile string

	Runner  *Runner
	Cluster *Cluster

	// // Environment variables that are set on every process run through the harness.
	// Env map[string]string
	// Dir string
}

func EnableDebugLogging() {
	err := logging.SetLogLevel("testharness", "DEBUG")
	if err != nil {
		panic(err)
	}
}

// NewT constructs a harness that cleans up after the given test is done.
func NewT(t *testing.T, options ...func(h *Harness)) *Harness {
	h := New(options...)
	t.Cleanup(h.Cleanup)
	return h
}

func New(options ...func(h *Harness)) *Harness {
	h := &Harness{Runner: &Runner{Env: osEnviron()}}

	absIPFSPath := absPath(filepath.FromSlash("../../cmd/ipfs/ipfs"))

	h.IPFSBin = absIPFSPath

	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		log.Panicf("error creating temp dir: %s", err)
	}
	h.Dir = tmpDir
	h.Runner.Dir = h.Dir

	// h.IPFSMountpoint = filepath.Join(h.Dir, "ipfs")
	// h.IPNSMountpoint = filepath.Join(h.Dir, "ipns")

	// h.IPFSPath = filepath.Join(h.Dir, ".ipfs")
	// h.Runner.Env["IPFS_PATH"] = h.IPFSPath

	// h.APIFile = filepath.Join(h.IPFSPath, "api")

	// daemonEnv := osEnviron()
	// daemonEnv["IPFS_PATH"] = h.IPFSPath
	// h.Daemon = &Daemon{
	// 	Runner:  &Runner{Env: daemonEnv},
	// 	IPFSBin: h.IPFSBin,
	// 	APIFile: h.APIFile,
	// }

	clusterRoot := filepath.Join(h.Dir, ".cluster")
	iptbEnv := osEnviron()
	h.Cluster = &Cluster{
		ClusterRoot: clusterRoot,
		IPFSBin:     absIPFSPath,
		Runner:      &Runner{Env: iptbEnv},
	}

	// apply any customizations
	// this should happen after all initialization
	for _, o := range options {
		o(h)
	}

	return h
}
func absPath(rel string) string {
	abs, err := filepath.Abs(rel)
	if err != nil {
		log.Panicf("unable to find absolute path of %s: %s", rel, err)
	}
	return abs
}

func osEnviron() map[string]string {
	m := map[string]string{}
	for _, entry := range os.Environ() {
		split := strings.Split(entry, "=")
		m[split[0]] = split[1]
	}
	return m
}

// WriteToTemp writes the given contents to a guaranteed-unique temp file, returning its path.
func (h *Harness) WriteToTemp(contents string) string {
	f := h.TempFile()
	_, err := f.WriteString(contents)
	if err != nil {
		log.Panicf("writing to temp file: %s", err.Error())
	}
	err = f.Close()
	if err != nil {
		log.Panicf("closing temp file: %s", err.Error())
	}
	return f.Name()
}

// TempFile creates a new unique temp file.
func (h *Harness) TempFile() *os.File {
	f, err := os.CreateTemp(h.Dir, "")
	if err != nil {
		log.Panicf("creating temp file: %s", err.Error())
	}
	return f
}

// WriteFile writes a file given a filename and its contents.
// The filename should be a relative path.
func (h *Harness) WriteFile(filename, contents string) {
	if filepath.IsAbs(filename) {
		log.Panicf("%s must be a relative path", filename)
	}
	absPath := filepath.Join(h.Runner.Dir, filename)
	err := os.WriteFile(absPath, []byte(contents), 0644)
	if err != nil {
		log.Panicf("writing '%s' ('%s'): %s", filename, absPath, err.Error())
	}
}

func WaitForFile(path string, timeout time.Duration) error {
	start := time.Now()
	timer := time.NewTimer(timeout)
	ticker := time.NewTicker(1 * time.Millisecond)
	defer timer.Stop()
	defer ticker.Stop()
	for {
		select {
		case <-timer.C:
			end := time.Now()
			return fmt.Errorf("timeout waiting for %s after %v", path, end.Sub(start))
		case <-ticker.C:
			_, err := os.Stat(path)
			if err == nil {
				return nil
			}
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("error waiting for %s: %w", path, err)
		}
	}
}

func (h *Harness) Mkdirs(paths ...string) {
	for _, path := range paths {
		if filepath.IsAbs(path) {
			log.Panicf("%s must be a relative path when making dirs", path)
		}
		absPath := filepath.Join(h.Runner.Dir, path)
		err := os.MkdirAll(absPath, 0777)
		if err != nil {
			log.Panicf("recursively making dirs under %s: %s", absPath, err)
		}
	}
}

func (h *Harness) Sh(expr string) RunResult {
	return h.Runner.Run(RunRequest{
		Path: "bash",
		Args: []string{"-c", expr},
	})
}

func (h *Harness) Cleanup() {
	h.Cluster.Stop()
	// TODO: don't do this if test fails, not sure how?
	err := os.RemoveAll(h.Dir)
	if err != nil {
		log.Panicf("removing temp dir %s: %s", h.Dir, err)
	}
}
