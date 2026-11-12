package capture

import "testing"

func TestSSHDestination(t *testing.T) {
	for _, tt := range []struct {
		line, want string
	}{
		{"ssh prod", "prod"},
		{"/usr/bin/ssh prod", "prod"},
		{"ssh -p 2222 prod", "prod"},
		{"ssh -p2222 prod", "prod"},
		{"ssh -vp 22 prod", "prod"},
		{"ssh -N -L 8080:localhost:80 prod", "prod"},
		{"ssh -o StrictHostKeyChecking=no me@10.0.0.1", "10.0.0.1"},
		{"ssh -i key.pem -J jump box ls -la", "box"},
		{"ssh ssh://me@box", "box"},
		{"ssh", ""},
		{"ssh -p 22", ""},
		{"ssh 'a;b'", ""},
		{"sshd prod", ""},
		{"vim ssh", ""},
	} {
		got, ok := SSHDestination(tt.line)
		if got != tt.want || ok != (tt.want != "") {
			t.Errorf("%q: got %q,%v want %q", tt.line, got, ok, tt.want)
		}
	}
}
