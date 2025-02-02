package main_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	extism "github.com/extism/go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"helm.sh/helm/v4/pkg/chart"
	chartloader "helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/chartutil"
)

type RendererPluginInput struct {
	Chart      *chart.Chart `json:"chart"`
	ValuesJSON []byte       `json:"values"`
}

type RendererPluginOutputManifest struct {
	Filename string `json:"filename"`
	Manifest []byte `json:"manifest"`
}

type RendererPluginOutput struct {
	Manifests []RendererPluginOutputManifest `json:"manifests"`
}

func loadChartDir(t *testing.T, path string) *chart.Chart {
	chart, err := chartloader.LoadDir(path)
	require.Nil(t, err)

	return chart
}

func marshalValuesJSON(t *testing.T, values chartutil.Values) []byte {
	data, err := json.Marshal(values)
	require.Nil(t, err)

	return data
}

func init() {
	extism.SetLogLevel(extism.LogLevelDebug)
}

func TestRenderChart(t *testing.T) {

	pluginBytes, err := os.ReadFile("../gotemplate-renderer.wasm")
	require.Nil(t, err)

	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmData{
				Data: pluginBytes,
				Name: "gotemplate-renderer",
			},
		},
		Memory: &extism.ManifestMemory{
			MaxPages: 65535,
			//MaxHttpResponseBytes: 1024 * 1024 * 10,
			//MaxVarBytes:          1024 * 1024 * 10,
		},
		Config: map[string]string{},
		//AllowedHosts: []string{"ghcr.io"},
		AllowedPaths: map[string]string{},
		Timeout:      0,
	}

	ctx := context.Background()
	config := extism.PluginConfig{
		ModuleConfig:  wazero.NewModuleConfig().WithSysWalltime(),
		RuntimeConfig: wazero.NewRuntimeConfig().WithCloseOnContextDone(false),
		EnableWasi:    true,
		//EnableHttpResponseHeaders: true,
		//ObserveAdapter: ,
		//ObserveOptions: &observe.Options{},
	}
	plugin, err := extism.NewPlugin(ctx, manifest, config, []extism.HostFunction{})
	if err != nil {
		fmt.Printf("Failed to initialize plugin: %v\n", err)
		os.Exit(1)
	}

	plugin.SetLogger(func(logLevel extism.LogLevel, s string) {
		fmt.Printf("%s %s: %s\n", time.Now().Format(time.RFC3339), logLevel.String(), s)
	})

	fmt.Println("executing")
	testValues := chartutil.Values{
		"replicas": 3,
	}

	input := RendererPluginInput{
		Chart:      loadChartDir(t, "testdata/testchart"),
		ValuesJSON: marshalValuesJSON(t, testValues),
	}

	inputData, err := json.Marshal(input)
	require.Nil(t, err)
	require.NotEmpty(t, inputData)

	exitCode, outputData, err := plugin.Call("_start", inputData)
	require.Nil(t, err, "exitCode=%d plugin error=%s", exitCode, plugin.GetError())
	assert.Equal(t, uint32(0), exitCode)

	fmt.Printf("output data: %q\n", outputData)

	output := RendererPluginOutput{}
	{
		err := json.Unmarshal(outputData, &output)
		assert.Nil(t, err)
	}

	fmt.Printf("output: %+v\n", output)
	assert.Fail(t, "stonk")
}
