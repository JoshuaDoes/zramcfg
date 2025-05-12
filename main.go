package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/pflag"
)

var (
	block    = "zram0"
	enable   bool
	disable  bool
	size     int64
	compAlgo string
)

func main() {
	pflag.CommandLine.SortFlags = false
	pflag.StringVarP(&block, "block", "b", block, "ZRAM block device to use")
	pflag.BoolVarP(&enable, "enable", "e", false, "Enable ZRAM")
	pflag.BoolVarP(&disable, "disable", "d", false, "Disable ZRAM")
	pflag.Int64VarP(&size, "size", "s", 0, "ZRAM size in bytes")
	pflag.StringVarP(&compAlgo, "compalgo", "c", "", "Compression algorithm to use")
	pflag.Parse()

	zram, err := NewZRAM(block)
	if err != nil {
		fmt.Printf("Failed to find ZRAM device %s: %v\n", block, err)
		os.Exit(1)
	}

	if enable && disable {
		fmt.Println("Cannot enable and disable at the same time!")
		os.Exit(5)
	}

	if enable || disable || size > 0 || compAlgo != "" {
		if err := zram.Disable(); err != nil {
			fmt.Printf("Failed to disable ZRAM: %v\n", err)
			os.Exit(6)
		}
	}

	if size > 0 {
		if err := zram.SetSizeBytes(size); err != nil {
			fmt.Printf("Failed to set ZRAM size: %v\n", err)
			os.Exit(7)
		}
	}

	if compAlgo != "" {
		if err := zram.SetCompAlgo(compAlgo); err != nil {
			fmt.Printf("Failed to set ZRAM compression algorithm: %v\n", err)
			os.Exit(8)
		}
	}

	if enable || size > 0 || compAlgo != "" {
		if err := zram.Enable(); err != nil {
			fmt.Printf("Failed to enable ZRAM: %v\n", err)
			os.Exit(9)
		}
	}

	printzram(zram)
}

func printzram(zram *ZRAM) {
	str := fmt.Sprintf("ZRAM config: %s\n", zram.device)
	size, err := zram.GetSizeBytes()
	if err != nil {
		fmt.Printf("Failed to get ZRAM size: %v\n", err)
		os.Exit(2)
	}
	sizeGB := float64(size) / (1024 * 1024 * 1024)
	str += fmt.Sprintf("  Size: %.2fGB (%d bytes)\n", sizeGB, size)
	compAlgo, err := zram.GetCompAlgo()
	if err != nil {
		fmt.Printf("Failed to get ZRAM compression algorithm: %v\n", err)
		os.Exit(3)
	}
	compAlgos, err := zram.GetCompAlgos()
	if err != nil {
		fmt.Printf("Failed to get ZRAM compression algorithms: %v\n", err)
		os.Exit(4)
	}
	str += fmt.Sprintf("  Compressor: %s\n", compAlgo)
	str += "  Compressors:\n"
	for i := 0; i < len(compAlgos); i++ {
		str += fmt.Sprintf("  - %s\n", compAlgos[i])
	}
	fmt.Printf("%s", str)
}

type ZRAM struct {
	device string
}

func NewZRAM(device string) (*ZRAM, error) {
	if device == "" {
		return nil, fmt.Errorf("zram: device must not be empty")
	}

	zram := new(ZRAM)
	zram.device = device

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
	return nil
}

func (zram *ZRAM) GetSizeBytes() (int64, error) {
	data, err := os.ReadFile(zram.GetPathDiskSize())
	if err != nil {
		return 0, fmt.Errorf("zram: failed to get disk size: %w", err)
	}
	var size int64
	if _, err := fmt.Sscanf(string(data), "%d", &size); err != nil {
		return 0, fmt.Errorf("zram: failed to parse disk size: %w", err)
	}
	return size, nil
}
func (zram *ZRAM) SetSizeBytes(size int64) error {
	if size <= 0 {
		return fmt.Errorf("zram: invalid disk size")
	}
	if err := os.WriteFile(zram.GetPathReset(), []byte("1"), 0644); err != nil {
		return fmt.Errorf("zram: failed to reset zram device: %w", err)
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
	return "/dev/block/" + zram.device
}
func (zram *ZRAM) GetPathSysfs() string {
	return "/sys/block/" + zram.device
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
