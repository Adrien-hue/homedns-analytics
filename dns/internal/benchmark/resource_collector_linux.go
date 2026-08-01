//go:build linux

package benchmark

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	procRoot = "/proc"

	thermalPath = "/sys/class/thermal/thermal_zone0/temp"
)

// LinuxResourceCollector collects process and host metrics from Linux procfs,
// sysfs, and Raspberry Pi utilities.
type LinuxResourceCollector struct {
	now func() time.Time

	readFile func(string) ([]byte, error)

	runCommand func(
		context.Context,
		string,
		...string,
	) ([]byte, error)
}

// NewResourceCollector creates the production resource collector for Linux.
func NewResourceCollector() ResourceCollector {
	return &LinuxResourceCollector{
		now: time.Now,

		readFile: os.ReadFile,

		runCommand: func(
			ctx context.Context,
			name string,
			args ...string,
		) ([]byte, error) {
			return exec.CommandContext(
				ctx,
				name,
				args...,
			).Output()
		},
	}
}

// Collect executes the benchmark while periodically collecting Linux process
// and host resource metrics.
func (c *LinuxResourceCollector) Collect(
	ctx context.Context,
	config ResourceCollectionConfig,
	run func(context.Context) error,
) (ResourceCollectionResult, error) {
	if c == nil {
		return ResourceCollectionResult{},
			errors.New(
				"Linux resource collector is not initialized",
			)
	}

	if ctx == nil {
		return ResourceCollectionResult{},
			errors.New(
				"resource collection context is required",
			)
	}

	if run == nil {
		return ResourceCollectionResult{},
			errors.New(
				"resource collection run function is required",
			)
	}

	if err := config.Validate(); err != nil {
		return ResourceCollectionResult{},
			err
	}

	if c.now == nil {
		return ResourceCollectionResult{},
			errors.New(
				"resource collector clock is not initialized",
			)
	}

	if c.readFile == nil {
		return ResourceCollectionResult{},
			errors.New(
				"resource collector file reader is not initialized",
			)
	}

	if c.runCommand == nil {
		return ResourceCollectionResult{},
			errors.New(
				"resource collector command runner is not initialized",
			)
	}

	startedAt := c.now().UTC()

	result := ResourceCollectionResult{
		StartedAt: startedAt,

		BenchmarkPIDBefore: config.BenchmarkPID,

		TargetPIDBefore: config.TargetPID,

		Samples: make([]ResourceSample, 0),

		Errors: make([]string, 0),
	}

	var previous processTickSnapshot

	before, initialSnapshot :=
		c.collectSample(
			ctx,
			config,
			previous,
		)

	result.Before = before

	result.Errors = append(
		result.Errors,
		before.Errors...,
	)

	previous = initialSnapshot

	samplingContext, cancelSampling :=
		context.WithCancel(ctx)
	defer cancelSampling()

	var (
		samplesMutex sync.Mutex

		samples = make(
			[]ResourceSample,
			0,
		)

		finalSnapshot = previous
	)

	var waitGroup sync.WaitGroup

	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		ticker := time.NewTicker(
			config.Interval,
		)
		defer ticker.Stop()

		localPrevious := previous

		for {
			select {
			case <-samplingContext.Done():
				return

			case <-ticker.C:
				sample, nextSnapshot :=
					c.collectSample(
						samplingContext,
						config,
						localPrevious,
					)

				localPrevious =
					nextSnapshot

				samplesMutex.Lock()

				samples = append(
					samples,
					sample,
				)

				finalSnapshot =
					nextSnapshot

				samplesMutex.Unlock()
			}
		}
	}()

	runErr := run(ctx)

	cancelSampling()
	waitGroup.Wait()

	samplesMutex.Lock()

	result.Samples = append(
		result.Samples,
		samples...,
	)

	previous = finalSnapshot

	samplesMutex.Unlock()

	for _, sample := range result.Samples {
		result.Errors = append(
			result.Errors,
			sample.Errors...,
		)
	}

	after, _ := c.collectSample(
		context.Background(),
		config,
		previous,
	)

	result.After = after

	result.Errors = append(
		result.Errors,
		after.Errors...,
	)

	result.EndedAt = c.now().UTC()

	result.BenchmarkPIDAfter =
		after.BenchmarkProcess.PID

	result.TargetPIDAfter =
		after.TargetService.PID

	return result, runErr
}

