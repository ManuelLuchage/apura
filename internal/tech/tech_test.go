package tech

import (
	"reflect"
	"testing"
)

func TestInfer(t *testing.T) {
	cases := map[string][]string{
		"sudo apt install docker.io":                 {"apt"},
		"docker compose up -d && kubectl apply -f x": {"docker", "kubernetes"},
		"curl -sfL https://get.k3s.io | sh -":        {"http"},
		"ls -la":                                     nil,
	}
	for line, want := range cases {
		if got := Infer(line); !reflect.DeepEqual(got, want) {
			t.Errorf("Infer(%q) = %v, want %v", line, got, want)
		}
	}
}

func TestIsNoise(t *testing.T) {
	for line, want := range map[string]bool{
		"ls -la":                     true,
		"cat /etc/hosts | grep foo":  true,
		"cd ~/proj":                  true,
		"sudo systemctl restart x":   false,
		"cat file | sudo tee /etc/x": false,
		"":                           false,
	} {
		if got := IsNoise(line); got != want {
			t.Errorf("IsNoise(%q) = %v, want %v", line, got, want)
		}
	}
}
