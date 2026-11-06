package server

import "testing"

func TestPublicOnly(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:80", "[::1]:443", "10.0.0.5:80", "192.168.1.1:80", "172.16.0.1:80",
		"169.254.169.254:80", "100.64.0.1:80", "0.0.0.0:80", "[fc00::1]:80", "[fe80::1]:80", "[::ffff:127.0.0.1]:80"} {
		if publicOnly("tcp", addr, nil) == nil {
			t.Errorf("%s was allowed", addr)
		}
	}
	for _, addr := range []string{"93.184.216.34:443", "[2606:4700::6810:84e5]:443"} {
		if err := publicOnly("tcp", addr, nil); err != nil {
			t.Errorf("%s refused: %v", addr, err)
		}
	}
}
