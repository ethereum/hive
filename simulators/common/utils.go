package common

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

// truncateHead truncates the head lines, leaving somewhere slightly below 'size' bytes,
// and ensures that lines are kept intact.
func truncateHead(path string, size int64) error {
	var tmpFileName = fmt.Sprintf("%s.tmp", path)
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	// seek N bytes from the end (whence=2)
	if _, err := file.Seek(-size, 2); err != nil {
		file.Close()
		return err
	}
	reader := bufio.NewReader(file)
	// read until a line-break
	if _, err = reader.ReadString('\n'); err != nil {
		file.Close()
		return fmt.Errorf("seek failed: %v", err)
	}
	// reader is now positioned correctly, we can shove all remaining data into the next index-file
	newFile, err := os.OpenFile(tmpFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		file.Close()
		return err
	}
	io.Copy(newFile, reader)
	newFile.Close()
	file.Close()
	// Now, delete the old one, and swap in the new one
	if err = os.Remove(path); err != nil {
		return err
	}
	return os.Rename(tmpFileName, path)
}

// appendLine appends given data + linebreak to the file at the given path. The file is created with 0644
// permissions if it does not already exist
func appendLine(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(string(data) + "\n")
	f.Close()
	return err
}
