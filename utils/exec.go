package utils

import (
	"bytes"
	"encoding/json"
	"log"
	"os/exec"
)

func ExecuteScript(script string, params interface{}, l *log.Logger) (interface{}, error) {
	if script == "" {
		return nil, nil
	}
	serializedParams, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(script, string(serializedParams))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	l.Printf("%s stderr: \n%s\n", script, stderrOutput)
	if err != nil {
		return nil, err
	}
	output := stdout.Bytes()
	stderrOutput := string(stderr.Bytes())
	if output != nil && string(output) != "" {
		var res interface{}
		err = json.Unmarshal(output, &res)
		return res, err
	}
	return nil, nil
}
