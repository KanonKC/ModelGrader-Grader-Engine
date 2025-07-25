package services_sandbox

import (
	"ModelGrader-Grader/types"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type SandboxService interface {
	Init() error
	FindAvailableSandbox() (int, error)
	ReleaseSandbox(id int) error
	MakeBusy(id int) error
	WriteInput(id int, content string, index int) error
	WriteCode(id int, lang types.ProgrammingLanguage, content string) error
	RunCode(id int, lang types.ProgrammingLanguage) (*RuntimeResult, error)
	RunCodePython(id int, timeout time.Duration) (*RuntimeResult, error)
}

type sandboxService struct {
	size        int
	statusArray []types.StatusArray
}

func (s *sandboxService) Init() error {
	s.statusArray = make([]types.StatusArray, s.size)
	for i := range s.statusArray {
		s.statusArray[i] = types.Available
		err := os.MkdirAll(fmt.Sprintf("./tmp/sandbox/%d", i), 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *sandboxService) FindAvailableSandbox() (int, error) {
	for i, status := range s.statusArray {
		if status == types.Available {
			return i, nil
		}
	}
	return -1, nil
}

func (s *sandboxService) ReleaseSandbox(id int) error {
	if s.statusArray[id] != types.Busy {
		return errors.New("sandbox is not busy")
	}

	inputsDir := fmt.Sprintf("./tmp/sandbox/%d/inputs", id)
	if err := os.RemoveAll(inputsDir); err != nil {
		return errors.New("failed to delete inputs directory")
	}

	s.statusArray[id] = types.Available
	return nil
}

func (s *sandboxService) MakeBusy(id int) error {
	if s.statusArray[id] != types.Available {
		return errors.New("sandbox is not available")
	}
	s.statusArray[id] = types.Busy
	return nil
}

func (s *sandboxService) WriteInput(id int, content string, index int) error {

	filename := fmt.Sprintf("%d", index)

	inputsDir := fmt.Sprintf("./tmp/sandbox/%d/inputs", id)
	if err := os.MkdirAll(inputsDir, 0755); err != nil {
		return fmt.Errorf("failed to create inputs directory: %w", err)
	}

	file, err := os.Create(fmt.Sprintf("%s/%s", inputsDir, filename))
	if err != nil {
		return err
	}
	defer file.Close()

	file.WriteString(content)

	return nil
}

func (s *sandboxService) WriteCode(id int, lang types.ProgrammingLanguage, content string) error {
	var fileType string
	switch lang {
	case types.Python:
		fileType = "py"
	case types.C:
		fileType = "c"
	case types.CPP:
		fileType = "cpp"
	}

	if fileType == "" {
		return errors.New("unsupported language")
	}

	filename := "main." + fileType
	file, err := os.Create(fmt.Sprintf("./tmp/sandbox/%d/%s", id, filename))
	if err != nil {
		return err
	}

	defer file.Close()

	file.WriteString(content)

	return nil
}

func (s *sandboxService) RunCode(id int, lang types.ProgrammingLanguage) (*RuntimeResult, error) {
	switch lang {
	case types.Python:
		return s.RunCodePython(id, 1*time.Second)
	case types.C, types.CPP:
		return nil, errors.New("C/C++ execution not implemented yet")
	default:
		return nil, errors.New("unsupported language")
	}
}

func (s *sandboxService) RunCodePython(id int, timeout time.Duration) (*RuntimeResult, error) {
	// Check if inputs directory exists
	inputsDir := fmt.Sprintf("./tmp/sandbox/%d/inputs", id)
	if _, err := os.Stat(inputsDir); os.IsNotExist(err) {
		// If inputs directory doesn't exist, return empty result
		return &RuntimeResult{
			IsError:          false,
			IsTimeout:        false,
			IsMemoryExceeded: false,
			Output:           []RuntimeOutput{},
		}, nil
	}

	// Find all files in the sandbox directory
	inputFiles, err := os.ReadDir(inputsDir)
	if err != nil {
		return nil, err
	}

	var runtimeOutputs []RuntimeOutput
	for index, input := range inputFiles {
		// Execute the Python file in the sandbox directory with timeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "python3", "main.py")
		cmd.Dir = fmt.Sprintf("./tmp/sandbox/%d", id)

		inputData, err := os.ReadFile(fmt.Sprintf("./tmp/sandbox/%d/inputs/"+input.Name(), id))
		if err != nil {
			return nil, err
		}

		// Set input data as stdin
		cmd.Stdin = strings.NewReader(string(inputData))

		// Measure execution time
		startTime := time.Now()

		// Get combined output (stdout and stderr) to capture error messages
		output, err := cmd.CombinedOutput()

		executionTime := time.Since(startTime)
		executionTimeMs := int(executionTime.Milliseconds())

		outputContent := ""
		errorMessage := ""
		isTimeout := false

		if err != nil {
			// Check if it's a timeout error
			if ctx.Err() == context.DeadlineExceeded {
				isTimeout = true
			} else {
				split := strings.Split(string(output), "\n")
				if len(split) > 2 {
					errorMessage = split[len(split)-2]
				}
			}
		} else {
			outputContent = string(output)
		}

		runtimeOutput := &RuntimeOutput{
			IsError:          err != nil,
			IsTimeout:        isTimeout,
			IsMemoryExceeded: false,
			InputIndex:       index,
			InputContent:     string(inputData), // TODO: read from input file
			OutputContent:    outputContent,
			ExecutionTimeMs:  executionTimeMs,
			MemoryUsageKB:    0, // TODO: measure memory usage
			Error:            errorMessage,
		}
		runtimeOutputs = append(runtimeOutputs, *runtimeOutput)
	}

	return &RuntimeResult{
		IsError:          err != nil,
		IsTimeout:        false,
		IsMemoryExceeded: false,
		Output:           runtimeOutputs,
	}, nil
}

func New(size int) SandboxService {
	return &sandboxService{size: size}
}
