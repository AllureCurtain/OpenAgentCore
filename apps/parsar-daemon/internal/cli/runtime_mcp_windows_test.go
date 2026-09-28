//go:build windows

package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestWindowsRuntimeMCPPackageManagers(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("native test requires installed Node.js", err)
	}
	workspace := filepath.Join(t.TempDir(), "local stdio project")
	bin := filepath.Join(workspace, "node_modules", ".bin")
	pkg := filepath.Join(workspace, "node_modules", "oac-stdio-fixture")
	if err = os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(pkg, 0700); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(pkg, "fixture.cjs")
	files := map[string]string{
		filepath.Join(workspace, "package.json"):    `{"name":"offline-test","version":"1.0.0","private":true}`,
		filepath.Join(pkg, "package.json"):          `{"name":"oac-stdio-fixture","version":"1.0.0","bin":{"oac-stdio-fixture":"fixture.cjs"}}`,
		fixture:                                     `let input="";process.stdin.setEncoding("utf8");process.stdin.on("data",v=>input+=v);process.stdin.on("end",()=>process.stdout.write(JSON.stringify({input,cwd:process.cwd(),value:process.env.OAC_MCP_FIXTURE_VALUE,args:process.argv.slice(2)})))`,
		filepath.Join(bin, "oac-stdio-fixture.cmd"): "@\"" + node + "\" \"" + fixture + "\" %*\r\n",
	}
	for path, body := range files {
		if err = os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Each invocation uses only this installed local bin, with an empty offline cache.
	environment := append(os.Environ(), "npm_config_offline=true", "npm_config_yes=false", "npm_config_cache="+t.TempDir(), "OAC_MCP_FIXTURE_VALUE=selected")
	for _, name := range []string{"npm", "npx"} {
		absolute, err := exec.LookPath(name + ".cmd")
		if err != nil {
			t.Fatal("native test requires installed npm/npx", err)
		}
		for _, command := range []string{name, name + ".cmd", absolute} {
			t.Run(command, func(t *testing.T) {
				args := []string{command}
				if name == "npm" {
					args = append(args, "exec")
				}
				args = append(args, "--offline", "--yes=false", "--", "oac-stdio-fixture", "two words")
				checkWindowsMCPStdio(t, mcpInvocation{command: command, args: args, cwd: workspace, env: environment}, []string{"two words"})
			})
		}
	}
	checkWindowsMCPStdio(t, mcpInvocation{command: node, args: []string{node, fixture, "ordinary exe", "literal & %PATH%"}, cwd: workspace, env: environment}, []string{"ordinary exe", "literal & %PATH%"})
}

func checkWindowsMCPStdio(t *testing.T, invocation mcpInvocation, wantArgs []string) {
	t.Helper()
	input, err := os.CreateTemp(t.TempDir(), "input")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	if _, err = input.WriteString("local stdio payload\n"); err != nil {
		t.Fatal(err)
	}
	if _, err = input.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	stdin, stdout, oldErr := os.Stdin, os.Stdout, os.Stderr
	os.Stdin, os.Stdout, os.Stderr = input, output, stderr
	defer func() { os.Stdin, os.Stdout, os.Stderr = stdin, stdout, oldErr }()
	if err = execRuntimeMCP(invocation); err != nil {
		detail, _ := os.ReadFile(stderr.Name())
		t.Fatalf("native local MCP failed: %v: %s", err, detail)
	}
	raw, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Input, CWD, Value string
		Args              []string
	}
	if json.Unmarshal(raw, &got) != nil || got.Input != "local stdio payload\n" || filepath.Clean(got.CWD) != filepath.Clean(invocation.cwd) || got.Value != "selected" || !slices.Equal(got.Args, wantArgs) {
		t.Fatalf("stdio invocation changed: %s", raw)
	}
}
