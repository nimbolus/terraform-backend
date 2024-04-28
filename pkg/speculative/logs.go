package speculative

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

type countingReader struct {
	io.Reader
	readBytes int
}

func (c *countingReader) Read(dst []byte) (int, error) {
	n, err := c.Reader.Read(dst)
	c.readBytes += n
	return n, err
}

var ignoredGroupNames = []string{
	"Operating System",
	"Runner Image",
	"Runner Image Provisioner",
	"GITHUB_TOKEN Permissions",
}

func streamLogs(logsURL *url.URL, skip int64) (int64, error) {
	logs, err := http.Get(logsURL.String())
	if err != nil {
		return 0, err
	}
	if logs.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("invalid status for logs: %d", logs.StatusCode)
	}
	defer logs.Body.Close()

	if _, err := io.Copy(io.Discard, io.LimitReader(logs.Body, skip)); err != nil {
		return 0, err
	}

	r := &countingReader{Reader: logs.Body}
	scanner := bufio.NewScanner(r)
	groupDepth := 0
	for scanner.Scan() {
		line := scanner.Text()
		ts, rest, ok := strings.Cut(line, " ")
		if !ok {
			rest = ts
		}
		if groupName, ok := strings.CutPrefix(rest, "##[group]"); ok {
			groupDepth++
			if !slices.Contains(ignoredGroupNames, groupName) {
				fmt.Printf("\n# %s\n", groupName)
			}
		}
		if groupDepth == 0 {
			fmt.Println(rest)
		}
		if strings.HasPrefix(rest, "##[endgroup]") {
			groupDepth--
		}
	}
	if err := scanner.Err(); err != nil {
		return int64(r.readBytes), err
	}

	return int64(r.readBytes), err
}
