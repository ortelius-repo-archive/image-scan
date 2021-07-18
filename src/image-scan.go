package main

import (
	"fmt"
	"log"

	"src/functions"

	"github.com/docker/docker/client"
)


func main() {
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Unable to create docker client: %s", err)
	}

	dockerfile := "C:/Users/utkar/git/ortelius-ms-textfile-crud/Dockerfile"
	dependentFileNames := []string{"requirements.txt", "main.py"}
	excCustomCommands := "\nRUN pip3 freeze > requirements.txt; pip3 install cyclonedx-bom==0.4.3 safety 2>/dev/null 1>/dev/null; cyclonedx-py -j -o /tmp/sbom.json; safety check -r requirements.txt --json --output /tmp/cve.json || true;"

	ImageScan(client, dockerfile, dependentFileNames, excCustomCommands)
}

func ImageScan(client *client.Client, dockerfile string, dependentFileNames []string, excCustomCommands string) error {
	
	imagename := "go-check"
	tags := []string{imagename}
	
	err := functions.BuildImage(client, tags, dockerfile, dependentFileNames, excCustomCommands)
	if err != nil {
		log.Println(err)
		return err
	}
	
	containername := imagename
	portopening := "8080"
	inputEnv := []string{fmt.Sprintf("LISTENINGPORT=%s", portopening)}
	
	err = functions.RunContainer(client, imagename, containername, portopening, inputEnv)
	if err != nil {
		log.Println(err)
	}

	err = functions.RemoveBuildImage(client, imagename)
	if err != nil {
		log.Println(err)
	}
	return nil
}
