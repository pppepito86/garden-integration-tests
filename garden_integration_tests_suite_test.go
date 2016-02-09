package garden_integration_tests_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	gardenHost            string
	gardenClient          garden.Client
	container             garden.Container
	containerCreateErr    error
	assertContainerCreate bool

	rootfs              string
	privilegedContainer bool
	properties          garden.Properties
	limits              garden.Limits
	env                 []string
	ginkgoIO            garden.ProcessIO = garden.ProcessIO{
		Stdout: GinkgoWriter,
		Stderr: GinkgoWriter,
	}
)

func TestGardenIntegrationTests(t *testing.T) {
	RegisterFailHandler(Fail)

	SetDefaultEventuallyTimeout(5 * time.Second)

	BeforeEach(func() {
		assertContainerCreate = true
		// rootfs = "docker:///cloudfoundry/garden-busybox"
		rootfs = ""
		privilegedContainer = false
		properties = garden.Properties{}
		limits = garden.Limits{}
		env = []string{}
		gardenHost = os.Getenv("GARDEN_ADDRESS")
		if gardenHost == "" {
			gardenHost = "10.244.16.6:7777"
		}
		gardenClient = client.New(connection.New("tcp", gardenHost))
	})

	JustBeforeEach(func() {
		container, containerCreateErr = gardenClient.Create(garden.ContainerSpec{
			RootFSPath: rootfs,
			Privileged: privilegedContainer,
			Properties: properties,
			Env:        env,
			Limits:     limits,
		})

		if assertContainerCreate {
			Expect(containerCreateErr).ToNot(HaveOccurred())
		}

		// Add alice user to guardian tests, because guardian doesn't yet support
		// pulling images from docker. Once it does, we'll be able to (successfully)
		// use the garden busybox image on dockerhub, which has alice already.
		createUser(container, "alice")

	})

	AfterEach(func() {
		if container != nil {
			Expect(gardenClient.Destroy(container.Handle())).To(Succeed())
		}
	})

	RunSpecs(t, "GardenIntegrationTests Suite")
}

func getContainerHandles() []string {
	containers, err := gardenClient.Containers(nil)
	Expect(err).ToNot(HaveOccurred())

	handles := make([]string, len(containers))
	for i, c := range containers {
		handles[i] = c.Handle()
	}

	return handles
}

func createUser(container garden.Container, username string) {
	process, err := container.Run(garden.ProcessSpec{
		User: "root",
		Path: "sh",
		Args: []string{"-c", fmt.Sprintf("id -u %s || adduser -D %s", username, username)},
	}, garden.ProcessIO{
		Stdout: GinkgoWriter,
		Stderr: GinkgoWriter,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(process.Wait()).To(Equal(0))
}
