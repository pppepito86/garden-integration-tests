package garden_integration_tests_test

import (
	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("goroutine leak", func() {

	runProcess := func(container garden.Container) {
		stdout := gbytes.NewBuffer()

		process, err := container.Run(garden.ProcessSpec{
			User: "vcap",
			Path: "echo",
			Args: []string{"hello"},
		}, garden.ProcessIO{
			Stdout: stdout,
		})
		Expect(err).ToNot(HaveOccurred())

		exitStatus, err := process.Wait()
		Expect(err).ToNot(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
	}

	numGoroutine := func(client garden.Client) int {
		cap, err := client.Capacity()
		Expect(err).NotTo(HaveOccurred())
		return int(cap.NumGoroutine)
	}

	stacks := func(client garden.Client) string {
		cap, err := client.Capacity()
		Expect(err).NotTo(HaveOccurred())
		return cap.Stacks
	}

	FIt("does not leak goroutines as multiple processes are run", func() {
		initialNumGoroutine := numGoroutine(gardenClient)

		for i := 0; i < 1; i++ {
			runProcess(container)
		}

		// If the number of goroutines has shrunk, this test is worthless, so bail out.
		Expect(numGoroutine(gardenClient)).Should(BeNumerically(">=", initialNumGoroutine))

		Eventually(func() int {
			return numGoroutine(gardenClient)
		}, "70s").Should(Equal(initialNumGoroutine), stacks(gardenClient))
	})

})
