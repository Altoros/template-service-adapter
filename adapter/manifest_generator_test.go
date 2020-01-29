package adapter_test

import (
	"fmt"
	"log"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"gopkg.in/yaml.v2"

	. "github.com/Altoros/template-service-adapter/adapter"
	"github.com/Altoros/template-service-adapter/config"
)

type genTest struct {
	manifesetTmpl string
	expectedRes   string
	params        serviceadapter.GenerateManifestParams
}

var tests = []genTest{
	{
		`{{$password := genPassword}}
name: {{.deployment.DeploymentName}}

{{getReleasesBlock}}

{{getStemcellsBlock}}

{{getUpdateBlock}}

instance_groups:
{{getInstanceGroup "redis_leader"}}
  jobs:
  - name: redis
    release: redis
    properties:
      redis:
        password: {{$password}}
{{if .params.use_slave_instances}}
{{getInstanceGroup "redis_slave"}}
  jobs:
  - name: redis
    release: redis
    properties:
      redis:
        master:
        password: {{$password}}
{{end}}`,
		`name: redis

releases:
- name: redis
  version: 123

stemcells:
- alias: stemcell_0
  os: ubuntu-trusty
  version: 123

update:
  canaries: 1
  max_in_flight: 1
  canary_watch_time: 30000-240000
  update_watch_time: 30000-240000

instance_groups:
- instances: 1
  name: redis_leader
  vm_type: medium
  stemcell: stemcell_0
  azs: [z1]
  networks:
  - name: default
  persistent_disk_type: large
  jobs:
  - name: redis
    release: redis
    properties:
      redis:
        password: password
- instances: 2
  name: redis_slave
  vm_type: medium
  stemcell: stemcell_0
  azs: [z1]
  networks:
  - name: default
  persistent_disk_type: large
  jobs:
  - name: redis
    release: redis
    properties:
      redis:
        master:
        password: password
`,
		serviceadapter.GenerateManifestParams{
			serviceadapter.ServiceDeployment{
				DeploymentName: "redis",
				Releases: serviceadapter.ServiceReleases{
					serviceadapter.ServiceRelease{
						Name:    "redis",
						Version: "123",
						Jobs:    []string{"redis", "redis_slave"},
					},
				},
				Stemcells: []serviceadapter.Stemcell{
					serviceadapter.Stemcell{
						OS:      "ubuntu-trusty",
						Version: "123",
					},
				},
			},
			serviceadapter.Plan{
				InstanceGroups: []serviceadapter.InstanceGroup{
					{
						Name:               "redis_leader",
						VMType:             "medium",
						PersistentDiskType: "large",
						Instances:          1,
						Networks:           []string{"default"},
						AZs:                []string{"z1"},
					},
					{
						Name:               "redis_slave",
						VMType:             "medium",
						PersistentDiskType: "large",
						Instances:          2,
						Networks:           []string{"default"},
						AZs:                []string{"z1"},
					},
				},
			},
			serviceadapter.RequestParameters{"use_slave_instances": true}, nil, nil, nil, nil,
		},
	},
}

var _ = Describe("Generate manifest", func() {
	GenPassword = func() (string, error) {
		return "password", nil
	}
	for i, test := range tests {
		It(fmt.Sprintf("Test case %d", i), func() {
			m := ManifestGenerator{
				Config: &config.Config{ManifestTemplates: map[string]string{"some-plan": test.manifesetTmpl}},
				Logger: log.New(os.Stderr, "[template-service-adapter] ", log.LstdFlags),
			}
			if test.params.Plan.Properties == nil {
				test.params.Plan.Properties = serviceadapter.Properties{}
			}
			test.params.Plan.Properties["name"] = "some-plan"
			manifest, err := m.GenerateManifest(test.params)
			Expect(err).ToNot(HaveOccurred())
			var expectedManifest bosh.BoshManifest
			err = yaml.Unmarshal([]byte(test.expectedRes), &expectedManifest)
			Expect(err).ToNot(HaveOccurred())
			manifestStr, err := yaml.Marshal(manifest.Manifest)
			Expect(err).ToNot(HaveOccurred())
			expectedManifestStr, err := yaml.Marshal(expectedManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(manifestStr)).To(Equal(string(expectedManifestStr)))
		})
	}

})
