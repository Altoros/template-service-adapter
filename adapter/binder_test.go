package adapter_test

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"

	. "github.com/Altoros/template-service-adapter/adapter"
	"github.com/Altoros/template-service-adapter/config"
)

type binderTest struct {
	bindingTmpl string
	expectedRes string
	params      serviceadapter.CreateBindingParams
}

var binderTests = []binderTest{
	{
		`{"credentials": {
"host": "{{ getFromDeployment "/redis_leader/0"}}" ,
"password": "{{ getFromManifest "/instance_groups/name=redis_leader/jobs/name=redis/properties/redis/password"}}",
"port": 58301
}}`,
		`{
"host": "127.0.0.1",
"password": "password",
"port": 58301
}`,
		serviceadapter.CreateBindingParams{
			"",
			bosh.BoshVMs{"redis_leader": []string{"127.0.0.1"}},
			bosh.BoshManifest{
				InstanceGroups: []bosh.InstanceGroup{
					{
						Name: "redis_leader",
						Jobs: []bosh.Job{
							{
								Name: "redis",
								Properties: map[string]interface{}{
									"redis": map[string]interface{}{
										"password": "password",
									},
								},
							},
						},
					},
				},
			},
			nil,
			nil,
			nil,
		},
	},
}

var _ = Describe("Bind service", func() {
	for i, test := range binderTests {
		It(fmt.Sprintf("Test case %d", i), func() {
			b := Binder{
				Config: &config.Config{BinderTemplate: test.bindingTmpl},
				Logger: log.New(os.Stderr, "[template-service-adapter] ", log.LstdFlags),
			}
			binding, err := b.CreateBinding(test.params)
			Expect(err).ToNot(HaveOccurred())
			var expectedCredentials map[string]interface{}
			err = json.Unmarshal([]byte(test.expectedRes), &expectedCredentials)
			expectedBinding := serviceadapter.Binding{Credentials: expectedCredentials}
			Expect(err).ToNot(HaveOccurred())
			bindingStr, err := json.Marshal(binding)
			Expect(err).ToNot(HaveOccurred())
			expectedBindingStr, err := json.Marshal(expectedBinding)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(bindingStr)).To(Equal(string(expectedBindingStr)))
		})
	}

})
