package garden_integration_tests_test

import (
	"os"

	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Limits", func() {
	Describe("LimitMemory", func() {
		Context("with a memory limit", func() {
			JustBeforeEach(func() {
				err := container.LimitMemory(garden.MemoryLimits{
					LimitInBytes: 64 * 1024 * 1024,
				})
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the process writes too much to /dev/shm", func() {
				It("is killed", func() {
					process, err := container.Run(garden.ProcessSpec{
						User: "alice",
						Path: "dd",
						Args: []string{"if=/dev/urandom", "of=/dev/shm/too-big", "bs=1M", "count=65"},
					}, garden.ProcessIO{})
					Expect(err).ToNot(HaveOccurred())

					Expect(process.Wait()).ToNot(Equal(0))
				})
			})
		})
	})

	Describe("LimitDisk", func() {
		Context("when quotas are enabled and there is a disk limit", func() {
			const BTRFS_WAIT_TIME = 120

			BeforeEach(func() {
				if os.Getenv("BTRFS_SUPPORTED") == "" {
					Skip("btrfs not available")
				}

				limits.Disk.ByteSoft = 180 * 1024 * 1024
				limits.Disk.ByteHard = 180 * 1024 * 1024
				limits.Disk.Scope = garden.DiskLimitScopeTotal
			})

			PContext("on a directory rootfs container", func() {
				It("reports correct disk usage", func() {
					var TotalBytesUsed = func() uint64 {
						metrics, err := container.Metrics()
						Expect(err).ToNot(HaveOccurred())
						return metrics.DiskStat.TotalBytesUsed
					}

					var ExclusiveBytesUsed = func() uint64 {
						metrics, err := container.Metrics()
						Expect(err).ToNot(HaveOccurred())
						return metrics.DiskStat.ExclusiveBytesUsed
					}

					initialTotalBytes := TotalBytesUsed()
					initialExclusiveBytes := ExclusiveBytesUsed()

					process, err := container.Run(garden.ProcessSpec{
						User: "alice",
						Path: "dd",
						Args: []string{"if=/dev/zero", "of=/home/alice/some-file", "bs=1M", "count=10"},
					}, garden.ProcessIO{})
					Expect(err).ToNot(HaveOccurred())
					Expect(process.Wait()).To(Equal(0))

					Eventually(TotalBytesUsed).Should(Equal(initialTotalBytes + uint64(10*1024*1024)))
					Eventually(ExclusiveBytesUsed).Should(BeNumerically("~", initialExclusiveBytes+uint64(10*1024*1024), uint64(1024*1024)))

					process, err = container.Run(garden.ProcessSpec{
						User: "alice",
						Path: "dd",
						Args: []string{"if=/dev/zero", "of=/home/alice/another-file", "bs=1M", "count=10"},
					}, garden.ProcessIO{})
					Expect(err).ToNot(HaveOccurred())
					Expect(process.Wait()).To(Equal(0))

					Eventually(TotalBytesUsed).Should(Equal(initialTotalBytes + uint64(20*1024*1024)))
					Eventually(ExclusiveBytesUsed).Should(BeNumerically("~", initialExclusiveBytes+uint64(20*1024*1024), uint64(1024*1024)))
				})
			})

			Context("on a Docker container", func() {
				BeforeEach(func() {
					privilegedContainer = false
					rootfs = "docker:///busybox#1.23"
					limits.Disk.ByteSoft = 9*1024*1024 + 1024
					limits.Disk.ByteHard = uint64(9*1024*1024 + 1024)
					limits.Disk.Scope = garden.DiskLimitScopeTotal
				})

				PIt("reports the correct disk limit size of the container", func() {
					limit, err := container.CurrentDiskLimits()
					Expect(err).ToNot(HaveOccurred())
					Expect(limit).To(Equal(garden.DiskLimits{
						ByteHard: limits.Disk.ByteHard,
						ByteSoft: limits.Disk.ByteSoft,
						Scope:    limits.Disk.Scope,
					}))
				})

				PContext("when the scope is total (the default)", func() {
					Context("and run a process that does not exceed the limit", func() {
						It("does not kill the process", func() {
							dd, err := container.Run(garden.ProcessSpec{
								User: "root",
								Path: "dd",
								Args: []string{"if=/dev/random", "of=/root/test", "bs=1M", "count=3"}, // should succeed, even though equivalent with 'total' scope does not
							}, garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter})
							Expect(err).ToNot(HaveOccurred())
							Expect(dd.Wait()).To(Equal(0))
						})
					})

					Context("and run a process that exceeds the quota due to the size of the rootfs", func() {
						It("kills the process", func() {
							dd, err := container.Run(garden.ProcessSpec{
								User: "root",
								Path: "dd",
								Args: []string{"if=/dev/zero", "of=/root/test", "bs=1M", "count=9"}, // assume busybox itself accounts for a few MB
							}, garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter})
							Expect(err).ToNot(HaveOccurred())
							Expect(dd.Wait()).ToNot(Equal(0))
						})
					})
				})

				Context("when the scope is exclusive", func() {
					BeforeEach(func() {
						limits.Disk.Scope = garden.DiskLimitScopeExclusive
						rootfs = "docker:///cloudfoundry/busyboxplus#curl"
					})

					Context("and run a process that would exceed the quota due to the size of the rootfs (but doesnt since this is not included)", func() {
						It("does not kill the process", func() {
							dd, err := container.Run(garden.ProcessSpec{
								User: "root",
								Path: "dd",
								Args: []string{"if=/dev/zero", "of=/root/test", "bs=1M", "count=7"}, // should succeed, even though equivalent with 'total' scope does not
							}, garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter})
							Expect(err).ToNot(HaveOccurred())
							Expect(dd.Wait()).To(Equal(0))
						})
					})

					Context("and run a process that exceeds the quota", func() {
						It("kills the process", func() {
							dd, err := container.Run(garden.ProcessSpec{
								User: "root",
								Path: "dd",
								Args: []string{"if=/dev/zero", "of=/root/test", "bs=1M", "count=11"},
							}, garden.ProcessIO{})
							Expect(err).ToNot(HaveOccurred())
							Expect(dd.Wait()).ToNot(Equal(0))
						})
					})
				})

				PContext("on a rootfs with pre-existing users", func() {
					BeforeEach(func() {
						rootfs = "docker:///cloudfoundry/preexisting_users"
					})

					Context("and run a process that exceeds the quota as bob", func() {
						BeforeEach(func() {
							limits.Disk.ByteSoft = 10 * 1024 * 1024
							limits.Disk.ByteHard = 10 * 1024 * 1024
							limits.Disk.Scope = garden.DiskLimitScopeTotal
						})

						It("kills the process", func() {
							dd, err := container.Run(garden.ProcessSpec{
								User: "bob",
								Path: "dd",
								Args: []string{"if=/dev/zero", "of=/home/bob/test", "bs=1M", "count=11"},
							}, garden.ProcessIO{})
							Expect(err).ToNot(HaveOccurred())
							Expect(dd.Wait()).ToNot(Equal(0))
						})
					})

					Context("and run a process that exceeds the quota as alice", func() {
						BeforeEach(func() {
							limits.Disk.ByteSoft = 10 * 1024 * 1024
							limits.Disk.ByteHard = 10 * 1024 * 1024
							limits.Disk.Scope = garden.DiskLimitScopeTotal
						})

						It("kills the process", func() {
							dd, err := container.Run(garden.ProcessSpec{
								User: "alice",
								Path: "dd",
								Args: []string{"if=/dev/zero", "of=/home/alice/test", "bs=1M", "count=11"},
							}, garden.ProcessIO{})
							Expect(err).ToNot(HaveOccurred())
							Expect(dd.Wait()).ToNot(Equal(0))
						})
					})

					Context("user alice is getting near the set limit", func() {
						JustBeforeEach(func() {
							metrics, err := container.Metrics()
							Expect(err).ToNot(HaveOccurred())
							bytesUsed := metrics.DiskStat.TotalBytesUsed

							err = container.LimitDisk(garden.DiskLimits{
								ByteSoft: 10*1024*1024 + bytesUsed,
								ByteHard: 10*1024*1024 + bytesUsed,
							})
							Expect(err).ToNot(HaveOccurred())

							dd, err := container.Run(garden.ProcessSpec{
								User: "alice",
								Path: "dd",
								Args: []string{"if=/dev/zero", "of=/home/alice/test", "bs=1M", "count=8"},
							}, garden.ProcessIO{
								Stderr: GinkgoWriter,
								Stdout: GinkgoWriter,
							})
							Expect(err).ToNot(HaveOccurred())
							Expect(dd.Wait()).To(Equal(0))
						})

						It("kills the process if user bob tries to exceed the shared limit", func() {
							dd, err := container.Run(garden.ProcessSpec{
								User: "bob",
								Path: "dd",
								Args: []string{"if=/dev/zero", "of=/home/bob/test", "bs=1M", "count=3"},
							}, garden.ProcessIO{
								Stderr: GinkgoWriter,
								Stdout: GinkgoWriter,
							})
							Expect(err).ToNot(HaveOccurred())
							Expect(dd.Wait()).ToNot(Equal(0))
						})
					})
				})

				PContext("that is privileged", func() {
					BeforeEach(func() {
						privilegedContainer = true
					})

					Context("and run a process that exceeds the quota as root", func() {
						It("kills the process", func() {
							dd, err := container.Run(garden.ProcessSpec{
								User: "root",
								Path: "dd",
								Args: []string{"if=/dev/zero", "of=/root/test", "bs=1M", "count=11"},
							}, garden.ProcessIO{})
							Expect(err).ToNot(HaveOccurred())
							Expect(dd.Wait()).ToNot(Equal(0))
						})
					})

					Context("and run a process that exceeds the quota as a new user", func() {
						It("kills the process", func() {
							addUser, err := container.Run(garden.ProcessSpec{
								User: "root",
								Path: "adduser",
								Args: []string{"-D", "-g", "", "bob"},
							}, garden.ProcessIO{})
							Expect(err).ToNot(HaveOccurred())
							Expect(addUser.Wait()).To(Equal(0))

							dd, err := container.Run(garden.ProcessSpec{
								User: "bob",
								Path: "dd",
								Args: []string{"if=/dev/zero", "of=/home/bob/test", "bs=1M", "count=11"},
							}, garden.ProcessIO{})
							Expect(err).ToNot(HaveOccurred())
							Expect(dd.Wait()).ToNot(Equal(0))
						})
					})
				})
			})

			PContext("when multiple containers are created for the same user", func() {
				var container2 garden.Container
				var err error

				BeforeEach(func() {
					limits.Disk.ByteSoft = 50 * 1024 * 1024
					limits.Disk.ByteHard = 50 * 1024 * 1024
					limits.Disk.Scope = garden.DiskLimitScopeTotal
				})

				JustBeforeEach(func() {
					container2, err = gardenClient.Create(garden.ContainerSpec{
						Privileged: privilegedContainer,
						RootFSPath: rootfs,
					})
					Expect(err).ToNot(HaveOccurred())
				})

				AfterEach(func() {
					if container2 != nil {
						Expect(gardenClient.Destroy(container2.Handle())).To(Succeed())
					}
				})

				It("gives each container its own quota", func() {
					process, err := container.Run(garden.ProcessSpec{
						User: "alice",
						Path: "dd",
						Args: []string{"if=/dev/urandom", "of=/home/alice/some-file", "bs=1M", "count=40"},
					}, garden.ProcessIO{})
					Expect(err).ToNot(HaveOccurred())
					Expect(process.Wait()).To(Equal(0))

					process, err = container2.Run(garden.ProcessSpec{
						User: "alice",
						Path: "dd",
						Args: []string{"if=/dev/urandom", "of=/home/alice/some-file", "bs=1M", "count=40"},
					}, garden.ProcessIO{})
					Expect(err).ToNot(HaveOccurred())
					Expect(process.Wait()).To(Equal(0))
				})
			})
		})
	})
})
