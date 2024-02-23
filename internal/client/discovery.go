package client

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
)

type ServiceDiscovery struct {
}

func (ps *ServiceDiscovery) Scan() []string {
	const timeout = 500 * time.Millisecond
	const port = 8080
	var lock *semaphore.Weighted = semaphore.NewWeighted(1024)
	ips := localAddresses()

	wg := sync.WaitGroup{}

	// we only run on 8080 (for now); don't bother checking other ports
	hostIPs := make([]string, 0)
	for _, ip := range ips {
		wg.Add(1)
		lock.Acquire(context.TODO(), 1)
		go func(ip string, port int) {
			defer lock.Release(1)
			defer wg.Done()
			resultIP := scanPort(ip, port, timeout)
			if resultIP != "" {
				hostIPs = append(hostIPs, resultIP)
			}
		}(ip, port)
	}

	wg.Wait()
	return hostIPs
}

func scanPort(ip string, port int, timeout time.Duration) string {
	target := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", target, timeout)

	if err != nil {
		if strings.Contains(err.Error(), "too many open files") {
			time.Sleep(timeout)
			scanPort(ip, port, timeout)
		}
		return ""
	}

	conn.Close()
	return ip
}

func localAddresses() []string {
	ips := []string{"localhost"}

	// super rudimentary but only plan on using this on my own home network
	for i := 2; i < 254; i++ {
		ips = append(ips, fmt.Sprintf("10.0.0.%d", i))
	}

	return ips
}
