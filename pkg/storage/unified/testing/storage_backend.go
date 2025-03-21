package test

import (
	"context"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/grafana/authlib/authn"
	"github.com/grafana/authlib/types"
	"github.com/grafana/grafana/pkg/apimachinery/utils"
	infraDB "github.com/grafana/grafana/pkg/infra/db"
	"github.com/grafana/grafana/pkg/storage/unified/resource"
	"github.com/grafana/grafana/pkg/storage/unified/sql"
	"github.com/grafana/grafana/pkg/util/testutil"
)

type NewBackendFunc func(ctx context.Context) resource.StorageBackend

// TestCase defines a test case for the storage backend
type TestCase struct {
	name string
	fn   func(*testing.T, resource.StorageBackend)
}

// StorageBackendTestSuite defines the test suite for storage backend
type StorageBackendTestSuite struct {
	newBackend NewBackendFunc
	cases      []TestCase
}

// NewStorageBackendTestSuite creates a new test suite
func NewStorageBackendTestSuite(newBackend NewBackendFunc) *StorageBackendTestSuite {
	return &StorageBackendTestSuite{
		newBackend: newBackend,
		cases: []TestCase{
			{"happy path", runTestIntegrationBackendHappyPath},
			{"watch write events from latest", runTestIntegrationBackendWatchWriteEventsFromLastest},
			{"list", runTestIntegrationBackendList},
			{"blob support", runTestIntegrationBlobSupport},
		},
	}
}

// Run executes all test cases in the suite
func (s *StorageBackendTestSuite) Run(t *testing.T) {
	for _, tc := range s.cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.fn(t, s.newBackend(context.Background()))
		})
	}
}

// RunStorageBackendTest runs the storage backend test suite
func RunStorageBackendTest(t *testing.T, newBackend NewBackendFunc) {
	suite := NewStorageBackendTestSuite(newBackend)
	suite.Run(t)
}

