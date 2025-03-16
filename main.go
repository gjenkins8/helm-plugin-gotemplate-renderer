package main

import (
	"encoding/json"
	"fmt"
	"time"

	pdk "github.com/extism/go-pdk"
	"github.com/helm/helm-plugin-gotemplate-renderer/pkg/engine"
	"github.com/helm/helm-plugin-gotemplate-renderer/pkg/release"
	chart "helm.sh/helm/v4/pkg/chart/v2"
)

type Input struct {
	Chart      *chart.Chart `json:"chart"`
	ValuesJSON []byte       `json:"values"`
}

type OutputManifest struct {
	Filename string `json:"filename"`
	Manifest []byte `json:"manifest"`
}

type Output struct {
	Manifests []OutputManifest `json:"manifests"`
}

type ExtismHostFunctions struct {
}

func (e *ExtismHostFunctions) LookupKubernetesResource(apiVersion string, kind string, namespace string, name string) (map[string]interface{}, error) {

	memApiVersion := pdk.AllocateString(apiVersion)
	memKind := pdk.AllocateString(kind)
	memNamespace := pdk.AllocateString(namespace)
	memName := pdk.AllocateString(name)

	resultPtr := extismKubernetesResourceLookup(
		extismPointer(memApiVersion.Offset()),
		extismPointer(memKind.Offset()),
		extismPointer(memNamespace.Offset()),
		extismPointer(memName.Offset()),
	)

	resultMem := pdk.FindMemory(uint64(resultPtr))

	type lookupKubernetesResourceResult struct {
		Error  *string        `json:"error,omitempty"`
		Result map[string]any `json:"result"`
	}

	result := lookupKubernetesResourceResult{}
	if err := json.Unmarshal(resultMem.ReadBytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to deserialize LookupKubernetesResource return json: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("host error: %s", *result.Error)
	}

	return result.Result, nil
}

func (e *ExtismHostFunctions) ResolveHostname(hostname string) string {
	memHostname := pdk.AllocateString(hostname)

	resultPtr := extismResolveHostname(
		extismPointer(memHostname.Offset()),
	)

	resultMem := pdk.FindMemory(uint64(resultPtr))

	return string(resultMem.ReadBytes())
}

func RenderChartTemplates(input Input) (*Output, error) {
	hostFunctions := ExtismHostFunctions{}

	//e, err := renderer.NewEngine(&hostFunctions, renderer.WithDNS(true))
	e, err := engine.NewEngine(&hostFunctions)
	if err != nil {
		return nil, fmt.Errorf("failed to create gotemplate engine: %w", err)
	}

	var vals map[string]any
	if err := json.Unmarshal(input.ValuesJSON, &vals); err != nil {
		return nil, fmt.Errorf("failed to parse input values json: %w", err)
	}

	chrt := input.Chart

	if err := release.ProcessDependencies(chrt, vals); err != nil {
		return nil, fmt.Errorf("chart dependencies processing failed: %w", err)
	}

	renderedManifests, err := e.RenderAllChartTemplates(chrt, vals)
	if err != nil {
		return nil, fmt.Errorf("failed to render chart templates: %w", err)
	}

	result := Output{}

	for filename, data := range renderedManifests {
		result.Manifests = append(result.Manifests, OutputManifest{
			Filename: filename,
			Manifest: []byte(data),
		})
	}
	return &result, nil
}

func RunPlugin() error {

	var input Input
	if err := pdk.InputJSON(&input); err != nil {
		return fmt.Errorf("failed to parse input json: %w", err)
	}

	output, err := RenderChartTemplates(input)
	if err != nil {
		pdk.Log(pdk.LogError, fmt.Sprintf("failed: %s", err.Error()))
		return err
	}

	if err := pdk.OutputJSON(output); err != nil {
		return fmt.Errorf("failed to write output json: %w", err)
	}

	return nil
}

//go:wasmexport helm_chart_renderer
func HelmChartRenderer() uint64 {

	d := time.Duration(0)
	start := time.Now()

	pdk.Log(pdk.LogDebug, "running gotemplate-renderer plugin")

	if err := RunPlugin(); err != nil {
		pdk.Log(pdk.LogError, err.Error())
		pdk.SetError(err)
		return 1
	}

	end := time.Now()
	d += end.Sub(start)
	pdk.Log(pdk.LogInfo, fmt.Sprintf("HelmChartRenderer duration=%s", d))

	return 0
}

func main() {}
