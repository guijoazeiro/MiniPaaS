package docker

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/go-connections/nat"
)

func ImageBuildOptions(tag string) build.ImageBuildOptions {
	return build.ImageBuildOptions{
		Tags:        []string{tag},
		Remove:      true,
		ForceRemove: true,
	}
}

func splitPortProto(cp string) (proto, port string) {
	if i := strings.IndexByte(cp, '/'); i >= 0 {
		return cp[i+1:], cp[:i]
	}
	return "tcp", cp
}

func envSlice(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

func hostPort(portMap nat.PortMap, target nat.Port) (int, error) {
	bindings, ok := portMap[target]
	if !ok || len(bindings) == 0 {
		return 0, fmt.Errorf("no host binding for %s", target)
	}
	pick := bindings[0]
	for _, b := range bindings {
		if b.HostIP == "0.0.0.0" || b.HostIP == "" {
			pick = b
			break
		}
	}
	n, err := strconv.Atoi(pick.HostPort)
	if err != nil {
		return 0, fmt.Errorf("parse host port %q: %w", pick.HostPort, err)
	}
	return n, nil
}
