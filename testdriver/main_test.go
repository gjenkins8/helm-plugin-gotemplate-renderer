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
	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartloader "helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
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

type testChart struct {
	Chart      *chart.Chart
	TestValues map[string]any
}

var testCharts map[string]testChart

func init() {

	loadGitlabChart := func() *chart.Chart {
		f, err := os.Open("testdata/gitlab-8.9.2.tgz")
		if err != nil {
			panic(err)
		}

		gitLabChart, err := chartloader.LoadArchive(f)
		if err != nil {
			panic(err)
		}

		return gitLabChart
	}

	loadSimpleChart := func() *chart.Chart {

		chrt, err := chartloader.LoadDir("testdata/simple_chart")
		if err != nil {
			panic(err)
		}

		return chrt
	}

	testCharts = map[string]testChart{
		"simple": {
			Chart: loadSimpleChart(),
			TestValues: chartutil.Values{
				"replicas": 3,
			},
		},
		"gitlab": {
			Chart: loadGitlabChart(),
			TestValues: chartutil.Values{
				"global": map[string]any{
					"hosts": map[string]any{
						"domain":     "example.com",
						"externalIP": "10.10.10.10",
					},
				},
				"certmanager-issuer": map[string]any{
					"email": "me@example.com",
				},
			},
		},
	}
}

func makeRenderValues(chrt *chart.Chart, setValues chartutil.Values) (map[string]any, error) {

	options := chartutil.ReleaseOptions{
		Name:      "test-plugin-release",
		Namespace: "default",
		Revision:  1,
		IsInstall: true,
		IsUpgrade: false,
	}

	renderValues, err := chartutil.ToRenderValuesWithSchemaValidation(chrt, setValues, options, nil, false)

	return renderValues, err
}

func init() {
	extism.SetLogLevel(extism.LogLevelDebug)
}

func loadFilePlugin(ctx context.Context, pluginPath string) (*extism.Plugin, error) {
	//pluginBytes, err := os.ReadFile(plugnPath)
	//require.Nil(t, err)

	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{
				Path: pluginPath,
				Name: "gotemplate-renderer",
			},
			//extism.WasmData{
			//	Data: pluginBytes,
			//	Name: "gotemplate-renderer",
			//},
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
		return nil, fmt.Errorf("failed to initialize plugin: %w", err)
	}

	plugin.SetLogger(func(logLevel extism.LogLevel, s string) {
		fmt.Printf("%s %s: %s\n", time.Now().Format(time.RFC3339), logLevel.String(), s)
	})

	return plugin, nil
}

func TestRenderChart(t *testing.T) {

	ctx := context.Background()

	pluginPath := "../gotemplate-renderer.wasm"
	plugin, err := loadFilePlugin(ctx, pluginPath)
	require.Nil(t, err)

	for chartName, testChart := range testCharts {
		t.Run(chartName, func(t *testing.T) {

			err := renderChart(plugin, testChart.Chart, testChart.TestValues)
			assert.Nil(t, err)
		})
	}
	//assert.Fail(t, "fail", "time taken: %s", end.Sub(start))
}

func BenchmarkRenderChart_SimpleChart(b *testing.B) {

	ctx := context.Background()

	pluginPath := "../gotemplate-renderer.wasm"
	plugin, err := loadFilePlugin(ctx, pluginPath)
	if err != nil {
		b.Fail()
	}

	testChart := testCharts["simple"]

	for b.Loop() {
		err := renderChart(plugin, testChart.Chart, testChart.TestValues)
		if err != nil {
			b.Fail()
		}
	}

}

func BenchmarkRenderChart_GitlabChart(b *testing.B) {

	ctx := context.Background()

	pluginPath := "../gotemplate-renderer.wasm"
	plugin, err := loadFilePlugin(ctx, pluginPath)
	if err != nil {
		b.Fail()
	}

	testChart := testCharts["gitlab"]

	for b.Loop() {
		err := renderChart(plugin, testChart.Chart, testChart.TestValues)
		if err != nil {
			b.Fail()
		}
	}

}

func renderChart(plugin *extism.Plugin, chrt *chart.Chart, testValues map[string]any) error {

	renderValues, err := makeRenderValues(chrt, testValues)
	if err != nil {
		return err
	}

	renderValuesJSON, err := json.Marshal(renderValues)
	if err != nil {
		return err
	}

	input := RendererPluginInput{
		Chart:      chrt,
		ValuesJSON: renderValuesJSON,
	}

	inputData, err := json.Marshal(input)
	if err != nil {
		return err
	}

	exitCode, outputData, err := plugin.Call("helm_chart_renderer", inputData)
	if err != nil {
		return err
	}

	if exitCode != 0 {
		return fmt.Errorf("plugin failed: exit code = %d", exitCode)
	}

	output := RendererPluginOutput{}
	if err := json.Unmarshal(outputData, &output); err != nil {
		return err
	}

	return nil
	//fmt.Printf("output: %+v\n", output)
	//assert.Fail(t, "forced failure")
}
