package libreoffice

import (
	"runtime"
	"testing"
)

func TestFileURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		if got := fileURL(`C:\tmp\profile`); got != "file:///C:/tmp/profile" {
			t.Fatalf("fileURL = %q", got)
		}
		return
	}
	if got := fileURL("/tmp/profile"); got != "file:///tmp/profile" {
		t.Fatalf("fileURL = %q", got)
	}
}
