package fsmeta

import (
	"testing"
	"time"
)

func TestFileTypeLetters(t *testing.T) {
	for _, ft := range AllTypes {
		got, err := ParseFileType(ft.String())
		if err != nil || got != ft {
			t.Errorf("ParseFileType(%q) = %v, %v; want %v", ft.String(), got, err, ft)
		}
	}
	if s := FileType(0).String(); s != "?" {
		t.Errorf("FileType(0).String() = %q, want ?", s)
	}
	for _, bad := range []string{"", "x", "fd", "F"} {
		if _, err := ParseFileType(bad); err == nil {
			t.Errorf("ParseFileType(%q) 应返回错误", bad)
		}
	}
}

func TestCreateTimeFallsBackToCtime(t *testing.T) {
	ctime := time.Unix(100, 0)
	btime := time.Unix(50, 0)
	if got := (FileInfo{Ctime: ctime}).CreateTime(); !got.Equal(ctime) {
		t.Errorf("无 Btime 时应返回 Ctime，得到 %v", got)
	}
	if got := (FileInfo{Ctime: ctime, Btime: btime}).CreateTime(); !got.Equal(btime) {
		t.Errorf("有 Btime 时应返回 Btime，得到 %v", got)
	}
}
