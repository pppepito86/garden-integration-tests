package garden_integration_tests_test

import (
	"io"
	"runtime/debug"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Process", func() {
	Describe("signalling", func() {
		It("a process can be sent SIGTERM immediately after having been started", func() {
			stdout := gbytes.NewBuffer()

			process, err := container.Run(garden.ProcessSpec{
				User: "root",
				Path: "sh",
				Args: []string{
					"-c",
					`
                sleep 10
                exit 12
                `,
				},
			}, garden.ProcessIO{
				Stdout: stdout,
			})
			Expect(err).ToNot(HaveOccurred())

			err = process.Signal(garden.SignalTerminate)
			Expect(err).ToNot(HaveOccurred())
			Expect(process.Wait()).NotTo(Equal(12))
		})
	})

	Describe("environment", func() {
		It("should apply the specified environment", func() {
			stdout := gbytes.NewBuffer()

			process, err := container.Run(garden.ProcessSpec{
				Path: "env",
				Env: []string{
					"TEST=hello",
					"FRUIT=banana",
				},
			}, garden.ProcessIO{
				Stdout: stdout,
			})
			Expect(err).ToNot(HaveOccurred())
			exitCode, err := process.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			Expect(stdout.Contents()).To(ContainSubstring("TEST=hello\nFRUIT=banana"))
		})

		Context("when the container has container spec environment specified", func() {
			BeforeEach(func() {
				env = []string{
					"CONTAINER_ENV=1",
					"TEST=hi",
				}
			})

			It("should apply the merged environment variables", func() {
				stdout := gbytes.NewBuffer()

				process, err := container.Run(garden.ProcessSpec{
					Path: "env",
					Env: []string{
						"TEST=hello",
						"FRUIT=banana",
					},
				}, garden.ProcessIO{
					Stdout: stdout,
				})
				Expect(err).ToNot(HaveOccurred())
				exitCode, err := process.Wait()
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCode).To(Equal(0))

				Expect(stdout.Contents()).To(ContainSubstring("CONTAINER_ENV=1\nTEST=hello\nFRUIT=banana"))
			})
		})
	})

	Describe("wait", func() {
		BeforeEach(func() {
			rootfs = "docker:///ubuntu"
		})

		It("does not block in Wait() when all children of the process have exited", func() {
			stderr := gbytes.NewBuffer()
			process, err := container.Run(garden.ProcessSpec{
				User: "root",
				Path: "/bin/bash",
				Args: []string{"-c", `

				  cleanup ()
				  {
				  	kill $child_pid
				  	exit 42
				  }

				  trap cleanup TERM
				  # Use stderr to avoid buffering in the shell.
				  echo trapping >&2

				  sleep 1000 &
				  child_pid=$!
				  wait
				`},
			}, garden.ProcessIO{Stderr: stderr})
			Expect(err).NotTo(HaveOccurred())

			exitChan := make(chan int)
			go func(p garden.Process, exited chan<- int) {
				GinkgoRecover()
				status, waitErr := p.Wait()
				Expect(waitErr).NotTo(HaveOccurred())
				exited <- status
			}(process, exitChan)

			Eventually(stderr).Should(gbytes.Say("trapping"))

			Expect(process.Signal(garden.SignalTerminate)).To(Succeed())

			select {
			case status := <-exitChan:
				Expect(status).To(Equal(42))
			case <-time.After(time.Second * 10):
				debug.PrintStack()
				Fail("timed out!")
			}
		})

		It("does not block in Wait() when a child of the process has not exited", func() {
			buffer := gbytes.NewBuffer()
			process, err := container.Run(garden.ProcessSpec{
				User: "root",
				Path: "/bin/bash",
				Args: []string{"-c", `
					cleanup ()
					{
						exit 42
					}

					trap cleanup TERM
					sleep 100000 &
					disown -h
 -
					echo trapping
					wait
				`},
			}, garden.ProcessIO{Stdout: buffer})
			Expect(err).NotTo(HaveOccurred())

			exitChan := make(chan int)
			go func(p garden.Process, exited chan<- int) {
				GinkgoRecover()
				status, waitErr := p.Wait()
				Expect(waitErr).NotTo(HaveOccurred())
				exited <- status
			}(process, exitChan)

			Eventually(buffer).Should(gbytes.Say("trapping"))

			Expect(process.Signal(garden.SignalTerminate)).To(Succeed())
			select {
			case status := <-exitChan:
				Expect(status).To(Equal(42))
			case <-time.After(time.Second * 10):
				Fail("Wait should not block when a child has not exited")
			}
		})
	})

	Describe("working directory", func() {
		JustBeforeEach(func() {
			createUser(container, "alice")
		})

		Context("when user has access to working directory", func() {
			Context("when working directory exists", func() {
				It("spawns the process", func() {
					stdout := gbytes.NewBuffer()
					process, err := container.Run(garden.ProcessSpec{
						User: "alice",
						Dir:  "/home/alice",
						Path: "pwd",
					}, garden.ProcessIO{
						Stdout: stdout,
						Stderr: GinkgoWriter,
					})
					Expect(err).ToNot(HaveOccurred())

					Expect(process.Wait()).To(Equal(0))
					Eventually(stdout).Should(gbytes.Say("/home/alice"))
				})
			})

			Context("when working directory does not exist", func() {
				It("spawns the process", func() {
					stdout := gbytes.NewBuffer()
					process, err := container.Run(garden.ProcessSpec{
						User: "alice",
						Dir:  "/home/alice/nonexistent",
						Path: "pwd",
					}, garden.ProcessIO{
						Stdout: stdout,
						Stderr: GinkgoWriter,
					})
					Expect(err).ToNot(HaveOccurred())

					Expect(process.Wait()).To(Equal(0))
					Eventually(stdout).Should(gbytes.Say("/home/alice/nonexistent"))
				})
			})
		})

		Context("when user does not have access to working directory", func() {
			JustBeforeEach(func() {
				process, err := container.Run(garden.ProcessSpec{
					User: "alice",
					Path: "sh",
					Args: []string{"-c", "mkdir -p /home/alice/nopermissions && chmod 0555 /home/alice/nopermissions"},
				}, garden.ProcessIO{
					Stdout: GinkgoWriter,
					Stderr: GinkgoWriter,
				})
				Expect(err).ToNot(HaveOccurred())
				exitStatus, err := process.Wait()
				Expect(exitStatus).To(Equal(0))
			})

			Context("when working directory does exist", func() {
				It("returns an error", func() {
					stderr := gbytes.NewBuffer()
					process, err := container.Run(garden.ProcessSpec{
						User: "alice",
						Dir:  "/home/alice/nopermissions",
						Path: "touch",
						Args: []string{"test.txt"},
					}, garden.ProcessIO{
						Stdout: GinkgoWriter,
						Stderr: io.MultiWriter(GinkgoWriter, stderr),
					})
					Expect(err).ToNot(HaveOccurred())
					exitStatus, err := process.Wait()
					Expect(exitStatus).ToNot(Equal(0))
					Expect(stderr).To(gbytes.Say("Permission denied"))
				})
			})

			Context("when working directory does not exist", func() {
				It("should create the working directory, and succeed", func() {
					stderr := gbytes.NewBuffer()
					process, err := container.Run(garden.ProcessSpec{
						User: "alice",
						Dir:  "/home/alice/nopermissions/nonexistent",
						Path: "touch",
						Args: []string{"test.txt"},
					}, garden.ProcessIO{
						Stdout: GinkgoWriter,
						Stderr: stderr,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(process.Wait()).To(Equal(0))
				})
			})
		})

		Context("when the user does not specify the working directory", func() {
			It("should have the user home directory in the output", func() {
				out := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{
					User: "alice",
					Path: "pwd",
				}, garden.ProcessIO{
					Stdout: out,
					Stderr: GinkgoWriter,
				})
				Expect(err).NotTo(HaveOccurred())

				exitStatus, err := process.Wait()
				Expect(exitStatus).To(Equal(0))
				Expect(out).To(gbytes.Say("/home/alice"))
			})
		})
	})
})