type processTickSnapshot struct {
	SystemTicks uint64

	BenchmarkTicks uint64
	TargetTicks    uint64

	BenchmarkValid bool
	TargetValid    bool
	SystemValid    bool
}

func (c *LinuxResourceCollector) collectSample(
	ctx context.Context,
	config ResourceCollectionConfig,
	previous processTickSnapshot,
) (ResourceSample, processTickSnapshot) {
	sample := ResourceSample{
		CollectedAt: c.now().UTC(),

		Errors: make([]string, 0),
	}

	systemTicks, systemErr :=
		c.readSystemCPUTicks()

	if systemErr != nil {
		sample.Errors = append(
			sample.Errors,
			fmt.Sprintf(
				"read system CPU ticks: %v",
				systemErr,
			),
		)
	}

	benchmarkTicks, benchmarkErr :=
		c.populateProcessSample(
			config.BenchmarkPID,
			&sample.BenchmarkProcess,
		)

	if benchmarkErr != nil {
		sample.Errors = append(
			sample.Errors,
			fmt.Sprintf(
				"read benchmark process %d: %v",
				config.BenchmarkPID,
				benchmarkErr,
			),
		)
	}

	targetTicks, targetErr :=
		c.populateProcessSample(
			config.TargetPID,
			&sample.TargetService,
		)

	if targetErr != nil {
		sample.Errors = append(
			sample.Errors,
			fmt.Sprintf(
				"read target process %d: %v",
				config.TargetPID,
				targetErr,
			),
		)
	}

	current := processTickSnapshot{
		SystemTicks: systemTicks,

		BenchmarkTicks: benchmarkTicks,

		TargetTicks: targetTicks,

		SystemValid: systemErr == nil,

		BenchmarkValid: benchmarkErr == nil,

		TargetValid: targetErr == nil,
	}

	if previous.SystemValid &&
		current.SystemValid &&
		current.SystemTicks >
			previous.SystemTicks {
		systemDelta :=
			current.SystemTicks -
				previous.SystemTicks

		if previous.BenchmarkValid &&
			current.BenchmarkValid &&
			current.BenchmarkTicks >=
				previous.BenchmarkTicks {
			benchmarkDelta :=
				current.BenchmarkTicks -
					previous.BenchmarkTicks

			sample.
				BenchmarkProcess.
				CPUPercent =
				float64Pointer(
					cpuPercentFromTicks(
						benchmarkDelta,
						systemDelta,
					),
				)
		}

		if previous.TargetValid &&
			current.TargetValid &&
			current.TargetTicks >=
				previous.TargetTicks {
			targetDelta :=
				current.TargetTicks -
					previous.TargetTicks

			sample.
				TargetService.
				CPUPercent =
				float64Pointer(
					cpuPercentFromTicks(
						targetDelta,
						systemDelta,
					),
				)
		}
	}

	if value, readErr :=
		c.readLoadAverage(); readErr != nil {
		sample.Errors = append(
			sample.Errors,
			fmt.Sprintf(
				"read load average: %v",
				readErr,
			),
		)
	} else {
		sample.System.
			LoadAverageOneMinute =
			float64Pointer(value)
	}

	if value, readErr :=
		c.readAvailableMemory(); readErr != nil {
		sample.Errors = append(
			sample.Errors,
			fmt.Sprintf(
				"read available memory: %v",
				readErr,
			),
		)
	} else {
		sample.System.
			MemoryAvailableBytes =
			uint64Pointer(value)
	}

	if value, readErr :=
		c.readTemperature(); readErr != nil {
		sample.Errors = append(
			sample.Errors,
			fmt.Sprintf(
				"read temperature: %v",
				readErr,
			),
		)
	} else {
		sample.System.
			TemperatureCelsius =
			float64Pointer(value)
	}

	if value, readErr :=
		c.readThrottledState(ctx); readErr != nil {
		sample.Errors = append(
			sample.Errors,
			fmt.Sprintf(
				"read throttling state: %v",
				readErr,
			),
		)
	} else {
		sample.System.
			ThrottledState =
			stringPointer(value)
	}

	return sample, current
}