func runTestIntegrationBackendHappyPath(t *testing.T, backend resource.StorageBackend) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := types.WithAuthInfo(context.Background(), authn.NewAccessTokenAuthInfo(authn.Claims[authn.AccessTokenClaims]{
		Claims: jwt.Claims{
			Subject: "testuser",
		},
		Rest: authn.AccessTokenClaims{},
	}))

	server := newServer(t, backend)

	stream, err := backend.WatchWriteEvents(context.Background()) // Using a different context to avoid canceling the stream after the DefaultContextTimeout
	require.NoError(t, err)
	var rv1, rv2, rv3, rv4, rv5 int64

	t.Run("Add 3 resources", func(t *testing.T) {
		rv1, err = writeEvent(ctx, backend, "item1", resource.WatchEvent_ADDED)
		require.NoError(t, err)
		require.Greater(t, rv1, int64(0))

		rv2, err = writeEvent(ctx, backend, "item2", resource.WatchEvent_ADDED)
		require.NoError(t, err)
		require.Greater(t, rv2, rv1)

		rv3, err = writeEvent(ctx, backend, "item3", resource.WatchEvent_ADDED)
		require.NoError(t, err)
		require.Greater(t, rv3, rv2)

		stats, err := backend.GetResourceStats(ctx, "", 0)
		require.NoError(t, err)
		require.Len(t, stats, 1)
		require.Equal(t, int64(3), stats[0].Count)
		require.Equal(t, rv3, stats[0].ResourceVersion)
	})

	t.Run("Update item2", func(t *testing.T) {
		rv4, err = writeEvent(ctx, backend, "item2", resource.WatchEvent_MODIFIED)
		require.NoError(t, err)
		require.Greater(t, rv4, rv3)
	})

	t.Run("Delete item1", func(t *testing.T) {
		rv5, err = writeEvent(ctx, backend, "item1", resource.WatchEvent_DELETED)
		require.NoError(t, err)
		require.Greater(t, rv5, rv4)
	})

	t.Run("Read latest item 2", func(t *testing.T) {
		resp := backend.ReadResource(ctx, &resource.ReadRequest{Key: resourceKey("item2")})
		require.Nil(t, resp.Error)
		require.Equal(t, rv4, resp.ResourceVersion)
		require.Equal(t, "item2 MODIFIED", string(resp.Value))
		require.Equal(t, "folderuid", resp.Folder)
	})

	t.Run("Read early version of item2", func(t *testing.T) {
		resp := backend.ReadResource(ctx, &resource.ReadRequest{
			Key:             resourceKey("item2"),
			ResourceVersion: rv3, // item2 was created at rv2 and updated at rv4
		})
		require.Nil(t, resp.Error)
		require.Equal(t, rv2, resp.ResourceVersion)
		require.Equal(t, "item2 ADDED", string(resp.Value))
	})

	t.Run("PrepareList latest", func(t *testing.T) {
		resp, err := server.List(ctx, &resource.ListRequest{
			Options: &resource.ListOptions{
				Key: &resource.ResourceKey{
					Namespace: "namespace",
					Group:     "group",
					Resource:  "resource",
				},
			},
		})
		require.NoError(t, err)
		require.Nil(t, resp.Error)
		require.Len(t, resp.Items, 2)
		require.Equal(t, "item2 MODIFIED", string(resp.Items[0].Value))
		require.Equal(t, "item3 ADDED", string(resp.Items[1].Value))
		require.Equal(t, rv5, resp.ResourceVersion)
	})

	t.Run("Watch events", func(t *testing.T) {
		event := <-stream
		require.Equal(t, "item1", event.Key.Name)
		require.Equal(t, rv1, event.ResourceVersion)
		require.Equal(t, resource.WatchEvent_ADDED, event.Type)
		require.Equal(t, "folderuid", event.Folder)

		event = <-stream
		require.Equal(t, "item2", event.Key.Name)
		require.Equal(t, rv2, event.ResourceVersion)
		require.Equal(t, resource.WatchEvent_ADDED, event.Type)
		require.Equal(t, "folderuid", event.Folder)

		event = <-stream
		require.Equal(t, "item3", event.Key.Name)
		require.Equal(t, rv3, event.ResourceVersion)
		require.Equal(t, resource.WatchEvent_ADDED, event.Type)

		event = <-stream
		require.Equal(t, "item2", event.Key.Name)
		require.Equal(t, rv4, event.ResourceVersion)
		require.Equal(t, resource.WatchEvent_MODIFIED, event.Type)

		event = <-stream
		require.Equal(t, "item1", event.Key.Name)
		require.Equal(t, rv5, event.ResourceVersion)
		require.Equal(t, resource.WatchEvent_DELETED, event.Type)
	})
}

func runTestIntegrationBackendWatchWriteEventsFromLastest(t *testing.T, backend resource.StorageBackend) {
	if infraDB.IsTestDbSQLite() {
		t.Skip("TODO: test blocking, skipping to unblock Enterprise until we fix this")
	}
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := testutil.NewTestContext(t, time.Now().Add(5*time.Second))

	// Create a few resources before initing the watch
	_, err := writeEvent(ctx, backend, "item1", resource.WatchEvent_ADDED)
	require.NoError(t, err)

	// Start the watch
	stream, err := backend.WatchWriteEvents(ctx)
	require.NoError(t, err)

	// Create one more event
	_, err = writeEvent(ctx, backend, "item2", resource.WatchEvent_ADDED)
	require.NoError(t, err)
	require.Equal(t, "item2", (<-stream).Key.Name)
}

