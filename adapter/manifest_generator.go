package adapter

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strconv"
	"text/template"

	text "github.com/kr/text"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	yaml "gopkg.in/yaml.v2"

	"github.com/Altoros/template-service-adapter/config"
	"github.com/Altoros/template-service-adapter/utils"
)

type ManifestGenerator struct {
	Config *config.Config
	Logger *log.Logger
}

var GenPassword = func() (string, error) {
	u, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (m ManifestGenerator) GenerateManifest(adapterParams serviceadapter.GenerateManifestParams) (serviceadapter.GenerateManifestOutput, error) {
	var planName string
	if name, ok := adapterParams.Plan.Properties["name"]; ok {
		planName = name.(string)
	} else {
		return serviceadapter.GenerateManifestOutput{}, errors.New("plan don't have a name property")
	}
	m.Logger.Printf("Generating manifest. plan: %s\n", planName)
	tmpl := template.New("manifest-template")

	tmpl.Funcs(template.FuncMap{"genPassword": GenPassword})
	tmpl.Funcs(template.FuncMap{"getInstanceGroup": func(name string) (string, error) {
		for _, g := range adapterParams.Plan.InstanceGroups {
			if g.Name == name {
				res, err := yaml.Marshal([]bosh.InstanceGroup{
					{
						Name:               g.Name,
						Instances:          g.Instances,
						VMType:             g.VMType,
						VMExtensions:       g.VMExtensions,
						PersistentDiskType: g.PersistentDiskType,
						Stemcell:           "stemcell_0",
						Networks:           m.mapNetworksToBoshNetworks(g.Networks),
						AZs:                g.AZs,
						Lifecycle:          g.Lifecycle,
					},
				})
				return string(res), err
			}
		}
		return "", fmt.Errorf("No instance group found with name %s", name)
	}})
	tmpl.Funcs(template.FuncMap{"getReleasesBlock": m.printYamlFunc("releases", m.generateReleasesBlock(adapterParams.ServiceDeployment.Releases), "")})
	tmpl.Funcs(template.FuncMap{"getStemcellsBlock": m.printYamlFunc("stemcells", m.generateStemcellsBlock(adapterParams.ServiceDeployment.Stemcells), "")})
	tmpl.Funcs(template.FuncMap{"getUpdateBlock": m.printYamlFunc("update", m.generateUpdateBlock(adapterParams.Plan.Update), "  ")})
	var planTemplate string
	var ok bool
	if planTemplate, ok = m.Config.ManifestTemplates[planName]; !ok {
		return serviceadapter.GenerateManifestOutput{}, fmt.Errorf("Can't find plan template for name %s", planName)
	}
	_, err := tmpl.Parse(planTemplate)
	if err != nil {
		return serviceadapter.GenerateManifestOutput{}, err
	}
	b := &bytes.Buffer{}
	params := map[string]interface{}{}
	params["params"] = adapterParams.RequestParams
	params["deployment"] = adapterParams.ServiceDeployment
	params["plan"] = adapterParams.Plan
	params["previousPlan"] = adapterParams.PreviousPlan
	executionRes, err := utils.ExecuteScript(m.Config.PreManifestGeneration, params, m.Logger)
	if err != nil {
		return serviceadapter.GenerateManifestOutput{}, err
	}
	params["generatedParams"] = executionRes
	err = tmpl.Execute(b, params)
	if err != nil {
		return serviceadapter.GenerateManifestOutput{}, err
	}

	manifest := bosh.BoshManifest{}
	manifestStr := b.String()
	m.Logger.Printf("Manifest: \n%s\n", manifestStr)

	err = yaml.Unmarshal([]byte(manifestStr), &manifest)
	if err != nil {
		return serviceadapter.GenerateManifestOutput{}, err
	}
	manifest = utils.MakeJsonCompatible(manifest)
	params["manifest"] = manifest
	_, err = utils.ExecuteScript(m.Config.PostManifestGeneration, params, m.Logger)
	return serviceadapter.GenerateManifestOutput{
		Manifest:          manifest,
		ODBManagedSecrets: serviceadapter.ODBManagedSecrets{},
	}, nil
}

func (m ManifestGenerator) printYamlFunc(blockName string, obj interface{}, indent string) func() (string, error) {
	return func() (string, error) {
		res, err := yaml.Marshal(obj)
		t := text.Indent(string(res), indent)
		return blockName + ":\n" + t, err
	}
}

func (m ManifestGenerator) generateUpdateBlock(update *serviceadapter.Update) bosh.Update {
	if update != nil {
		return bosh.Update{
			Canaries:        update.Canaries,
			MaxInFlight:     update.MaxInFlight,
			CanaryWatchTime: update.CanaryWatchTime,
			UpdateWatchTime: update.UpdateWatchTime,
			Serial:          update.Serial,
		}
	} else {
		return bosh.Update{
			Canaries:        1,
			CanaryWatchTime: "30000-240000",
			UpdateWatchTime: "30000-240000",
			MaxInFlight:     1,
		}
	}
}

func (m ManifestGenerator) generateStemcellsBlock(stemcells []serviceadapter.Stemcell) []bosh.Stemcell {
	res := []bosh.Stemcell{}
	for i, stemcell := range stemcells {
		res = append(res, bosh.Stemcell{
			Alias:   "stemcell_" + strconv.Itoa(i),
			OS:      stemcell.OS,
			Version: stemcell.Version,
		})
	}
	return res
}

func (m ManifestGenerator) generateReleasesBlock(releases serviceadapter.ServiceReleases) []bosh.Release {
	res := []bosh.Release{}
	for _, release := range releases {
		res = append(res, bosh.Release{
			Name:    release.Name,
			Version: release.Version,
		})
	}
	return res
}

func (m ManifestGenerator) mapNetworksToBoshNetworks(networks []string) []bosh.Network {
	boshNetworks := []bosh.Network{}
	for _, network := range networks {
		boshNetworks = append(boshNetworks, bosh.Network{Name: network})
	}
	return boshNetworks
}
