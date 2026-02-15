package zram

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type ZRAM struct {
	Device string
}

func NewZRAM(device string) (*ZRAM, error) {
	if device == "" {
		return nil, fmt.Errorf("zram: device must not be empty")
	}

	zram := new(ZRAM)
	zram.Device = device

	if _, err := os.Stat(zram.GetPathDevBlock()); err != nil {
		return nil, fmt.Errorf("zram: device %s not found", device)
	}
	if _, err := os.Stat(zram.GetPathSysfs()); err != nil {
		return nil, fmt.Errorf("zram: sysfs block for device %s not found", zram.GetPathSysfs())
	}
	if _, err := os.Stat(zram.GetPathDiskSize()); err != nil {
		return nil, fmt.Errorf("zram: disk size for device %s not found", zram.GetPathDiskSize())
	}
	if _, err := os.Stat(zram.GetPathReset()); err != nil {
		return nil, fmt.Errorf("zram: reset control for device %s not found", zram.GetPathReset())
	}
	if _, err := os.Stat(zram.GetPathCompAlgo()); err != nil {
		return nil, fmt.Errorf("zram: compression algorithms for device %s not found", zram.GetPathCompAlgo())
	}

	return zram, nil
}

func (zram *ZRAM) Enable() error {
	_, _ = run("mkswap", zram.GetPathDevBlock())
	_, _ = run("swapon", zram.GetPathDevBlock())
	return nil
}
func (zram *ZRAM) Disable() error {
	_, _ = run("swapoff", zram.GetPathDevBlock())
	return zram.Reset()
}
func (zram *ZRAM) Reset() error {
	if err := os.WriteFile(zram.GetPathReset(), []byte("1"), 0644); err != nil {
		return fmt.Errorf("zram: failed to reset zram device: %w", err)
	}
	return nil
}

func (zram *ZRAM) GetSizeBytes() (uint64, error) {
	data, err := os.ReadFile(zram.GetPathDiskSize())
	if err != nil {
		return 0, fmt.Errorf("zram: failed to get disk size: %w", err)
	}
	var size uint64
	if _, err := fmt.Sscanf(string(data), "%d", &size); err != nil {
		return 0, fmt.Errorf("zram: failed to parse disk size: %w", err)
	}
	return size, nil
}
func (zram *ZRAM) SetSizeBytes(size uint64) error {
	if size < 0 {
		return fmt.Errorf("zram: invalid disk size")
	}
	if size == 0 {
		return nil
	}
	if err := os.WriteFile(zram.GetPathDiskSize(), []byte(fmt.Sprintf("%d", size)), 0644); err != nil {
		return fmt.Errorf("zram: failed to set disk size: %w", err)
	}
	return nil
}

func (zram *ZRAM) getCompAlgos() ([]string, error) {
	data, err := os.ReadFile(zram.GetPathCompAlgo())
	if err != nil {
		return nil, fmt.Errorf("zram: failed to get compression algorithms: %w", err)
	}
	algos := strings.Split(string(data), " ")
	if len(algos) == 0 {
		return nil, fmt.Errorf("zram: no compression algorithms found")
	}
	algos = algos[:len(algos)-1]
	return algos, nil
}
func (zram *ZRAM) GetCompAlgos() ([]string, error) {
	algos, err := zram.getCompAlgos()
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(algos); i++ {
		if algos[i][0] == '[' {
			algos[i] = algos[i][1 : len(algos[i])-1]
		}
	}
	return algos, nil
}
func (zram *ZRAM) GetCompAlgo() (string, error) {
	algos, err := zram.getCompAlgos()
	if err != nil {
		return "", err
	}
	for i := 0; i < len(algos); i++ {
		if algos[i][0] == '[' {
			algo := algos[i][1 : len(algos[i])-1]
			return algo, nil
		}
	}
	return "", fmt.Errorf("zram: no compression algorithm selected")
}
func (zram *ZRAM) SetCompAlgo(algo string) error {
	if err := os.WriteFile(zram.GetPathCompAlgo(), []byte(algo), 0644); err != nil {
		return fmt.Errorf("zram: failed to set compression algorithm: %w", err)
	}
	return nil
}

func (zram *ZRAM) GetPathDevBlock() string {
	return "/dev/block/" + zram.Device
}
func (zram *ZRAM) GetPathSysfs() string {
	return "/sys/block/" + zram.Device
}
func (zram *ZRAM) GetPathDiskSize() string {
	return zram.GetPathSysfs() + "/disksize"
}
func (zram *ZRAM) GetPathReset() string {
	return zram.GetPathSysfs() + "/reset"
}
func (zram *ZRAM) GetPathCompAlgo() string {
	return zram.GetPathSysfs() + "/comp_algorithm"
}

func run(args ...string) (string, error) {
	process := exec.Command(args[0], args[1:]...)
	stdoutPipe, _ := process.StdoutPipe()
	stderrPipe, _ := process.StderrPipe()
	if err := process.Run(); err != nil {
		return "", err
	}
	stdout, _ := io.ReadAll(stdoutPipe)
	stderr, _ := io.ReadAll(stderrPipe)

	if len(stderr) > 0 {
		return string(stdout), fmt.Errorf("%s", string(stderr))
	}
	return string(stdout), nil
}
