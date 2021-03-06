package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"text/template"

	"github.com/cppforlife/go-patch/patch"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"

	"github.com/Altoros/template-service-adapter/config"
	"github.com/Altoros/template-service-adapter/utils"
)

type Binder struct {
	Config         *config.Config
	Logger         *log.Logger
	manifestYaml   interface{}
	deploymentYaml interface{}
}

func (b Binder) CreateBinding(bindingParams serviceadapter.CreateBindingParams) (serviceadapter.Binding, error) {
	b.Logger.Printf("Creating binding. id: %s", bindingParams.BindingID)
	var err error
	b.manifestYaml, err = utils.ConvertToYamlCompatibleObject(bindingParams.Manifest)
	if err != nil {
		return serviceadapter.Binding{}, err
	}
	b.deploymentYaml, err = utils.ConvertToYamlCompatibleObject(bindingParams.DeploymentTopology)
	if err != nil {
		return serviceadapter.Binding{}, err
	}

	tmpl := template.New("binder-template")
	tmpl.Funcs(template.FuncMap{"getFromManifest": b.getTemplateFunc(b.manifestYaml)})
	tmpl.Funcs(template.FuncMap{"getFromDeployment": b.getTemplateFunc(b.deploymentYaml)})
	tmpl, err = tmpl.Parse(b.Config.BinderTemplate)
	if err != nil {
		return serviceadapter.Binding{}, err
	}
	params := map[string]interface{}{}
	params["deployment"] = bindingParams.DeploymentTopology
	manifest := utils.MakeJsonCompatible(bindingParams.Manifest)
	params["manifest"] = manifest
	executionRes, err := utils.ExecuteScript(b.Config.PreBinding, params, b.Logger)
	if err != nil {
		return serviceadapter.Binding{}, err
	}
	params["generatedParams"] = executionRes
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, params)
	if err != nil {
		return serviceadapter.Binding{}, err
	}
	bindingStr := buf.String()
	b.Logger.Printf("Binding: \n%s\n", bindingStr)

	binding := serviceadapter.Binding{}
	err = json.Unmarshal([]byte(bindingStr), &binding)
	if err != nil {
		return serviceadapter.Binding{}, err
	}
	params["binding"] = binding
	_, err = utils.ExecuteScript(b.Config.PostBinding, params, b.Logger)
	if err != nil {
		return serviceadapter.Binding{}, err
	}
	return binding, nil
}

func (b Binder) DeleteBinding(params serviceadapter.DeleteBindingParams) error {
	return nil
}

func (b Binder) getTemplateFunc(doc interface{}) func(string) (string, error) {
	return func(path string) (string, error) {
		p, err := patch.NewPointerFromString(path)
		if err != nil {
			return "", err
		}
		res, err := patch.FindOp{Path: p}.Apply(doc)
		return fmt.Sprintf("%v", res), err
	}
}
