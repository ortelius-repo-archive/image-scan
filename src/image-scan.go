package main

import (
	"fmt"
	"log"

	"src/functions"

	"github.com/docker/docker/client"
)

func ImageScanWithCustomCommands(client *client.Client, imagename string, containername string, port string, inputEnv []string, commands []string) error {

	//---------- Start container -------------
	containerId, err := functions.RunContainer(client, imagename, containername, port, inputEnv)

	if err != nil {
		fmt.Println(err)
		return err
	}

	//---------- Execute commands inside container -------------
	err = functions.ExecCommand(client, containerId, commands)
	if err != nil {
		fmt.Println(err)
		return err
	}

	// ---------- Copy generated files to host directory -------------
	err = functions.CopyFileAndRemoveContainer(client, containerId) //This method will also remove the container after task is completed
	if err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func main() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	// cli, err := client.NewEnvClient()
	if err != nil {
		log.Fatalf("Unable to create docker client")
	}

	imagename := "go-test:img" //mandatory input
	containername := "go-test" //not necessarily be taken from the user
	portopening := "8080"      //not necessarily be taken from the user
	inputEnv := []string{fmt.Sprintf("LISTENINGPORT=%s", portopening)}

	//mandatory input
	commands := []string{"python -m ensurepip --upgrade", "pip3 freeze > requirements.txt", "pip3 install cyclonedx-bom==0.4.3 safety", "cyclonedx-py -j -o /tmp/sbom.json", "safety check -r requirements.txt --json --output /tmp/cve.json || true"}

	err = ImageScanWithCustomCommands(cli, imagename, containername, portopening, inputEnv, commands)
	if err != nil {
		log.Println(err)
	}
}
