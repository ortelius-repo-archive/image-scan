package functions

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	network "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	natting "github.com/docker/go-connections/nat"
)

func ExecCommand(client *client.Client, containerId string, commands []string) error {

	createdExcComand := strings.Join(commands, " && ")
	c := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"sh", "-c", createdExcComand},
		Tty:          false,
		Detach:       false,
	}
	execID, _ := client.ContainerExecCreate(context.Background(), containerId, c)
	fmt.Println(execID)

	res, err := client.ContainerExecAttach(context.Background(), execID.ID, types.ExecStartCheck{
		Detach: false,
		Tty:    false,
	})
	if err != nil {
		return err
	}
	defer res.Close()

	err = client.ContainerExecStart(context.Background(), execID.ID, types.ExecStartCheck{})
	if err != nil {
		return err
	}

	run := true
	for run {
		resp, err := client.ContainerExecInspect(context.Background(), execID.ID)
		if err != nil {
			panic(err)
		}

		if !resp.Running {
			run = false
		}
		time.Sleep(250 * time.Millisecond)
	}

	return nil
}

func RunContainer(client *client.Client, imagename string, containername string, port string, inputEnv []string) (string, error) {
	// Define a PORT opening
	newport, err := natting.NewPort("tcp", port)
	if err != nil {
		fmt.Println("Unable to create docker port")
		return "", err
	}

	// Configured hostConfig:
	// https://godoc.org/github.com/docker/docker/api/types/container#HostConfig
	hostConfig := &container.HostConfig{
		PortBindings: natting.PortMap{
			newport: []natting.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: port,
				},
			},
		},
		RestartPolicy: container.RestartPolicy{
			Name: "always",
		},
		LogConfig: container.LogConfig{
			Type:   "json-file",
			Config: map[string]string{},
		},
	}

	// Define Network config (why isn't PORT in here...?:
	// https://godoc.org/github.com/docker/docker/api/types/network#NetworkingConfig
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{},
	}
	gatewayConfig := &network.EndpointSettings{
		Gateway: "gatewayname",
	}
	networkConfig.EndpointsConfig["bridge"] = gatewayConfig

	// Define ports to be exposed (has to be same as hostconfig.portbindings.newport)
	exposedPorts := map[natting.Port]struct{}{
		newport: {},
	}

	// Configuration
	// https://godoc.org/github.com/docker/docker/api/types/container#Config
	config := &container.Config{
		Image:        imagename,
		Env:          []string{"DB_HOST=192.168.225.51", "DB_PORT=9876"},
		ExposedPorts: exposedPorts,
		Hostname:     fmt.Sprintf("%s-hostnameexample", imagename),
	}

	// Creating the actual container. This is "nil,nil,nil" in every example.
	cont, err := client.ContainerCreate(
		context.Background(),
		config,
		hostConfig,
		networkConfig,
		nil,
		containername,
	)

	if err != nil {
		log.Println(err)
		return "", err
	}

	// Run the actual container
	client.ContainerStart(context.Background(), cont.ID, types.ContainerStartOptions{})
	log.Printf("Container %s is created", cont.ID)
	return cont.ID, err
}

func CopyFileAndRemoveContainer(client *client.Client, containerId string, dirToSave string) error {

	log.Printf("Started copying from the container")
	reader, _, err := client.CopyFromContainer(context.Background(), containerId, dirToSave)
	if err != nil {
		log.Println(err.Error())
	}
	tr := tar.NewReader(reader)
	for {
		// hdr gives you the header of the tar file
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			log.Fatalln(err)
		}
		buf := new(bytes.Buffer)
		buf.ReadFrom(tr)

		// You can use this wholeContent to create new file
		wholeContent := buf.String()

		dir, _ := path.Split(hdr.Name)
		if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(dir, os.ModePerm)
			if err != nil {
				log.Println(err)
			}
		}

		errr := ioutil.WriteFile(hdr.Name, []byte(wholeContent), 0644)
		if err != nil {
			log.Fatal(errr)
		}
	}

	log.Printf("Stops and removes the container")
	err = stopAndRemoveContainer(client, containerId)
	if err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func stopAndRemoveContainer(client *client.Client, containerId string) error {
	ctx := context.Background()

	if err := client.ContainerStop(ctx, containerId, nil); err != nil {
		log.Printf("Unable to stop container %s: %s", containerId, err)
		return err
	}

	removeOptions := types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}

	if err := client.ContainerRemove(ctx, containerId, removeOptions); err != nil {
		log.Printf("Unable to remove container: %s", err)
		return err
	}
	return nil
}

func PullImage(client *client.Client, username string, password string, imagePath string) error { //imagePath eg: 1645370/test-imag:latest

	authConfig := types.AuthConfig{
		Username: username,
		Password: password,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)

	if err != nil {
		return err
	}

	reader, err := client.ImagePull(context.Background(), imagePath, types.ImagePullOptions{RegistryAuth: authStr})
	if err != nil {
		return err
	}
	wr, err := io.Copy(os.Stdout, reader)
	fmt.Println(wr)
	if err != nil {
		return err
	}
	return nil
}
