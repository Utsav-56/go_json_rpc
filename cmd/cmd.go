package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

const (
	LogTypeStdErr = "stderr"
	LogTypeStdOut = "stdout"
	maxMemoryLogs = 100

	StatusTransitioning = "transitioning" //  the user just send an command but the process is not started yet
	StatusRunning       = "running"       // the process is running
	StatusStopped       = "stopped"       // the process is stopped (either by user or it exited by itself)
)

type ProcessLog struct {
	Type        string `json:"type"`
	ProcessName string `json:"process_name"`
	Log         string `json:"log"`
	Timestamp   int64  `json:"timestamp"`
}

type ProcessStatus struct {
	Name      string `json:"name"`
	Pid       int    `json:"pid"`
	StartTime int64  `json:"start_time"`
	Status    string `json:"status"` // transitioning, running, stopped
	// if stopped, the exit code
	ExitCode *int `json:"exit_code,omitempty"`
}

func (l *ProcessLog) String() string {
	return fmt.Sprintf("[%s] %s: %s",
		time.UnixMilli(l.Timestamp).Format(time.RFC3339),
		l.Type,
		l.Log)
}

type ControlledProcess struct {
	Name   string
	CMD    *exec.Cmd
	Cancel context.CancelFunc

	Status ProcessStatus

	workDir     string
	logfilePath string
	logfile     *os.File

	logs []ProcessLog
	mu   sync.RWMutex // protects Status, logs, and logfile

	onLog func(ProcessLog)

	onStatusChange func(status ProcessStatus)
}

func (cp *ControlledProcess) UpdateStatus(status string, exitCode *int) {
	cp.mu.Lock()
	cp.Status.Status = status
	cp.Status.ExitCode = exitCode
	// Copy status for callback to avoid holding lock during callback
	statusCopy := cp.Status
	handler := cp.onStatusChange
	cp.mu.Unlock()

	// Call handler outside of lock to prevent deadlocks
	if handler != nil {
		handler(statusCopy)
	}
}

func (cp *ControlledProcess) GetStatus() ProcessStatus {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.Status
}

type ProcessManager struct {
	processes map[string]*ControlledProcess
	mu        sync.RWMutex

	ctx context.Context
}

func NewProcessManager(ctx context.Context) *ProcessManager {
	return &ProcessManager{
		processes: make(map[string]*ControlledProcess),
		ctx:       ctx,
	}
}

type ProcessRequest struct {
	Name        string   `json:"name"`
	Command     string   `json:"command"`
	Args        []string `json:"args,omitempty"`
	WorkDir     string   `json:"work_dir,omitempty"`
	LogfilePath string   `json:"logfile_path,omitempty"`

	onLog          func(ProcessLog)
	onStatusChange func(ProcessStatus)
}

// SetOnLog sets the callback for process logs
func (pr *ProcessRequest) SetOnLog(callback func(ProcessLog)) {
	pr.onLog = callback
}

// SetOnStatusChange sets the callback for status changes
func (pr *ProcessRequest) SetOnStatusChange(callback func(ProcessStatus)) {
	pr.onStatusChange = callback
}

func (pm *ProcessManager) StartProcess(req *ProcessRequest) error {

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.processes[req.Name]; exists {
		return fmt.Errorf("process already exists")
	}

	ctx, cancel := context.WithCancel(pm.ctx)

	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	cmd.Dir = req.WorkDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return err
	}

	var logfile *os.File
	if req.LogfilePath != "" {
		logfile, err = os.OpenFile(req.LogfilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			cancel()
			return err
		}
	}

	if err := cmd.Start(); err != nil {
		cancel()
		if logfile != nil {
			logfile.Close()
		}
		return err
	}

	process := &ControlledProcess{
		Name:   req.Name,
		CMD:    cmd,
		Cancel: cancel,
		Status: ProcessStatus{
			Name:      req.Name,
			Pid:       cmd.Process.Pid,
			StartTime: time.Now().UnixMilli(),
			Status:    StatusTransitioning,
		},
		workDir:        req.WorkDir,
		logfilePath:    req.LogfilePath,
		logfile:        logfile,
		onLog:          req.onLog,
		onStatusChange: req.onStatusChange,
	}

	pm.processes[req.Name] = process

	// Start log capture and monitoring
	go pm.captureLogs(process, stdout, LogTypeStdOut)
	go pm.captureLogs(process, stderr, LogTypeStdErr)
	go pm.waitForExit(process)

	// Update to running status after a brief moment
	go func() {
		time.Sleep(50 * time.Millisecond)
		// Verify process is still running
		if process.CMD.Process != nil && process.CMD.ProcessState == nil {
			process.UpdateStatus(StatusRunning, nil)
		}
	}()

	// Send initial transitioning status
	if req.onStatusChange != nil {
		req.onStatusChange(process.Status)
	}

	return nil
}

