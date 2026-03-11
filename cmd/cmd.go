// Package cmd provides functionality for managing external processes.
// It allows you to start, stop, monitor and capture logs from long running processes.
// This package is designed for applications that need to control and monitor multiple
// external commands or processes simultaneously.
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
	// LogTypeStdErr identifies log entries that come from the standard error stream of a process.
	// This constant is used to tag logs so you can distinguish error output from normal output.
	LogTypeStdErr = "stderr"

	// LogTypeStdOut identifies log entries that come from the standard output stream of a process.
	// This constant is used to tag logs so you can distinguish normal output from error output.
	LogTypeStdOut = "stdout"

	// maxMemoryLogs defines the maximum number of log entries kept in memory for each process.
	// When this limit is reached, older logs are removed to prevent unbounded memory growth.
	// The value of 100 provides a reasonable recent history without consuming too much memory.
	maxMemoryLogs = 100

	// StatusTransitioning indicates that a process state change has been requested but not yet completed.
	// This happens when a start or stop command is issued but the process has not yet reached its new state.
	// For example, when starting a process, it transitions from stopped to running.
	StatusTransitioning = "transitioning"

	// StatusRunning indicates that a process is currently executing.
	// The process has been successfully started and is actively running.
	StatusRunning = "running"

	// StatusStopped indicates that a process is not running.
	// This could be because the user stopped it manually or because it exited on its own.
	StatusStopped = "stopped"
)

// ProcessLog represents a single log entry from a running process.
// It captures output from either the standard output or standard error stream.
// Each log entry includes a timestamp to track when the output was generated.
type ProcessLog struct {
	// Type identifies whether this log came from stdout or stderr.
	// Valid values are LogTypeStdOut or LogTypeStdErr.
	Type string `json:"type"`

	// ProcessName is the name of the process that generated this log.
	// This helps identify which process produced the output when managing multiple processes.
	ProcessName string `json:"process_name"`

	// Log contains the actual log message text from the process output.
	// This is a single line of output from the process.
	Log string `json:"log"`

	// Timestamp is the Unix timestamp in milliseconds when this log was captured.
	// This allows chronological ordering and time-based filtering of logs.
	Timestamp int64 `json:"timestamp"`
}

// ProcessStatus holds the current state information of a process.
// It provides details about whether the process is running, when it started, and how it ended.
// This information is useful for monitoring and debugging process behavior.
type ProcessStatus struct {
	// Name is the unique identifier for this process.
	// This name is used to reference the process in commands and queries.
	Name string `json:"name"`

	// Pid is the process ID assigned by the operating system.
	// This is the actual OS-level process identifier used by the system.
	Pid int `json:"pid"`

	// StartTime is the Unix timestamp in milliseconds when the process was started.
	// This helps track how long a process has been running.
	StartTime int64 `json:"start_time"`

	// Status indicates the current state of the process.
	// Possible values are StatusTransitioning, StatusRunning, or StatusStopped.
	Status string `json:"status"`

	// ExitCode contains the process exit code if the process has stopped.
	// It will be nil if the process is still running or transitioning.
	// A zero value typically indicates successful completion, while non-zero indicates an error.
	ExitCode *int `json:"exit_code,omitempty"`
}

// String converts a ProcessLog into a human-readable formatted string.
// The format includes the timestamp in RFC3339 format, the log type, and the log message.
// This makes it easy to read and understand log entries when printing or saving them.
// Returns a string in the format: [timestamp] type: message
func (l *ProcessLog) String() string {
	return fmt.Sprintf("[%s] %s: %s",
		time.UnixMilli(l.Timestamp).Format(time.RFC3339),
		l.Type,
		l.Log)
}

