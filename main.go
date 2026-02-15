package main

import (
	"fmt"
	"os"

	"github.com/JoshuaDoes/zramcfg/zram"
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

	zram, err := zram.NewZRAM(block)
	if err != nil {
		fmt.Printf("Failed to find ZRAM device %s: %v\n", block, err)
		os.Exit(1)
	}

	if enable && disable {
		fmt.Println("Cannot enable and disable at the same time!")
		os.Exit(5)
	}

	if enable || disable || size > 0 || compAlgo != "" {
		fmt.Println("Disabling")
		if err := zram.Disable(); err != nil {
			fmt.Printf("Failed to disable ZRAM: %v\n", err)
			os.Exit(6)
		}
	}

	if compAlgo != "" {
		fmt.Println("Setting compressor:", compAlgo)
		if err := zram.SetCompAlgo(compAlgo); err != nil {
			fmt.Printf("Failed to set ZRAM compression algorithm: %v\n", err)
			os.Exit(8)
		}
	}

	if size > 0 {
		fmt.Println("Setting size:", size)
		if err := zram.SetSizeBytes(size); err != nil {
			fmt.Printf("Failed to set ZRAM size: %v\n", err)
			os.Exit(7)
		}
	}

	if enable || size > 0 || compAlgo != "" {
		fmt.Println("Enabling")
		if err := zram.Enable(); err != nil {
			fmt.Printf("Failed to enable ZRAM: %v\n", err)
			os.Exit(9)
		}
	}

	printzram(zram)
}

func printzram(zram *zram.ZRAM) {
	str := fmt.Sprintf("ZRAM config: %s\n", zram.Device)
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
