package garden_integration_tests_test

import (
	"io"

	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Rootfses", func() {
	Context("when the rootfs path is a docker image URL", func() {
		JustBeforeEach(func() {
			var err error
			container, err = createTestContainer()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("and the docker image specifies $PATH", func() {
			BeforeEach(func() {
				// Dockerfile contains:
				//   ENV PATH /usr/local/bin:/usr/bin:/bin:/from-dockerfile
				//   ENV TEST test-from-dockerfile
				//   ENV TEST second-test-from-dockerfile:$TEST
				// see diego-dockerfiles/with-volume
				rootfs = "docker:///cloudfoundry/with-volume"
			})

			It("$PATH is taken from the docker image", func() {
				stdout := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{
					User: "vcap",
					Path: "/bin/sh",
					Args: []string{"-c", "echo $PATH"},
				}, garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, stdout),
					Stderr: GinkgoWriter,
				})

				Expect(err).ToNot(HaveOccurred())

				process.Wait()
				Expect(stdout).To(gbytes.Say("/usr/local/bin:/usr/bin:/bin:/from-dockerfile"))
			})

			It("$TEST is taken from the docker image", func() {
				stdout := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{
					User: "vcap",
					Path: "/bin/sh",
					Args: []string{"-c", "echo $TEST"},
				}, garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, stdout),
					Stderr: GinkgoWriter,
				})

				Expect(err).ToNot(HaveOccurred())

				process.Wait()
				Expect(stdout).To(gbytes.Say("second-test-from-dockerfile:test-from-dockerfile"))
			})
		})

		Describe("quota support", func() {
			Context("when the docker image is smaller than the container's quota", func() {
				It("should allow creation", func() {
					_, err := gardenClient.Create(garden.ContainerSpec{
						RootFSPath: "docker:///busybox",
						Limits: garden.Limits{
							Disk: garden.DiskLimits{
								ByteHard: 3000000,
							},
						},
					})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the docker image is larger than the container's quota", func() {
				It("should fail to create", func() {
					_, err := gardenClient.Create(garden.ContainerSpec{
						RootFSPath: "docker:///busybox",
						Limits: garden.Limits{
							Disk: garden.DiskLimits{
								ByteHard: 1000000,
							},
						},
					})
					Expect(err).To(MatchError("quota exceeded"))
				})
			})
		})
	})
})
