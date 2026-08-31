// Package testing provides shared test-support helpers for the leakspok test
// suites. It exists so that container-backed Valkey helpers are defined once and
// reused by both the analyzer and analyzer/cache test packages instead of being
// duplicated per package.
package testing

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/valkey-io/valkey-go"
)

// StartValkeyContainer starts a disposable single-node Valkey server and returns
// its "host:port" address.
func StartValkeyContainer(t *testing.T) string {
	t.Helper()

	ctx := context.Background()

	container, err := tcredis.Run(ctx, "docker.io/valkey/valkey:8")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, container.Terminate(ctx))
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "6379")
	require.NoError(t, err)

	return host + ":" + port.Port()
}

// AssertValkeyClientName connects an independent inspection client to addr and
// asserts that Valkey's CLIENT LIST output reports a connection whose name is
// name. CLIENT GETNAME is never used: issued on the inspection connection it can
// only report that connection's own (empty) name, never the client under test,
// so CLIENT LIST is queried and filtered instead.
func AssertValkeyClientName(t *testing.T, addr, name string) {
	t.Helper()

	ctx := context.Background()

	inspector, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:       []string{addr},
		ForceSingleClient: true,
	})
	require.NoError(t, err)
	t.Cleanup(inspector.Close)

	list, err := inspector.Do(ctx, inspector.B().ClientList().Build()).ToString()
	require.NoError(t, err)

	found := false
	for _, line := range strings.Split(list, "\n") {
		for _, field := range strings.Fields(line) {
			if field == "name="+name {
				found = true
			}
		}
	}

	assert.True(t, found, "CLIENT LIST did not report connection named %q:\n%s", name, list)
}
