package coremetrics

import (
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func readProcessFiles(procSelf string, cpus int) processReading {
	reading := processReading{rssBytes: parseRSS(readBounded(filepath.Join(procSelf, "status"), 64<<10))}
	group := cgroupDirectory(readBounded(filepath.Join(procSelf, "cgroup"), 64<<10), readBounded(filepath.Join(procSelf, "mountinfo"), 1<<20))
	if group != "" {
		reading.cpuLimitCores = parseCPULimit(readBounded(filepath.Join(group, "cpu.max"), 4096), cpus)
		reading.memoryLimitBytes = parseMemoryLimit(readBounded(filepath.Join(group, "memory.max"), 4096))
	}
	return reading
}

// procfs and cgroup values are small. A truncated or failed read is unknown.
func readBounded(path string, limit int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return ""
	}
	return string(data)
}

func parseRSS(status string) *uint64 {
	for _, line := range strings.Split(status, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "VmRSS:" {
			continue
		}
		if len(fields) != 3 || fields[2] != "kB" {
			return nil
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil || value > math.MaxUint64/1024 {
			return nil
		}
		return ptr(value * 1024)
	}
	return nil
}

func parseCPULimit(data string, cpus int) *float64 {
	fields := strings.Fields(data)
	if len(fields) != 2 {
		return nil
	}
	period, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil || period == 0 {
		return nil
	}
	if fields[0] == "max" {
		if cpus > 0 {
			return ptr(float64(cpus))
		}
		return nil
	}
	quota, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil || quota == 0 {
		return nil
	}
	return ptr(float64(quota) / float64(period))
}

func parseMemoryLimit(data string) *uint64 {
	value, err := strconv.ParseUint(strings.TrimSpace(data), 10, 64)
	if err != nil {
		return nil
	}
	return &value
}

// Resolve membership against the mounted cgroup v2 subtree. Do not assume that
// the process belongs to the root or that cgroupfs is mounted at /sys/fs/cgroup.
func cgroupDirectory(membership, mountinfo string) string {
	group := ""
	for _, line := range strings.Split(membership, "\n") {
		if strings.HasPrefix(line, "0::") {
			if group != "" {
				return ""
			}
			group = strings.TrimPrefix(line, "0::")
		}
	}
	if !canonicalAbsolute(group) {
		return ""
	}
	decode := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	selected, rootLength := "", -1
	for _, line := range strings.Split(mountinfo, "\n") {
		left, right, ok := strings.Cut(line, " - ")
		fields, fs := strings.Fields(left), strings.Fields(right)
		if !ok || len(fields) < 6 || len(fs) < 3 || fs[0] != "cgroup2" {
			continue
		}
		root, point := decode.Replace(fields[3]), decode.Replace(fields[4])
		if !canonicalAbsolute(root) || !canonicalAbsolute(point) {
			continue
		}
		relative := ""
		switch {
		case group == root:
		case root == "/":
			relative = strings.TrimPrefix(group, "/")
		case strings.HasPrefix(group, root+"/"):
			relative = strings.TrimPrefix(group, root+"/")
		default:
			continue
		}
		if len(root) > rootLength {
			selected, rootLength = filepath.Join(point, relative), len(root)
		}
	}
	return selected
}

func canonicalAbsolute(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path
}
