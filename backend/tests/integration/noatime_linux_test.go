package integration

import "golang.org/x/sys/unix"

// oNoAtime makes stateOf's read leave the atime alone. On Linux a read moves an atime that is
// older than the file's mtime or ctime to now (relatime), and the tests compare atimes.
const oNoAtime = unix.O_NOATIME