// ControlledProcess represents a managed external process with monitoring capabilities.
// It provides complete lifecycle management including starting, stopping, and capturing output.
// Each process can have callbacks for real-time log streaming and status change notifications.
type ControlledProcess struct {
	// Name is the unique identifier assigned to this process.
	// This name is used to reference the process for operations like stop, status check, or log retrieval.
	Name string

	// CMD is the underlying OS command being executed.
	// This holds the actual process handle and command details from the exec package.
	CMD *exec.Cmd

	// Cancel is a function that can be called to stop the process.
	// It sends a cancellation signal to the process context, triggering a graceful shutdown.
	Cancel context.CancelFunc

	// Status contains the current state information of the process.
	// This includes whether it is running, stopped, or transitioning, along with PID and timing information.
	Status ProcessStatus

	// workDir is the directory where the process should execute.
	// This sets the current working directory for the process, similar to running cd before a command.
	workDir string

	// logfilePath is the file path where logs should be written to disk.
	// If empty, logs are only kept in memory up to maxMemoryLogs entries.
	logfilePath string

	// logfile is the open file handle for writing logs to disk.
	// This is nil if no log file path was specified or if the file could not be opened.
	logfile *os.File

	// logs holds recent log entries in memory for quick access.
	// This circular buffer is limited to maxMemoryLogs entries to prevent unbounded growth.
	logs []ProcessLog

	// mu protects concurrent access to Status, logs, and logfile fields.
	// This mutex ensures thread-safe reads and writes when multiple goroutines access these fields.
	mu sync.RWMutex

	// onLog is a callback function invoked whenever the process produces output.
	// This allows real-time streaming of logs to external handlers or notification systems.
	onLog func(ProcessLog)

	// onStatusChange is a callback function invoked whenever the process status changes.
	// This notifies external systems when a process starts, stops, or transitions between states.
	onStatusChange func(status ProcessStatus)
}

// UpdateStatus changes the current status of the process and triggers status change callbacks.
// This method is thread-safe and ensures the callback is invoked outside of the lock to prevent deadlocks.
// Parameters:
//   - status: the new status value (StatusTransitioning, StatusRunning, or StatusStopped)
//   - exitCode: the process exit code if stopped, otherwise nil
//
// The onStatusChange callback is invoked with a copy of the updated status if it was set.
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

// GetStatus returns a copy of the current process status in a thread-safe manner.
// This method acquires a read lock to safely access the status information.
// Returns a ProcessStatus struct containing name, PID, start time, status, and exit code.
func (cp *ControlledProcess) GetStatus() ProcessStatus {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.Status
}

// ProcessManager manages multiple controlled processes concurrently.
// It provides a central place to start, stop, and monitor multiple external processes.
// All operations are thread-safe and can be called from multiple goroutines simultaneously.
type ProcessManager struct {
	// processes is a map of process names to their ControlledProcess instances.
	// This allows quick lookup and management of processes by name.
	processes map[string]*ControlledProcess

	// mu protects concurrent access to the processes map.
	// This ensures thread-safe operations when adding, removing, or accessing processes.
	mu sync.RWMutex

	// ctx is the parent context for all managed processes.
	// When this context is cancelled, all child processes will be signaled to stop.
	ctx context.Context
}

// NewProcessManager creates a new ProcessManager instance for managing multiple processes.
// The provided context is used as the parent context for all processes managed by this instance.
// When the context is cancelled, all running processes will receive cancellation signals.
// Parameters:
//   - ctx: the parent context that controls the lifecycle of all managed processes
//
// Returns a pointer to a new ProcessManager ready to manage processes.
func NewProcessManager(ctx context.Context) *ProcessManager {
	return &ProcessManager{
		processes: make(map[string]*ControlledProcess),
		ctx:       ctx,
	}
}

// ProcessRequest contains all the information needed to start a new process.
// It specifies the command to run, its arguments, working directory, and optional callbacks.
type ProcessRequest struct {
	// Name is a unique identifier for the process.
	// This name must be unique among all processes managed by the ProcessManager.
	Name string `json:"name"`

	// Command is the executable or command to run.
	// This can be a binary path or a command available in the system PATH.
	Command string `json:"command"`

	// Args contains the command-line arguments to pass to the command.
	// Each argument should be a separate string in the slice.
	Args []string `json:"args,omitempty"`

	// WorkDir is the working directory where the process should execute.
	// If empty, the process will use the current working directory of the parent process.
	WorkDir string `json:"work_dir,omitempty"`

	// LogfilePath is the path where logs should be written to disk.
	// If empty, logs are only kept in memory up to maxMemoryLogs entries.
	LogfilePath string `json:"logfile_path,omitempty"`

	// onLog is a callback function called whenever the process produces output.
	// This is not exported via JSON and must be set programmatically using SetOnLog.
	onLog func(ProcessLog)

	// onStatusChange is a callback function called when the process status changes.
	// This is not exported via JSON and must be set programmatically using SetOnStatusChange.
	onStatusChange func(ProcessStatus)
}