func (c *LinuxResourceCollector) populateProcessSample(
	pid int,
	sample *ProcessResourceSample,
) (uint64, error) {
	if sample == nil {
		return 0,
			errors.New(
				"process sample is required",
			)
	}

	stat, err := c.readProcessStat(pid)
	if err != nil {
		return 0, err
	}

	status, err :=
		c.readProcessStatus(pid)
	if err != nil {
		return 0, err
	}

	sample.PID = pid

	sample.RSSBytes =
		uint64Pointer(
			status.RSSBytes,
		)

	sample.Threads =
		uint64Pointer(
			status.Threads,
		)

	return stat.UserTicks +
		stat.SystemTicks, nil
}

type processStat struct {
	UserTicks   uint64
	SystemTicks uint64
}

func (c *LinuxResourceCollector) readProcessStat(
	pid int,
) (processStat, error) {
	path := filepath.Join(
		procRoot,
		strconv.Itoa(pid),
		"stat",
	)

	content, err := c.readFile(path)
	if err != nil {
		return processStat{}, err
	}

	text := string(content)

	closingParenthesis :=
		strings.LastIndex(text, ")")

	if closingParenthesis < 0 {
		return processStat{},
			errors.New(
				"malformed process stat",
			)
	}

	fields := strings.Fields(
		text[closingParenthesis+1:],
	)

	// The fields after the process command begin with field 3, the process
	// state. utime and stime are fields 14 and 15, making their indexes 11 and
	// 12 in this sliced representation.
	if len(fields) <= 12 {
		return processStat{},
			errors.New(
				"process stat has insufficient fields",
			)
	}

	userTicks, err := strconv.ParseUint(
		fields[11],
		10,
		64,
	)
	if err != nil {
		return processStat{},
			fmt.Errorf(
				"parse user CPU ticks: %w",
				err,
			)
	}

	systemTicks, err := strconv.ParseUint(
		fields[12],
		10,
		64,
	)
	if err != nil {
		return processStat{},
			fmt.Errorf(
				"parse system CPU ticks: %w",
				err,
			)
	}

	return processStat{
		UserTicks:   userTicks,
		SystemTicks: systemTicks,
	}, nil
}

type processStatus struct {
	RSSBytes uint64
	Threads  uint64
}

func (c *LinuxResourceCollector) readProcessStatus(
	pid int,
) (processStatus, error) {
	path := filepath.Join(
		procRoot,
		strconv.Itoa(pid),
		"status",
	)

	content, err := c.readFile(path)
	if err != nil {
		return processStatus{}, err
	}

	var (
		status processStatus

		foundRSS     bool
		foundThreads bool
	)

	scanner := bufio.NewScanner(
		strings.NewReader(
			string(content),
		),
	)

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(
			line,
			"VmRSS:",
		):
			value, parseErr :=
				parseProcKilobytes(line)
			if parseErr != nil {
				return processStatus{},
					parseErr
			}

			status.RSSBytes =
				value * 1024

			foundRSS = true

		case strings.HasPrefix(
			line,
			"Threads:",
		):
			fields :=
				strings.Fields(line)

			if len(fields) != 2 {
				return processStatus{},
					errors.New(
						"malformed Threads field",
					)
			}

			value, parseErr :=
				strconv.ParseUint(
					fields[1],
					10,
					64,
				)
			if parseErr != nil {
				return processStatus{},
					fmt.Errorf(
						"parse thread count: %w",
						parseErr,
					)
			}

			status.Threads = value
			foundThreads = true
		}
	}

	if err := scanner.Err(); err != nil {
		return processStatus{}, err
	}

	if !foundRSS {
		return processStatus{},
			errors.New(
				"VmRSS is missing",
			)
	}

	if !foundThreads {
		return processStatus{},
			errors.New(
				"Threads is missing",
			)
	}

	return status, nil
}

