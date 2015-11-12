package garden_integration_tests_test

import (
	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
)

var _ = FDescribe("Mount", func() {
	BeforeEach(func() {
		privilegedContainer = true
		limits = garden.Limits{
			Disk: garden.DiskLimits{
				ByteHard: 1024 * 1024 * 10,
			},
		}
	})

	It("Can mount a filesystem in the container", func() {
		container.Run(garden.ProcessSpec{
			Path: "mount",
			Args: []string{"-t", "tmpfs", "none", "/tmp"},
			User: "root",
		}, garden.ProcessIO{
			Stdout: GinkgoWriter,
			Stderr: GinkgoWriter,
		})
	})

})
