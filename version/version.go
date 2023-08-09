/*
Copyright 2019 The hostpath provisioner operator Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package version

import (
	"bufio"
	"os"
	"strings"

	"github.com/blang/semver"
)

// VersionStringFunc is the function that feeds the version string into GetVersion
var (
	VersionStringFunc = getStringFromVersionTxt
)

// GetVersion reads the version.txt and returns the version as a semver.Version.
func GetVersion() (*semver.Version, error) {
	versionString, err := VersionStringFunc()
	if err != nil {
		return nil, err
	}
	return GetVersionFromString(versionString)
}

// GetVersionFromString takes the passed in string and attempts to make semver.Version out of it.
func GetVersionFromString(versionString string) (*semver.Version, error) {
	trimmedVersion := strings.TrimPrefix(versionString, "v")
	version, err := semver.Make(trimmedVersion)
	return &version, err
}

func getStringFromVersionTxt() (string, error) {
	return GetStringFromFile("version.txt")
}

// GetStringFromFile returns the first line of the passed in file.
func GetStringFromFile(fileName string) (string, error) {
	file, err := os.OpenFile(fileName, os.O_RDONLY, os.ModeExclusive)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Scan()
	return scanner.Text(), nil
}
