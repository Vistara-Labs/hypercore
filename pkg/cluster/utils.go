package cluster

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

// Returns available memory (in kB)
func getAvailableMem() (uint64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		split := strings.Split(scanner.Text(), " ")
		// MemAvailable:    2303952 kB
		if len(split) > 1 && split[0] == "MemAvailable:" {
			return strconv.ParseUint(split[len(split)-2], 10, 0)
		}
	}

	return 0, errors.New("could not find MemAvailable section")
}