func (c *LinuxResourceCollector) readSystemCPUTicks() (
	uint64,
	error,
) {
	content, err := c.readFile(
		filepath.Join(
			procRoot,
			"stat",
		),
	)
	if err != nil {
		return 0, err
	}

	line, _, _ := strings.Cut(
		string(content),
		"\n",
	)

	fields := strings.Fields(line)

	if len(fields) < 2 ||
		fields[0] != "cpu" {
		return 0,
			errors.New(
				"malformed aggregate CPU stat",
			)
	}

	var total uint64

	for _, field := range fields[1:] {
		value, parseErr :=
			strconv.ParseUint(
				field,
				10,
				64,
			)
		if parseErr != nil {
			return 0,
				fmt.Errorf(
					"parse aggregate CPU ticks: %w",
					parseErr,
				)
		}

		total += value
	}

	return total, nil
}

func (c *LinuxResourceCollector) readLoadAverage() (
	float64,
	error,
) {
	content, err := c.readFile(
		filepath.Join(
			procRoot,
			"loadavg",
		),
	)
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(
		string(content),
	)

	if len(fields) == 0 {
		return 0,
			errors.New(
				"load average is empty",
			)
	}

	value, err := strconv.ParseFloat(
		fields[0],
		64,
	)
	if err != nil {
		return 0,
			fmt.Errorf(
				"parse load average: %w",
				err,
			)
	}

	return value, nil
}

func (c *LinuxResourceCollector) readAvailableMemory() (
	uint64,
	error,
) {
	content, err := c.readFile(
		filepath.Join(
			procRoot,
			"meminfo",
		),
	)
	if err != nil {
		return 0, err
	}

	scanner := bufio.NewScanner(
		strings.NewReader(
			string(content),
		),
	)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(
			line,
			"MemAvailable:",
		) {
			continue
		}

		kilobytes, parseErr :=
			parseProcKilobytes(line)
		if parseErr != nil {
			return 0, parseErr
		}

		return kilobytes * 1024, nil
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return 0,
		errors.New(
			"MemAvailable is missing",
		)
}

func (c *LinuxResourceCollector) readTemperature() (
	float64,
	error,
) {
	content, err := c.readFile(
		thermalPath,
	)
	if err != nil {
		return 0, err
	}

	milliCelsius, err :=
		strconv.ParseFloat(
			strings.TrimSpace(
				string(content),
			),
			64,
		)
	if err != nil {
		return 0,
			fmt.Errorf(
				"parse thermal value: %w",
				err,
			)
	}

	return milliCelsius / 1000, nil
}

func (c *LinuxResourceCollector) readThrottledState(
	ctx context.Context,
) (string, error) {
	output, err := c.runCommand(
		ctx,
		"vcgencmd",
		"get_throttled",
	)
	if err != nil {
		return "", err
	}

	value := strings.TrimSpace(
		string(output),
	)

	if value == "" {
		return "",
			errors.New(
				"empty throttling state",
			)
	}

	return value, nil
}

func parseProcKilobytes(
	line string,
) (uint64, error) {
	fields := strings.Fields(line)

	if len(fields) < 2 {
		return 0,
			errors.New(
				"malformed procfs memory field",
			)
	}

	value, err := strconv.ParseUint(
		fields[1],
		10,
		64,
	)
	if err != nil {
		return 0,
			fmt.Errorf(
				"parse procfs memory value: %w",
				err,
			)
	}

	return value, nil
}

func cpuPercentFromTicks(
	processDelta uint64,
	systemDelta uint64,
) float64 {
	if systemDelta == 0 {
		return 0
	}

	return round(
		float64(processDelta)/
			float64(systemDelta)*
			100,
		6,
	)
}