func runTestIntegrationBackendList(t *testing.T, backend resource.StorageBackend) {
	if infraDB.IsTestDbSQLite() {
		t.Skip("TODO: test blocking, skipping to unblock Enterprise until we fix this")
	}
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := testutil.NewTestContext(t, time.Now().Add(5*time.Second))
	server := newServer(t, backend)

	// Create a few resources before starting the watch
	rv1, _ := writeEvent(ctx, backend, "item1", resource.WatchEvent_ADDED)
	require.Greater(t, rv1, int64(0))
	rv2, _ := writeEvent(ctx, backend, "item2", resource.WatchEvent_ADDED) // rv=2 - will be modified at rv=6
	require.Greater(t, rv2, rv1)
	rv3, _ := writeEvent(ctx, backend, "item3", resource.WatchEvent_ADDED) // rv=3 - will be deleted  at rv=7
	require.Greater(t, rv3, rv2)
	rv4, _ := writeEvent(ctx, backend, "item4", resource.WatchEvent_ADDED)
	require.Greater(t, rv4, rv3)
	rv5, _ := writeEvent(ctx, backend, "item5", resource.WatchEvent_ADDED)
	require.Greater(t, rv5, rv4)
	rv6, _ := writeEvent(ctx, backend, "item2", resource.WatchEvent_MODIFIED)
	require.Greater(t, rv6, rv5)
	rv7, _ := writeEvent(ctx, backend, "item3", resource.WatchEvent_DELETED)
	require.Greater(t, rv7, rv6)
	rv8, _ := writeEvent(ctx, backend, "item6", resource.WatchEvent_ADDED)
	require.Greater(t, rv8, rv7)

	t.Run("fetch all latest", func(t *testing.T) {
		res, err := server.List(ctx, &resource.ListRequest{
			Options: &resource.ListOptions{
				Key: &resource.ResourceKey{
					Group:    "group",
					Resource: "resource",
				},
			},
		})
		require.NoError(t, err)
		require.Nil(t, res.Error)
		require.Len(t, res.Items, 5)
		// should be sorted by key ASC
		require.Equal(t, "item1 ADDED", string(res.Items[0].Value))
		require.Equal(t, "item2 MODIFIED", string(res.Items[1].Value))
		require.Equal(t, "item4 ADDED", string(res.Items[2].Value))
		require.Equal(t, "item5 ADDED", string(res.Items[3].Value))
		require.Equal(t, "item6 ADDED", string(res.Items[4].Value))

		require.Empty(t, res.NextPageToken)
	})

	t.Run("list latest first page ", func(t *testing.T) {
		res, err := server.List(ctx, &resource.ListRequest{
			Limit: 3,
			Options: &resource.ListOptions{
				Key: &resource.ResourceKey{
					Group:    "group",
					Resource: "resource",
				},
			},
		})
		require.NoError(t, err)
		require.Nil(t, res.Error)
		require.Len(t, res.Items, 3)
		continueToken, err := sql.GetContinueToken(res.NextPageToken)
		require.NoError(t, err)
		require.Equal(t, "item1 ADDED", string(res.Items[0].Value))
		require.Equal(t, "item2 MODIFIED", string(res.Items[1].Value))
		require.Equal(t, "item4 ADDED", string(res.Items[2].Value))
		require.Equal(t, rv8, continueToken.ResourceVersion)
	})

	t.Run("list at revision", func(t *testing.T) {
		res, err := server.List(ctx, &resource.ListRequest{
			ResourceVersion: rv4,
			Options: &resource.ListOptions{
				Key: &resource.ResourceKey{
					Group:    "group",
					Resource: "resource",
				},
			},
		})
		require.NoError(t, err)
		require.Nil(t, res.Error)
		require.Len(t, res.Items, 4)
		require.Equal(t, "item1 ADDED", string(res.Items[0].Value))
		require.Equal(t, "item2 ADDED", string(res.Items[1].Value))
		require.Equal(t, "item3 ADDED", string(res.Items[2].Value))
		require.Equal(t, "item4 ADDED", string(res.Items[3].Value))
		require.Empty(t, res.NextPageToken)
	})

	t.Run("fetch first page at revision with limit", func(t *testing.T) {
		res, err := server.List(ctx, &resource.ListRequest{
			Limit:           3,
			ResourceVersion: rv7,
			Options: &resource.ListOptions{
				Key: &resource.ResourceKey{
					Group:    "group",
					Resource: "resource",
				},
			},
		})
		require.NoError(t, err)
		require.NoError(t, err)
		require.Nil(t, res.Error)
		require.Len(t, res.Items, 3)
		t.Log(res.Items)
		require.Equal(t, "item1 ADDED", string(res.Items[0].Value))
		require.Equal(t, "item2 MODIFIED", string(res.Items[1].Value))
		require.Equal(t, "item4 ADDED", string(res.Items[2].Value))

		continueToken, err := sql.GetContinueToken(res.NextPageToken)
		require.NoError(t, err)
		require.Equal(t, rv7, continueToken.ResourceVersion)
	})

	t.Run("fetch second page at revision", func(t *testing.T) {
		continueToken := &sql.ContinueToken{
			ResourceVersion: rv8,
			StartOffset:     2,
		}
		res, err := server.List(ctx, &resource.ListRequest{
			NextPageToken: continueToken.String(),
			Limit:         2,
			Options: &resource.ListOptions{
				Key: &resource.ResourceKey{
					Group:    "group",
					Resource: "resource",
				},
			},
		})
		require.NoError(t, err)
		require.Nil(t, res.Error)
		require.Len(t, res.Items, 2)
		t.Log(res.Items)
		require.Equal(t, "item4 ADDED", string(res.Items[0].Value))
		require.Equal(t, "item5 ADDED", string(res.Items[1].Value))

		continueToken, err = sql.GetContinueToken(res.NextPageToken)
		require.NoError(t, err)
		require.Equal(t, rv8, continueToken.ResourceVersion)
		require.Equal(t, int64(4), continueToken.StartOffset)
	})

	// add 5 events for item1 - should be saved to history
	rvHistory1, err := writeEvent(ctx, backend, "item1", resource.WatchEvent_MODIFIED)
	require.NoError(t, err)
	require.Greater(t, rvHistory1, rv1)
	rvHistory2, err := writeEvent(ctx, backend, "item1", resource.WatchEvent_MODIFIED)
	require.NoError(t, err)
	require.Greater(t, rvHistory2, rvHistory1)
	rvHistory3, err := writeEvent(ctx, backend, "item1", resource.WatchEvent_MODIFIED)
	require.NoError(t, err)
	require.Greater(t, rvHistory3, rvHistory2)
	rvHistory4, err := writeEvent(ctx, backend, "item1", resource.WatchEvent_MODIFIED)
	require.NoError(t, err)
	require.Greater(t, rvHistory4, rvHistory3)
	rvHistory5, err := writeEvent(ctx, backend, "item1", resource.WatchEvent_MODIFIED)
	require.NoError(t, err)
	require.Greater(t, rvHistory5, rvHistory4)

	t.Run("fetch first history page at revision with limit", func(t *testing.T) {
		res, err := server.List(ctx, &resource.ListRequest{
			Limit:  3,
			Source: resource.ListRequest_HISTORY,
			Options: &resource.ListOptions{
				Key: &resource.ResourceKey{
					Namespace: "namespace",
					Group:     "group",
					Resource:  "resource",
					Name:      "item1",
				},
			},
		})
		require.NoError(t, err)
		require.NoError(t, err)
		require.Nil(t, res.Error)
		require.Len(t, res.Items, 3)
		t.Log(res.Items)
		// should be in desc order, so the newest RVs are returned first
		require.Equal(t, "item1 MODIFIED", string(res.Items[0].Value))
		require.Equal(t, rvHistory5, res.Items[0].ResourceVersion)
		require.Equal(t, "item1 MODIFIED", string(res.Items[1].Value))
		require.Equal(t, rvHistory4, res.Items[1].ResourceVersion)
		require.Equal(t, "item1 MODIFIED", string(res.Items[2].Value))
		require.Equal(t, rvHistory3, res.Items[2].ResourceVersion)

		continueToken, err := sql.GetContinueToken(res.NextPageToken)
		require.NoError(t, err)
		//  should return the furthest back RV as the next page token
		require.Equal(t, rvHistory3, continueToken.ResourceVersion)
	})

	t.Run("fetch second page of history at revision", func(t *testing.T) {
		continueToken := &sql.ContinueToken{
			ResourceVersion: rvHistory3,
			StartOffset:     2,
		}
		res, err := server.List(ctx, &resource.ListRequest{
			NextPageToken: continueToken.String(),
			Limit:         2,
			Source:        resource.ListRequest_HISTORY,
			Options: &resource.ListOptions{
				Key: &resource.ResourceKey{
					Namespace: "namespace",
					Group:     "group",
					Resource:  "resource",
					Name:      "item1",
				},
			},
		})
		require.NoError(t, err)
		require.Nil(t, res.Error)
		require.Len(t, res.Items, 2)
		t.Log(res.Items)
		require.Equal(t, "item1 MODIFIED", string(res.Items[0].Value))
		require.Equal(t, rvHistory2, res.Items[0].ResourceVersion)
		require.Equal(t, "item1 MODIFIED", string(res.Items[1].Value))
		require.Equal(t, rvHistory1, res.Items[1].ResourceVersion)
	})
}