// SetOnLog assigns a callback function that will be invoked for each log entry from the process.
// This enables real-time streaming of process output to external handlers.
// Parameters:
//   - callback: a function that receives each ProcessLog as the process generates output
func (pr *ProcessRequest) SetOnLog(callback func(ProcessLog)) {
	pr.onLog = callback
}

// SetOnStatusChange assigns a callback function that will be invoked when the process status changes.
// This enables monitoring of process lifecycle events like starting, running, and stopping.
// Parameters:
//   - callback: a function that receives the updated ProcessStatus whenever it changes
func (pr *ProcessRequest) SetOnStatusChange(callback func(ProcessStatus)) {
	pr.onStatusChange = callback
}

// StartProcess creates and starts a new process based on the provided request.
// It sets up output pipes, log capturing, and process monitoring.
// Three goroutines are started for this process:
//   - One for capturing stdout: runs in a goroutine to avoid blocking while reading output
//   - One for capturing stderr: runs in a goroutine to handle error output separately
//   - One for monitoring exit: runs in a goroutine to detect when the process terminates
//
// The goroutines are necessary because reading from pipes is blocking, and we need to
// capture both stdout and stderr simultaneously while also waiting for process completion.
// Parameters:
//   - req: the ProcessRequest containing all configuration for the new process
//
// Returns an error if the process name already exists or if starting the process fails.
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

// waitForExit monitors a process and handles cleanup when it exits.
// This function runs in a goroutine because it blocks waiting for the process to complete.
// Running it in a goroutine allows the process to be started without blocking the caller.
// It closes log files, determines the exit code, updates the status, and removes the process from the manager.
// The exit code is set based on how the process terminated:
//   - 0 for successful completion
//   - The actual exit code if the process failed
//   - -1 for termination by signal or other non-exit errors
//
// Parameters:
//   - process: the ControlledProcess to monitor
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

// captureLogs reads output from a process pipe and stores it as log entries.
// This function runs in a goroutine because reading from the pipe is blocking.
// Running it in a goroutine allows continuous log capture without blocking other operations.
// It reads line by line, creates ProcessLog entries, stores them in memory, writes to file if configured,
// and invokes the onLog callback for each line.
// The buffer size is set to 1MB to handle large log lines that exceed the default 64KB scanner limit.
// Parameters:
//   - process: the ControlledProcess whose logs are being captured
//   - pipe: the io.Reader connected to stdout or stderr of the process
//   - logType: either LogTypeStdOut or LogTypeStdErr to identify the source
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

// StopProcess terminates a running process by name.
// It changes the status to transitioning and then cancels the process context.
// The actual cleanup happens asynchronously in the waitForExit goroutine.
// Parameters:
//   - name: the unique name of the process to stop
//
// Returns an error if no process with that name exists.
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

// GetStatus retrieves the current status of a specific process by name.
// It returns a copy of the status to prevent external modification.
// Parameters:
//   - name: the unique name of the process to query
//
// Returns the ProcessStatus and nil error if found, or nil and an error if the process does not exist.
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

// GetAllStatuses retrieves the current status of all managed processes.
// It creates a map where each key is a process name and each value is its status.
// Returns a map of process names to their current ProcessStatus structs.
func (pm *ProcessManager) GetAllStatuses() map[string]ProcessStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	statuses := make(map[string]ProcessStatus, len(pm.processes))
	for name, process := range pm.processes {
		statuses[name] = process.GetStatus()
	}
	return statuses
}

// GetProcessLogs retrieves all available logs for a specific process.
// If a log file path was configured, it reads the complete log history from the file.
// Otherwise, it returns the recent logs stored in memory (up to maxMemoryLogs entries).
// The buffer size is set to 1MB to handle large log lines when reading from file.
// Parameters:
//   - name: the unique name of the process whose logs to retrieve
//
// Returns a slice of formatted log strings and nil error if successful, or nil and an error if the process is not found.
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

// StopAllProcesses gracefully stops all currently managed processes.
// Each process is stopped in its own goroutine to parallelize the shutdown process.
// Running each stop operation in a goroutine allows all processes to be signaled simultaneously
// instead of waiting for each one to stop before signaling the next.
// The function waits for all stop operations to complete before returning.
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
