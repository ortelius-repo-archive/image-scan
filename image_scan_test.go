package main

import (
	"os"
	"testing"

	"github.com/docker/docker/client"
)

func TestPositiveCase(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	// cli, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("Unable to create docker client")
	}

	imagename := "1645370/ortelius-test:latest"                                                                                                                                                                                                        //mandatory input
	commands := []string{"python -m ensurepip --upgrade", "pip3 freeze > requirements.txt", "pip3 install cyclonedx-bom==0.4.3 safety", "cyclonedx-py -j -o /tmp/sbom.json", "safety check -r requirements.txt --json --output /tmp/cve.json || true"} //mandatory input
	directoryToSaveGeneratedFiles := "/tmp"

	inputEnv := []string{"DB_HOST=192.168.225.51", "DB_PORT=9876"}

	err = ImageScanWithCustomCommands(cli, imagename, commands, directoryToSaveGeneratedFiles, inputEnv)
	if err != nil {
		t.Errorf("Error occured while ImageScanWithCustomCommands(); err= %s", err)
	}

	exist, boolErr := isFileExists("tmp/cve.json")
	if boolErr != nil || !exist {
		t.Errorf("Required file not generated")
	}

	exist, boolErr = isFileExists("tmp/sbom.json")
	if boolErr != nil || !exist {
		t.Errorf("Required file not generated")
	}

	// err = os.RemoveAll("tmp/")
	// if err != nil {
	// 	t.Errorf("Cleaning of the generated files failed")
	// }
}

//negative test cases

func TestImageNotFound(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	// cli, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("Unable to create docker client")
	}

	imagename := "1645370/ortelius:latest"                                                                                                                                                                                                             //mandatory input
	commands := []string{"python -m ensurepip --upgrade", "pip3 freeze > requirements.txt", "pip3 install cyclonedx-bom==0.4.3 safety", "cyclonedx-py -j -o /tmp/sbom.json", "safety check -r requirements.txt --json --output /tmp/cve.json || true"} //mandatory input
	directoryToSaveGeneratedFiles := "/tmp"

	inputEnv := []string{"DB_HOST=192.168.225.51", "DB_PORT=9876"}

	err = ImageScanWithCustomCommands(cli, imagename, commands, directoryToSaveGeneratedFiles, inputEnv)

	if err == nil {
		t.Errorf("Error excpected but found nil")
	}
}

// exists returns whether the given file or directory exists
func isFileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