func runTestIntegrationBlobSupport(t *testing.T, backend resource.StorageBackend) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := testutil.NewTestContext(t, time.Now().Add(5*time.Second))
	server := newServer(t, backend)
	store, ok := backend.(resource.BlobSupport)
	require.True(t, ok)

	t.Run("put and fetch blob", func(t *testing.T) {
		key := &resource.ResourceKey{
			Namespace: "ns",
			Group:     "g",
			Resource:  "r",
			Name:      "n",
		}

		b1, err := server.PutBlob(ctx, &resource.PutBlobRequest{
			Resource:    key,
			Method:      resource.PutBlobRequest_GRPC,
			ContentType: "plain/text",
			Value:       []byte("hello 11111"),
		})
		require.NoError(t, err)
		require.Nil(t, b1.Error)
		require.Equal(t, "c894ae57bd227b8f8c63f38a2ddf458b", b1.Hash)

		b2, err := server.PutBlob(ctx, &resource.PutBlobRequest{
			Resource:    key,
			Method:      resource.PutBlobRequest_GRPC,
			ContentType: "plain/text",
			Value:       []byte("hello 22222"), // the most recent
		})
		require.NoError(t, err)
		require.Nil(t, b2.Error)
		require.Equal(t, "b0da48de4ff92e0ad0d836de4d746937", b2.Hash)

		// Check that we can still access both values
		found, err := store.GetResourceBlob(ctx, key, &utils.BlobInfo{UID: b1.Uid}, true)
		require.NoError(t, err)
		require.Equal(t, []byte("hello 11111"), found.Value)

		found, err = store.GetResourceBlob(ctx, key, &utils.BlobInfo{UID: b2.Uid}, true)
		require.NoError(t, err)
		require.Equal(t, []byte("hello 22222"), found.Value)
	})
}

func writeEvent(ctx context.Context, store resource.StorageBackend, name string, action resource.WatchEvent_Type) (int64, error) {
	res := &unstructured.Unstructured{
		Object: map[string]any{},
	}
	meta, err := utils.MetaAccessor(res)
	if err != nil {
		return 0, err
	}
	meta.SetFolder("folderuid")
	return store.WriteEvent(ctx, resource.WriteEvent{
		Type:  action,
		Value: []byte(name + " " + resource.WatchEvent_Type_name[int32(action)]),
		Key: &resource.ResourceKey{
			Namespace: "namespace",
			Group:     "group",
			Resource:  "resource",
			Name:      name,
		},
		Object: meta,
	})
}

func resourceKey(name string) *resource.ResourceKey {
	return &resource.ResourceKey{
		Namespace: "namespace",
		Group:     "group",
		Resource:  "resource",
		Name:      name,
	}
}

func newServer(t *testing.T, b resource.StorageBackend) resource.ResourceServer {
	t.Helper()

	server, err := resource.NewResourceServer(resource.ResourceServerOptions{
		Backend: b,
	})
	require.NoError(t, err)
	require.NotNil(t, server)

	return server
}
