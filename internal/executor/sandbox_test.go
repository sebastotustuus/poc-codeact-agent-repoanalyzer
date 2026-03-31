package executor

import "testing"

func TestValidateCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		script  string
		wantErr bool
	}{
		{name: "simple ls", script: "ls -la", wantErr: false},
		{name: "git clone", script: "git clone https://github.com/user/repo", wantErr: false},
		{name: "find and grep", script: "find . -name '*.go' | grep main", wantErr: false},
		{name: "cat a file", script: "cat README.md", wantErr: false},
		{name: "rm blocked", script: "rm -rf /", wantErr: true},
		{name: "rmdir blocked", script: "rmdir somedir", wantErr: true},
		{name: "sudo blocked", script: "sudo apt-get install curl", wantErr: true},
		{name: "su blocked", script: "su root", wantErr: true},
		{name: "chmod blocked", script: "chmod 777 script.sh", wantErr: true},
		{name: "chown blocked", script: "chown user:user file", wantErr: true},
		{name: "dd blocked", script: "dd if=/dev/zero of=/dev/sda", wantErr: true},
		{name: "mkfs blocked", script: "mkfs.ext4 /dev/sdb", wantErr: true},
		{name: "rm in compound", script: "ls -la && rm -rf /", wantErr: true},
		{name: "rm after semicolon", script: "echo hello; rm -rf /", wantErr: true},
		{name: "rm after pipe", script: "echo hello | rm foo", wantErr: true},
		{name: "rm after or", script: "false || rm foo", wantErr: true},
		{name: "rm as argument allowed", script: "grep rm /etc/passwd", wantErr: false},
		{name: "full path rm blocked", script: "/usr/bin/rm -f file", wantErr: true},
		{name: "multiline with blocked", script: "ls\ncat file\nrm badfile", wantErr: true},
		{name: "multiline clean", script: "ls\ncat file\nwc -l *.go", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateCommand(tt.script)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
