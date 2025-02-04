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
	"github.com/tetratelabs/wazero/api"
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

func makeRenderValues(t *testing.T, chrt *chart.Chart, setValues chartutil.Values) map[string]any {

	options := chartutil.ReleaseOptions{
		Name:      "test-plugin-release",
		Namespace: "default",
		Revision:  1,
		IsInstall: true,
		IsUpgrade: false,
	}
	renderValues, err := chartutil.ToRenderValuesWithSchemaValidation(chrt, setValues, options, nil, false)
	require.Nil(t, err)

	return renderValues
}

func marshalValuesJSON(t *testing.T, renderValues chartutil.Values) []byte {

	data, err := json.Marshal(renderValues)
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
	plugin, err := extism.NewPlugin(ctx, manifest, config, []extism.HostFunction{
		extism.NewHostFunctionWithStack(
			"kubernetes_resource_lookup",
			func(ctx context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
				// TODO error checks
				apiVersion, _ := plugin.ReadString(stack[0])
				kind, _ := plugin.ReadString(stack[1])
				namespace, _ := plugin.ReadString(stack[2])
				name, _ := plugin.ReadString(stack[3])

				_ = plugin.Free(stack[0])
				_ = plugin.Free(stack[1])
				_ = plugin.Free(stack[2])
				_ = plugin.Free(stack[3])

				fmt.Printf("received unimplemented lookup: %q %q %q %q\n", apiVersion, kind, namespace, name)

				type lookupKubernetesResourceResult struct {
					Error  *string        `json:"error,omitempty"`
					Result map[string]any `json:"result"`
				}

				result := lookupKubernetesResourceResult{}
				resultData, _ := json.Marshal(&result)

				resultBytes, _ := plugin.WriteBytes(resultData)
				stack[0] = resultBytes
			},
			[]api.ValueType{
				api.ValueTypeI64, // apiGroup
				api.ValueTypeI64, // kind
				api.ValueTypeI64, // name
				api.ValueTypeI64, // namespace
			},
			[]api.ValueType{
				api.ValueTypeI64,
			},
		),
		extism.NewHostFunctionWithStack(
			"resolve_hostname",
			func(ctx context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			},
			[]api.ValueType{
				api.ValueTypeI64, // apiGroup
			},
			[]api.ValueType{
				api.ValueTypeI64,
			},
		),
	})
	if err != nil {
		fmt.Printf("Failed to initialize plugin: %v\n", err)
		os.Exit(1)
	}

	plugin.SetLogger(func(logLevel extism.LogLevel, s string) {
		fmt.Printf("%s %s: %s\n", time.Now().Format(time.RFC3339), logLevel.String(), s)
	})

	testValues := chartutil.Values{
		"replicas": 3,
	}

	chrt := loadChartDir(t, "testdata/testchart")
	input := RendererPluginInput{
		Chart:      chrt,
		ValuesJSON: marshalValuesJSON(t, makeRenderValues(t, chrt, testValues)),
	}

	inputData, err := json.Marshal(input)
	require.Nil(t, err)
	require.NotEmpty(t, inputData)

	exitCode, outputData, err := plugin.Call("helm_chart_renderer", inputData)
	require.Nil(t, err, "exitCode=%d plugin error=%s", exitCode, plugin.GetError())
	assert.Equal(t, uint32(0), exitCode)

	output := RendererPluginOutput{}
	{
		err := json.Unmarshal(outputData, &output)
		assert.Nil(t, err)
	}

	fmt.Printf("output: %+v\n", output)
	assert.Fail(t, "forced failure")
}
