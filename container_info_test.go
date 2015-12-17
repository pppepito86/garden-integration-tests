package garden_integration_tests_test

import (
	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container information", func() {
	Describe("for a single container", func() {
		BeforeEach(func() {
			properties = garden.Properties{
				"foo": "bar",
				"a":   "b",
			}
		})

		PDescribe("info for one container", func() {
			It("includes the properties", func() {
				info, err := container.Info()
				Expect(err).ToNot(HaveOccurred())

				Expect(info.Properties["foo"]).To(Equal("bar"))
				Expect(info.Properties["a"]).To(Equal("b"))

				Expect(info.Properties).To(HaveLen(2))
			})
		})

		PDescribe("getting container metrics without getting info", func() {
			It("can list metrics", func() {
				metrics, err := container.Metrics()
				Expect(err).ToNot(HaveOccurred())

				Expect(metrics).To(BeAssignableToTypeOf(garden.Metrics{}))
				Expect(metrics).ToNot(Equal(garden.Metrics{}))
			})
		})

		Describe("getting container properties without getting info", func() {
			It("can list properties", func() {
				err := container.SetProperty("bar", "baz")

				value, err := container.Properties()
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(HaveKeyWithValue("foo", "bar"))
				Expect(value).To(HaveKeyWithValue("bar", "baz"))
			})
		})

		PDescribe("updating container properties", func() {
			It("can CRUD", func() {
				value, err := container.Property("foo")
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal("bar"))

				err = container.SetProperty("foo", "baz")
				Expect(err).ToNot(HaveOccurred())

				err = container.RemoveProperty("a")
				Expect(err).ToNot(HaveOccurred())

				info, err := container.Info()
				Expect(err).ToNot(HaveOccurred())

				Expect(info.Properties).To(Equal(garden.Properties{
					"foo": "baz",
				}))
			})
		})

		Describe("listing container info", func() {
			var undesiredHandles []string

			BeforeEach(func() {
				undesiredContainer, err := gardenClient.Create(garden.ContainerSpec{
					Properties: garden.Properties{
						"foo": "baz",
						"a":   "b",
					},
				})

				Expect(err).ToNot(HaveOccurred())

				undesiredHandles = append(undesiredHandles, undesiredContainer.Handle())

				undesiredContainer, err = gardenClient.Create(garden.ContainerSpec{
					Properties: garden.Properties{
						"baz": "bar",
						"a":   "b",
					},
				})

				Expect(err).ToNot(HaveOccurred())

				undesiredHandles = append(undesiredHandles, undesiredContainer.Handle())
			})

			AfterEach(func() {
				for _, handle := range undesiredHandles {
					Expect(gardenClient.Destroy(handle)).To(Succeed())
				}
			})

			It("can filter by property", func() {
				containers, err := gardenClient.Containers(garden.Properties{"foo": "bar"})
				Expect(err).ToNot(HaveOccurred())

				Expect(containers).To(HaveLen(1))
				Expect(containers[0].Handle()).To(Equal(container.Handle()))

				containers, err = gardenClient.Containers(garden.Properties{"matthew": "mcconaughey"})
				Expect(err).ToNot(HaveOccurred())

				Expect(containers).To(BeEmpty())
			})
		})
	})

	PDescribe("multiple containers", func() {
		var extraContainer garden.Container

		BeforeEach(func() {
			var err error
			extraContainer, err = gardenClient.Create(garden.ContainerSpec{})
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			if extraContainer != nil {
				Expect(gardenClient.Destroy(extraContainer.Handle())).To(Succeed())
			}
		})

		It("should list all containers", func() {
			containers, err := gardenClient.Containers(garden.Properties{})
			Expect(err).ToNot(HaveOccurred())

			handles := []string{}
			for _, c := range containers {
				handles = append(handles, c.Handle())
			}

			Expect(handles).To(ConsistOf(container.Handle(), extraContainer.Handle()))
		})
	})

})
