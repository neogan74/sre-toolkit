// Package follow streams lines appended to a file, like `tail -f`. It polls
// instead of using inotify, so it works the same on every platform and on
// network filesystems, and it survives truncation and log rotation.
package follow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

// DefaultInterval is how often the file is checked for new data.
const DefaultInterval = 250 * time.Millisecond

const readBufSize = 64 * 1024

// Lines calls fn for every complete line appended to path after offset, until
// ctx is cancelled (which is not an error). If the file is truncated it is
// re-read from the start; if it is replaced (rotation) the new file is read
// from the start once the old one is drained. A missing file is waited for.
func Lines(ctx context.Context, path string, offset int64, interval time.Duration, fn func(line string)) error {
	if interval <= 0 {
		interval = DefaultInterval
	}
	f, err := os.Open(path) //nolint:gosec // file path is provided by user via CLI argument
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("seeking %s: %w", path, err)
	}

	var pending []byte
	buf := make([]byte, readBufSize)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		// Drain everything currently available.
		for {
			n, err := f.Read(buf)
			if n > 0 {
				pending = emit(append(pending, buf[:n]...), fn)
			}
			if err != nil {
				if err != io.EOF {
					return fmt.Errorf("reading %s: %w", path, err)
				}
				break
			}
		}

		next, rewound, err := reopenIfReplaced(f, path)
		switch {
		case err != nil:
			return err
		case rewound:
			pending = nil // partial line belonged to the pre-truncation content
		case next != nil:
			if len(pending) > 0 { // unterminated last line of the old file
				fn(string(bytes.TrimSuffix(pending, []byte("\r"))))
				pending = nil
			}
			f.Close() //nolint:errcheck,gosec // read-only file
			f = next
			continue // read the new file right away
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// emit calls fn for each complete line in data and returns the remainder.
func emit(data []byte, fn func(string)) []byte {
	for {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			return data
		}
		fn(string(bytes.TrimSuffix(data[:i], []byte("\r"))))
		data = data[i+1:]
	}
}

// reopenIfReplaced returns a freshly opened file when path now refers to a
// different file than f. When the same file shrank, it rewinds f and reports
// rewound. A path that does not exist (mid-rotation) is not an error: the
// caller keeps f and checks again next tick.
func reopenIfReplaced(f *os.File, path string) (next *os.File, rewound bool, err error) {
	cur, err := f.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("stat open file: %w", err)
	}
	onDisk, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("stat %s: %w", path, err)
	}
	if !os.SameFile(cur, onDisk) {
		next, err = os.Open(path) //nolint:gosec // file path is provided by user via CLI argument
		if err != nil {
			if os.IsNotExist(err) {
				return nil, false, nil
			}
			return nil, false, err
		}
		return next, false, nil
	}
	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, false, fmt.Errorf("seeking %s: %w", path, err)
	}
	if onDisk.Size() < pos {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, false, fmt.Errorf("seeking %s: %w", path, err)
		}
		return nil, true, nil
	}
	return nil, false, nil
}
