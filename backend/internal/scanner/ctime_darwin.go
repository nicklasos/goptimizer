package scanner

import (
	"io/fs"
	"syscall"
	"time"
)

func changeTime(info fs.FileInfo) time.Time {
	return time.Unix(info.Sys().(*syscall.Stat_t).Ctimespec.Unix())
}
