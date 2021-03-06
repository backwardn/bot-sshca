package kbfs

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

// Returns whether or not the current system supports accessing KBFS via a FUSE filesystem mounted at /keybase
// This is used in order to optimize heavily used functions in the below library. Generally, it is preferred to
// rely on `keybase fs` commands since those are guaranteed to work across systems (and are what is used inside the
// integration tests). But in a few cases (namely when kssh is searching for kssh-client.config files) it gives very
// large speed improvements to use the FUSE filesystem when available (an order of magnitude improvement for kssh)
func supportsFuse() bool {
	// Note that this function is not tested via integration tests since fuse does not run in docker. Handle with care.
	_, err1 := os.Stat("/keybase")
	_, err2 := os.Stat("/keybase/team")
	_, err3 := os.Stat("/keybase/private")
	_, err4 := os.Stat("/keybase/public")
	return err1 == nil && err2 == nil && err3 == nil && err4 == nil
}

type Operation struct {
	KeybaseBinaryPath string
}

// Returns whether the given KBFS file exists
func (ko *Operation) FileExists(filename string) (bool, error) {
	if supportsFuse() {
		// Note that this code is not tested via integration tests since fuse does not run in docker. Handle with care.
		_, err := os.Stat(filename)
		if err == nil {
			return true, nil
		}
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	cmd := exec.Command(ko.KeybaseBinaryPath, "fs", "stat", filename)
	bytes, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	if strings.Contains(string(bytes), "ERROR file does not exist") {
		return false, nil
	}
	return false, fmt.Errorf("failed to stat %s: %s (%v)", filename, strings.TrimSpace(string(bytes)), err)
}

// Reads the specified KBFS file into a byte array
func (ko *Operation) Read(filename string) ([]byte, error) {
	if supportsFuse() {
		// Note that this code is not tested via integration tests since fuse does not run in docker. Handle with care.
		return ioutil.ReadFile(filename)
	}
	cmd := exec.Command(ko.KeybaseBinaryPath, "fs", "read", filename)
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %s (%v)", filename, strings.TrimSpace(string(bytes)), err)
	}
	return bytes, nil
}

// Delete the specified KBFS file
func (ko *Operation) Delete(filename string) error {
	cmd := exec.Command(ko.KeybaseBinaryPath, "fs", "rm", filename)
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete the file at %s: %s (%v)", filename, strings.TrimSpace(string(bytes)), err)
	}
	return nil
}

// Write contents to the specified KBFS file. If appendToFile, appends onto the end of the file. Otherwise, overwrites
// and truncates the file.
func (ko *Operation) Write(filename string, contents string, appendToFile bool) error {
	var cmd *exec.Cmd
	if appendToFile {
		// `keybase fs write --append` only works if the file already exists so create it if it does not exist
		exists, err := ko.FileExists(filename)
		if !exists || err != nil {
			err = ko.Write(filename, "", false)
			if err != nil {
				return err
			}
		}
		cmd = exec.Command(ko.KeybaseBinaryPath, "fs", "write", "--append", filename)
	} else {
		cmd = exec.Command(ko.KeybaseBinaryPath, "fs", "write", filename)
	}

	cmd.Stdin = strings.NewReader(contents)
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write to file at %s: %s (%v)", filename, strings.TrimSpace(string(bytes)), err)
	}
	return nil
}

// List KBFS files in the given KBFS path
func (ko *Operation) List(path string) ([]string, error) {
	cmd := exec.Command(ko.KeybaseBinaryPath, "fs", "ls", "-1", "--nocolor", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list files in /keybase/team/: %s (%v)", strings.TrimSpace(string(output)), err)
	}
	var ret []string
	for _, s := range strings.Split(string(output), "\n") {
		if s != "" {
			ret = append(ret, s)
		}
	}
	return ret, nil
}
