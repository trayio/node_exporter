package collector

import (
	"strings"
	"testing"
)

var memoryStatsData = `
cache 3293184
rss 487424
rss_huge 0
mapped_file 2732032
dirty 0
writeback 0
swap 0
pgpgin 2694
pgpgout 1771
pgfault 2367
pgmajfault 32
inactive_anon 0
active_anon 487424
inactive_file 0
active_file 3293184
unevictable 0
hierarchical_memory_limit 9223372036854771712
hierarchical_memsw_limit 9223372036854771712
total_cache 3293184
total_rss 487424
total_rss_huge 0
total_mapped_file 2732032
total_dirty 0
total_writeback 0
total_swap 0
total_pgpgin 2694
total_pgpgout 1771
total_pgfault 2367
total_pgmajfault 32
total_inactive_anon 0
total_active_anon 487424
total_inactive_file 0
total_active_file 3293184
total_unevictable 0
recent_rotated_anon 1890
recent_rotated_file 804
recent_scanned_anon 1890
recent_scanned_file 804
`

var cpuStatsData = `
61239221
`

func TestParseMemoryInfo(t *testing.T) {
	var (
		requiredFields = []string{"rss", "cache"}
		out            memoryStats
		err            error
	)

	data := strings.NewReader(memoryStatsData)
	out, err = parseMemoryInfo(data)
	if err != nil {
		t.Fatalf("failed to parse memory stats: %s\n", err)
	}

	for _, field := range requiredFields {
		if _, ok := out[field]; !ok {
			t.Fatalf("missing %s entry", field)
		}
	}
}

func TestParseCpuInfo(t *testing.T) {
	var (
		out float64
		err error
	)

	data := strings.NewReader(cpuStatsData)
	out, err = parseCpuInfo(data)
	if err != nil {
		t.Fatalf("failed to parse cpu stats: %s\n", err)
	}

	if out != 61239221 {
		t.Fatalf("wrong value for cpu stats\n")
	}
}
