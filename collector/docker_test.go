package collector

import (
	"log"
	"strings"
	"testing"
	"time"
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

var networkStatsData = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
veth3741d42:     648       8    0    0    0     0          0         0     4412      30    0    0    0     0       0          0
virbr0-nic:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
enp0s20u1: 816828705  900036    0    0    0     0          0         0 37370902  204804    0    0    0     0       0          0
docker0: 29473964  297207    0    0    0     0          0         0 977390200  289365    0    0    0     0       0          0
virbr0:  274778    3890    0    0    0     0          0         0  8605326    7328    0    0    0     0       0          0
wlp2s0: 2142008307 1973811    0    0    0     0          0         0 220904804 1209362    0    0    0     0       0          0
    lo: 580019245 1087616    0    0    0     0          0         0 580019245 1087616    0    0    0     0       0          0
`

func TestParseContainerMemoryInfo(t *testing.T) {
	var (
		requiredFields = []string{"rss", "cache"}
		out            memoryStatistics
		err            error
	)

	data := strings.NewReader(memoryStatsData)
	out, err = parseContainerMemoryInfo(data)
	if err != nil {
		t.Fatalf("failed to parse memory stats: %s\n", err)
	}

	for _, field := range requiredFields {
		if _, ok := out[field]; !ok {
			t.Fatalf("missing %s entry", field)
		}
	}
}

func TestContainerMemoryUsageBytes(t *testing.T) {
	data := strings.NewReader(memoryStatsData)
	out, err := parseContainerMemoryInfo(data)
	if err != nil {
		t.Fatalf("failed to parse memory stats: %s\n", err)
	}

	if bytes := containerMemoryUsageBytes(out); bytes != 3780608 {
		t.Fatalf("memory usage calculation wrong\n")
	}
}

func TestParseContainerCpuInfo(t *testing.T) {
	var (
		out float64
		err error
	)

	data := strings.NewReader(cpuStatsData)
	out, err = parseContainerCpuInfo(data)
	if err != nil {
		t.Fatalf("failed to parse cpu stats: %s\n", err)
	}

	if out != (time.Duration(61239221) * time.Nanosecond).Seconds() {
		t.Fatalf("wrong value for cpu stats\n")
	}
}

func TestParseContainerNetworkInfo(t *testing.T) {
	var (
		net        network
		interfaces = []string{"veth3741d42", "virbr0-nic", "enp0s20u1", "docker0", "virbr0", "wlp2s0", "lo"}
		topics     = []string{"bytes", "packets", "errs", "drop", "fifo", "frame", "compressed", "multicast"}
	)

	data := strings.NewReader(networkStatsData)

	net, err := parseContainerNetworkInfo(data)
	if err != nil {
		log.Fatalf("failed to parse network data: %s\n", err)
	}

	for _, iface := range interfaces {
		if _, ok := net[iface]; !ok {
			t.Fatalf("failed to collect interface: %s\n", iface)
		}

		for _, topic := range topics {
			if _, ok := net[iface][topic]; !ok {
				t.Fatalf("failed to collect %s for interface %s\n", topic, iface)
			}

			if net[iface][topic].receive == "" {
				t.Fatalf("failed to collect receive metrics for %s for interface %s\n", topic, iface)
			}

			if net[iface][topic].transmit == "" {
				t.Fatalf("failed to collect transmit metrics for %s for interface %s\n", topic, iface)
			}
		}
	}

	// receive
	if want, got := "29473964", net["docker0"]["bytes"].receive; want != got {
		t.Errorf("docker0 receive bytes got: %s, expected: %s\n", got, want)
	}

	if want, got := "297207", net["docker0"]["packets"].receive; want != got {
		t.Errorf("docker0 receive packets got: %s, expected: %s\n", got, want)
	}

	// transmit
	if want, got := "977390200", net["docker0"]["bytes"].transmit; want != got {
		t.Errorf("docker0 transmit bytes got: %s, expected: %s\n", got, want)
	}

	if want, got := "289365", net["docker0"]["packets"].transmit; want != got {
		t.Errorf("docker0 transmit packets got: %s, expected: %s\n", got, want)
	}
}