func (pm *ProcessManager) waitForExit(process *ControlledProcess) {
	err := process.CMD.Wait()

	// Close log file safely
	process.mu.Lock()
	if process.logfile != nil {
		process.logfile.Close()
		process.logfile = nil
	}
	process.mu.Unlock()

	// Determine exit code
	var exitCode *int
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			code := exitError.ExitCode()
			exitCode = &code
		} else {
			// Non-exit error (e.g., signal)
			code := -1
			exitCode = &code
		}
	} else {
		// Successful exit
		code := 0
		exitCode = &code
	}

	// Update status before removing from map
	process.UpdateStatus(StatusStopped, exitCode)

	// Remove from process map
	pm.mu.Lock()
	delete(pm.processes, process.Name)
	pm.mu.Unlock()
}

func (pm *ProcessManager) captureLogs(process *ControlledProcess, pipe io.Reader, logType string) {
	scanner := bufio.NewScanner(pipe)

	// fix scanner 64KB limit
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		log := ProcessLog{
			Type:        logType,
			ProcessName: process.Name,
			Log:         line,
			Timestamp:   time.Now().UnixMilli(),
		}

		// Store in memory logs and write to file within lock
		process.mu.Lock()
		process.logs = append(process.logs, log)
		if len(process.logs) > maxMemoryLogs {
			process.logs = process.logs[len(process.logs)-maxMemoryLogs:]
		}

		// Write to logfile if available
		if process.logfile != nil {
			process.logfile.WriteString(log.String() + "\n")
		}

		// Get callback reference before unlocking
		handler := process.onLog
		process.mu.Unlock()

		// Call handler outside of lock to prevent deadlocks
		if handler != nil {
			handler(log)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] log scanner error: %v\n", process.Name, err)
	}
}

func (pm *ProcessManager) StopProcess(name string) error {
	pm.mu.RLock()
	process, exists := pm.processes[name]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("process not found")
	}

	// Update status before canceling
	process.UpdateStatus(StatusTransitioning, nil)
	process.Cancel()
	return nil
}

// GetStatus returns the current status of a process
func (pm *ProcessManager) GetStatus(name string) (*ProcessStatus, error) {
	pm.mu.RLock()
	process, exists := pm.processes[name]
	pm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("process not found")
	}

	status := process.GetStatus()
	return &status, nil
}

// GetAllStatuses returns the status of all processes
func (pm *ProcessManager) GetAllStatuses() map[string]ProcessStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	statuses := make(map[string]ProcessStatus, len(pm.processes))
	for name, process := range pm.processes {
		statuses[name] = process.GetStatus()
	}
	return statuses
}

func (pm *ProcessManager) GetProcessLogs(name string) ([]string, error) {

	pm.mu.RLock()
	process, exists := pm.processes[name]
	pm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("process not found")
	}

	// if logfile exists read full logs
	if process.logfilePath != "" {

		file, err := os.Open(process.logfilePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		var logs []string

		scanner := bufio.NewScanner(file)

		buf := make([]byte, 0, 1024*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			logs = append(logs, scanner.Text())
		}

		return logs, scanner.Err()
	}

	// fallback to memory logs
	process.mu.RLock()
	defer process.mu.RUnlock()

	var logs []string

	for _, l := range process.logs {
		logs = append(logs, l.String())
	}

	return logs, nil
}

func (pm *ProcessManager) StopAllProcesses() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var wg sync.WaitGroup

	for _, process := range pm.processes {
		wg.Add(1)
		go func(p *ControlledProcess) {
			defer wg.Done()
			p.UpdateStatus(StatusTransitioning, nil)
			p.Cancel()
		}(process)
	}

	wg.Wait()
}
