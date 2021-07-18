package functions

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	network "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	natting "github.com/docker/go-connections/nat"
)

func BuildImage(client *client.Client, tags []string, directory string, dependentFileNames []string, excCustomCommand string) error {
	ctx := context.Background()

	// Create a buffer
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	//add files mentioned in dockerfile [this is issue]
	for _, file := range dependentFileNames {
		err := addToArchive(tw, file)
		if err != nil {
			return err
		}
	}

	// Create a filereader
	dockerFileReader, err := os.Open(directory)
	if err != nil {
		return err
	}

	// Read the actual Dockerfile
	readDockerFile, err := ioutil.ReadAll(dockerFileReader)
	if err != nil {
		return err
	}

	readDockerFileFinal := []byte{}
	if len(excCustomCommand) > 0 {
		checkIfPipInstalled := []byte("RUN python -m ensurepip --upgrade;")
		dymanicInjectable := []byte(excCustomCommand)
		readDockerFileFinal = append(append(readDockerFile, checkIfPipInstalled...), dymanicInjectable...)
	} else {
		readDockerFileFinal = readDockerFile
	}

	// Make a TAR header for the fill
	tarHeader := &tar.Header{
		Name: directory,
		Size: int64(len(readDockerFileFinal)),
	}

	// Writes the header described for the TAR file
	err = tw.WriteHeader(tarHeader)
	if err != nil {
		return err
	}

	// Writes the dockerfile data to the TAR file
	_, err = tw.Write(readDockerFileFinal)
	if err != nil {
		return err
	}

	dockerBuildContext := bytes.NewReader(buf.Bytes())

	// Define the build options to use for the file
	// https://godoc.org/github.com/docker/docker/api/types#ImageBuildOptions
	buildOptions := types.ImageBuildOptions{
		Context:        dockerBuildContext,
		Dockerfile:     directory,
		Remove:         true,
		Tags:           tags,
		PullParent:     true,
		SuppressOutput: true,
	}

	// Build the actual image
	log.Println("Started building Image")
	imageBuildResponse, err := client.ImageBuild(
		ctx,
		dockerBuildContext,
		buildOptions,
	)

	if err != nil {
		return err
	}

	// Read the STDOUT from the build process
	defer imageBuildResponse.Body.Close()
	_, err = io.Copy(os.Stdout, imageBuildResponse.Body)
	if err != nil {
		return err
	}
	log.Println("Image build successfully")
	return nil
}

func addToArchive(tw *tar.Writer, filename string) error {
	// Open the file which will be written into the archive
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get FileInfo about our file providing file size, mode, etc.
	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Create a tar Header from the FileInfo data
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	// Use full path as name (FileInfoHeader only takes the basename)
	// If we don't do this the directory strucuture would
	// not be preserved
	// https://golang.org/src/archive/tar/common.go?#L626
	header.Name = filename

	// Write file header to the tar archive
	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}

	// Copy file content to tar archive
	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}

	return nil
}

func RunContainer(client *client.Client, imagename string, containername string, port string, inputEnv []string) error {
	// Define a PORT opening
	newport, err := natting.NewPort("tcp", port)
	if err != nil {
		fmt.Println("Unable to create docker port")
		return err
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
		Env:          inputEnv,
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
		return err
	}

	// Run the actual container
	
	client.ContainerStart(context.Background(), cont.ID, types.ContainerStartOptions{})
	log.Printf("Container %s started", cont.ID)

	log.Printf("Started copying from the container")
	reader, _, err := client.CopyFromContainer(context.Background(), cont.ID, "/tmp")
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
	stopAndRemoveContainer(client, containername)
	return nil
}

func stopAndRemoveContainer(client *client.Client, containername string) error {
	ctx := context.Background()

	if err := client.ContainerStop(ctx, containername, nil); err != nil {
		log.Printf("Unable to stop container %s: %s", containername, err)
	}

	removeOptions := types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}

	if err := client.ContainerRemove(ctx, containername, removeOptions); err != nil {
		log.Printf("Unable to remove container: %s", err)
		return err
	}
	return nil
}

func RemoveBuildImage(client *client.Client, imageName string) error {
	ctx := context.Background()

	if _, err := client.ImageRemove(ctx, imageName, types.ImageRemoveOptions{Force: false, PruneChildren: false}); err != nil {
		log.Printf("Unable to remove image: %s", err)
		return err
	}
	return nil
}
