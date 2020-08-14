/*
   Copyright 2020 Docker, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/icmd"

	. "github.com/docker/api/tests/framework"
)

var binDir string

func TestMain(m *testing.M) {
	p, cleanup, err := SetupExistingCLI()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	binDir = p
	exitCode := m.Run()
	cleanup()
	os.Exit(exitCode)
}

func TestComposeNotImplemented(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)
	res := c.RunDockerCmd("context", "show")
	res.Assert(t, icmd.Expected{Out: "default"})
	res = c.RunDockerCmd("compose", "up")
	res.Assert(t, icmd.Expected{
		ExitCode: 1,
		Err:      `compose command not supported on context type "moby": not implemented`,
	})
}

func TestContextDefault(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)

	t.Run("show", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("context", "show")
		res.Assert(t, icmd.Expected{Out: "default"})
	})

	t.Run("ls", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("context", "ls")
		res.Assert(t, icmd.Success)
		golden.Assert(t, res.Stdout(), GoldenFile("ls-out-default"))
	})

	t.Run("inspect", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("context", "inspect", "default")
		res.Assert(t, icmd.Expected{Out: `"Name": "default"`})
	})

	t.Run("inspect current", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("context", "inspect")
		res.Assert(t, icmd.Expected{Out: `"Name": "default"`})
	})
}

func TestContextCreateDocker(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)
	res := c.RunDockerCmd("context", "create", "test-docker", "--from", "default")
	res.Assert(t, icmd.Expected{Out: "test-docker"})

	t.Run("ls", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("context", "ls")
		res.Assert(t, icmd.Success)
		golden.Assert(t, res.Stdout(), GoldenFile("ls-out-test-docker"))
	})

	t.Run("ls quiet", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("context", "ls", "-q")
		golden.Assert(t, res.Stdout(), "ls-out-test-docker-quiet.golden")
	})

	t.Run("ls format", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("context", "ls", "--format", "{{ json . }}")
		res.Assert(t, icmd.Expected{Out: `"Name":"default"`})
	})
}

func TestContextInspect(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)
	res := c.RunDockerCmd("context", "create", "test-docker", "--from", "default")
	res.Assert(t, icmd.Expected{Out: "test-docker"})

	t.Run("inspect current", func(t *testing.T) {
		// Cannot be run in parallel because of "context use"
		res := c.RunDockerCmd("context", "use", "test-docker")
		res.Assert(t, icmd.Expected{Out: "test-docker"})

		res = c.RunDockerCmd("context", "inspect")
		res.Assert(t, icmd.Expected{Out: `"Name": "test-docker"`})
	})
}

func TestContextHelpACI(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)

	t.Run("help", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("context", "create", "aci", "--help")
		// Can't use golden here as the help prints the config directory which changes
		res.Assert(t, icmd.Expected{Out: "docker context create aci CONTEXT [flags]"})
		res.Assert(t, icmd.Expected{Out: "--location"})
		res.Assert(t, icmd.Expected{Out: "--subscription-id"})
		res.Assert(t, icmd.Expected{Out: "--resource-group"})
	})

	t.Run("check exec", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("context", "create", "aci", "--subscription-id", "invalid-id")
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "accepts 1 arg(s), received 0",
		})
		assert.Assert(t, !strings.Contains(res.Combined(), "unknown flag"))
	})
}

func TestContextDuplicateACI(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)

	c.RunDockerCmd("context", "create", "mycontext", "--from", "default").Assert(t, icmd.Success)
	res := c.RunDockerCmd("context", "create", "aci", "mycontext")
	res.Assert(t, icmd.Expected{
		ExitCode: 1,
		Err:      "context mycontext: already exists",
	})
}

func TestContextRemove(t *testing.T) {

	t.Run("remove current", func(t *testing.T) {
		c := NewParallelE2eCLI(t, binDir)

		c.RunDockerCmd("context", "create", "test-context-rm", "--from", "default").Assert(t, icmd.Success)
		res := c.RunDockerCmd("context", "use", "test-context-rm")
		res.Assert(t, icmd.Expected{Out: "test-context-rm"})
		res = c.RunDockerCmd("context", "rm", "test-context-rm")
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "cannot delete current context",
		})
	})

	t.Run("force remove current", func(t *testing.T) {
		c := NewParallelE2eCLI(t, binDir)

		c.RunDockerCmd("context", "create", "test-context-rmf").Assert(t, icmd.Success)
		c.RunDockerCmd("context", "use", "test-context-rmf").Assert(t, icmd.Success)
		res := c.RunDockerCmd("context", "rm", "-f", "test-context-rmf")
		res.Assert(t, icmd.Expected{Out: "test-context-rmf"})
		res = c.RunDockerCmd("context", "ls")
		res.Assert(t, icmd.Expected{Out: "default *"})
	})
}

func TestLoginCommandDelegation(t *testing.T) {
	// These tests just check that the existing CLI is called in various cases.
	// They do not test actual login functionality.
	c := NewParallelE2eCLI(t, binDir)

	t.Run("default context", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("login", "-u", "nouser", "-p", "wrongpasword")
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "unauthorized: incorrect username or password",
		})
	})

	t.Run("interactive", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("login", "someregistry.docker.io")
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "Cannot perform an interactive login from a non TTY device",
		})
	})

	t.Run("logout", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("logout", "someregistry.docker.io")
		res.Assert(t, icmd.Expected{Out: "someregistry.docker.io"})
	})

	t.Run("existing context", func(t *testing.T) {
		c := NewParallelE2eCLI(t, binDir)
		c.RunDockerCmd("context", "create", "local", "local").Assert(t, icmd.Success)
		c.RunDockerCmd("context", "use", "local").Assert(t, icmd.Success)
		res := c.RunDockerCmd("login", "-u", "nouser", "-p", "wrongpasword")
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "unauthorized: incorrect username or password",
		})
	})
}

func TestCloudLogin(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)

	t.Run("unknown backend", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("login", "mycloudbackend")
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "unknown backend type for cloud login: mycloudbackend",
		})
	})
}

func TestMissingExistingCLI(t *testing.T) {
	t.Parallel()
	home, err := ioutil.TempDir("", "")
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(home)
	})

	bin, err := ioutil.TempDir("", "")
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(bin)
	})
	err = CopyFile(filepath.Join(binDir, DockerExecutableName), filepath.Join(bin, DockerExecutableName))
	assert.NilError(t, err)

	env := []string{"PATH=" + bin}
	if runtime.GOOS == "windows" {
		env = append(env, "USERPROFILE="+home)

	} else {
		env = append(env, "HOME="+home)
	}

	c := icmd.Cmd{
		Env:     env,
		Command: []string{filepath.Join(bin, "docker")},
	}
	res := icmd.RunCmd(c)
	res.Assert(t, icmd.Expected{
		ExitCode: 1,
		Err:      `"com.docker.cli": executable file not found`,
	})
}

func TestLegacy(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)

	t.Run("help", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("--help")
		res.Assert(t, icmd.Expected{Out: "swarm"})
	})

	t.Run("swarm", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("swarm", "join")
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      `"docker swarm join" requires exactly 1 argument.`,
		})
	})

	t.Run("local run", func(t *testing.T) {
		t.Parallel()
		cmd := c.NewDockerCmd("run", "--rm", "hello-world")
		cmd.Timeout = 40 * time.Second
		res := icmd.RunCmd(cmd)
		res.Assert(t, icmd.Expected{Out: "Hello from Docker!"})
	})

	t.Run("error messages", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("foo")
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "docker: 'foo' is not a docker command.",
		})
	})

	t.Run("host flag", func(t *testing.T) {
		t.Parallel()
		stderr := "Cannot connect to the Docker daemon at tcp://localhost:123"
		if runtime.GOOS == "windows" {
			stderr = "error during connect: Get http://localhost:123"
		}
		res := c.RunDockerCmd("-H", "tcp://localhost:123", "version")
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      stderr,
		})
	})

	t.Run("existing contexts delegate", func(t *testing.T) {
		c := NewParallelE2eCLI(t, binDir)
		c.RunDockerCmd("context", "create", "moby-ctx", "--from=default").Assert(t, icmd.Success)
		c.RunDockerCmd("context", "use", "moby-ctx").Assert(t, icmd.Success)
		res := c.RunDockerCmd("swarm", "join")
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      `"docker swarm join" requires exactly 1 argument.`,
		})
	})

	t.Run("host flag overrides context", func(t *testing.T) {
		c := NewParallelE2eCLI(t, binDir)
		c.RunDockerCmd("context", "create", "example", "test-example").Assert(t, icmd.Success)
		c.RunDockerCmd("context", "use", "test-example").Assert(t, icmd.Success)
		endpoint := "unix:///var/run/docker.sock"
		if runtime.GOOS == "windows" {
			endpoint = "npipe:////./pipe/docker_engine"
		}
		res := c.RunDockerCmd("-H", endpoint, "ps")
		res.Assert(t, icmd.Success)
		// Example backend's ps output includes these strings
		assert.Assert(t, !strings.Contains(res.Stdout(), "id"))
		assert.Assert(t, !strings.Contains(res.Stdout(), "1234"))
	})
}

func TestLegacyLogin(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)

	t.Run("host flag login", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("-H", "tcp://localhost:123", "login", "-u", "nouser", "-p", "wrongpasword")
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "WARNING! Using --password via the CLI is insecure. Use --password-stdin.",
		})
	})

	t.Run("log level flag login", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("--log-level", "debug", "login", "-u", "nouser", "-p", "wrongpasword")
		res.Assert(t, icmd.Expected{
			ExitCode: 1,
			Err:      "WARNING! Using --password via the CLI is insecure",
		})
	})

	t.Run("login help global flags", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("login", "--help")
		res.Assert(t, icmd.Success)
		assert.Assert(t, !strings.Contains(res.Combined(), "--log-level"))
	})
}

func TestUnsupportedCommand(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)

	res := c.RunDockerCmd("context", "create", "example", "test-example")
	res.Assert(t, icmd.Success)
	res = c.RunDockerCmd("--context", "test-example", "images")
	res.Assert(t, icmd.Expected{
		ExitCode: 1,
		Err:      `Command "images" not available in current context (test-example), you can use the "default" context to run this command`,
	})
}

func TestVersion(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)

	t.Run("azure version", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("version")
		res.Assert(t, icmd.Expected{Out: "Azure integration"})
	})

	t.Run("format", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("version", "-f", "{{ json . }}")
		res.Assert(t, icmd.Expected{Out: `"Client":`})
		res = c.RunDockerCmd("version", "--format", "{{ json . }}")
		res.Assert(t, icmd.Expected{Out: `"Client":`})
	})

	t.Run("delegate version flag", func(t *testing.T) {
		c := NewParallelE2eCLI(t, binDir)
		c.RunDockerCmd("context", "create", "example", "test-example").Assert(t, icmd.Success)
		c.RunDockerCmd("context", "use", "test-example").Assert(t, icmd.Success)
		res := c.RunDockerCmd("-v")
		res.Assert(t, icmd.Expected{Out: "Docker version"})
	})
}

func TestMockBackend(t *testing.T) {
	c := NewParallelE2eCLI(t, binDir)
	c.RunDockerCmd("context", "create", "example", "test-example").Assert(t, icmd.Success)
	res := c.RunDockerCmd("context", "use", "test-example")
	res.Assert(t, icmd.Expected{Out: "test-example"})

	t.Run("use", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("context", "show")
		res.Assert(t, icmd.Expected{Out: "test-example"})
		res = c.RunDockerCmd("context", "ls")
		golden.Assert(t, res.Stdout(), GoldenFile("ls-out-test-example"))
	})

	t.Run("ps", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("ps")
		res.Assert(t, icmd.Success)
		golden.Assert(t, res.Stdout(), "ps-out-example.golden")
	})

	t.Run("ps quiet", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("ps", "-q")
		res.Assert(t, icmd.Success)
		golden.Assert(t, res.Stdout(), "ps-quiet-out-example.golden")
	})

	t.Run("ps quiet all", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("ps", "-q", "--all")
		res.Assert(t, icmd.Success)
		golden.Assert(t, res.Stdout(), "ps-quiet-all-out-example.golden")
	})

	t.Run("inspect", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("inspect", "id")
		res.Assert(t, icmd.Success)
		golden.Assert(t, res.Stdout(), "inspect-id.golden")
	})

	t.Run("run", func(t *testing.T) {
		t.Parallel()
		res := c.RunDockerCmd("run", "-d", "nginx", "-p", "80:80")
		res.Assert(t, icmd.Expected{
			Out: `Running container "nginx" with name`,
		})
	})
}
