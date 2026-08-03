package recon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"sync"
	"time"
)

var ErrBusy = errors.New("recon already running")

type Job struct {
	ID     string  `json:"id"`
	Status string  `json:"status"`
	Result *Result `json:"result,omitempty"`
	Err    string  `json:"error,omitempty"`
}

type Runner struct {
	mu   sync.Mutex
	jobs map[string]*Job
	busy bool
}

func NewRunner() *Runner { return &Runner{jobs: map[string]*Job{}} }

func validateTarget(target string) error {
	u, err := url.Parse(target)
	if err != nil {
		return err
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid target URL")
	}
	return nil
}

// Get returns a snapshot copy of the job (safe to read without locking) so
// callers never race with the runner goroutine mutating the live job.
func (r *Runner) Get(id string) (*Job, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return nil, false
	}
	cp := *j
	return &cp, true
}

// Start validates the target, ensures no other job is running (ErrBusy → 409),
// and spawns the recon subprocess with an argv slice (no shell). Progress lines
// from stderr are forwarded to the optional progress callback. On completion the
// parsed Result is stored on the Job.
func (r *Runner) Start(nodePath, script, target string, depth, maxPages int, progress func(string)) (string, error) {
	if err := validateTarget(target); err != nil {
		return "", err
	}
	r.mu.Lock()
	if r.busy {
		r.mu.Unlock()
		return "", ErrBusy
	}
	r.busy = true
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	job := &Job{ID: id, Status: "running"}
	r.jobs[id] = job
	r.mu.Unlock()

	go func() {
		defer func() {
			r.mu.Lock()
			r.busy = false
			r.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, nodePath, script,
			"--target", target,
			"--depth", fmt.Sprintf("%d", depth),
			"--max-pages", fmt.Sprintf("%d", maxPages))

		stderr, _ := cmd.StderrPipe()
		out, err := cmd.StdoutPipe()
		if err != nil {
			r.fail(id, err)
			return
		}
		if err := cmd.Start(); err != nil {
			r.fail(id, err)
			return
		}
		// stream progress from stderr
		go func() {
			sc := bufio.NewScanner(stderr)
			for sc.Scan() {
				var p struct {
					Msg string `json:"msg"`
				}
				if json.Unmarshal(sc.Bytes(), &p) == nil && progress != nil {
					progress(p.Msg)
				}
			}
		}()
		data, _ := io.ReadAll(out)
		if err := cmd.Wait(); err != nil {
			r.fail(id, err)
			return
		}
		res, err := Parse(data)
		if err != nil {
			r.fail(id, err)
			return
		}
		r.mu.Lock()
		job.Status = "done"
		job.Result = res
		r.mu.Unlock()
	}()

	return id, nil
}

func (r *Runner) fail(id string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if j := r.jobs[id]; j != nil {
		j.Status = "failed"
		j.Err = err.Error()
	}
}
