package isureal

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type RegionListener struct{ Region, Address string }

func (c *Config) RegionListeners() ([]RegionListener, error) {
	spec := c.ListenRegions
	if spec == "" {
		spec = c.Region + ":" + envOrDefault("ISUREAL_PORT", "8080")
	}
	if c.EdgeUpstream != "" && strings.Contains(spec, ",") {
		return nil, fmt.Errorf("edge must have exactly one region listener")
	}
	var out []RegionListener
	regions, ports := map[string]bool{}, map[int]bool{}
	for _, entry := range strings.Split(spec, ",") {
		parts := strings.Split(entry, ":")
		if len(parts) != 2 || !validRegion(parts[0]) {
			return nil, fmt.Errorf("invalid region listener")
		}
		p, err := strconv.Atoi(parts[1])
		if err != nil || p < 1 || p > 65535 || regions[parts[0]] || ports[p] {
			return nil, fmt.Errorf("invalid or duplicate region listener")
		}
		if c.EdgeUpstream != "" && parts[0] != c.Region {
			return nil, fmt.Errorf("edge region mismatch")
		}
		regions[parts[0]] = true
		ports[p] = true
		out = append(out, RegionListener{parts[0], net.JoinHostPort(os.Getenv("ISUREAL_BIND_ADDRESS"), strconv.Itoa(p))})
	}
	if (c.EdgeKey != "" || c.EdgeUpstream != "" || len(out) > 1) && len(c.EdgeKey) < 32 {
		return nil, fmt.Errorf("edge key must contain at least 32 bytes")
	}
	return out, nil
}
