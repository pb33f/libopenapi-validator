// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package requeststate

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"
)

func TestSnapshotAndInstall(t *testing.T) {
	request, err := http.NewRequest(http.MethodPost, "http://example.com", bytes.NewBufferString("body"))
	require.NoError(t, err)
	body, err := Snapshot(request)
	require.NoError(t, err)
	assert.Equal(t, "body", string(body))
	body, err = Snapshot(request)
	require.NoError(t, err)
	assert.Equal(t, "body", string(body))
	assert.Equal(t, int64(4), request.ContentLength)
	replayed, err := request.GetBody()
	require.NoError(t, err)
	bytesRead, _ := io.ReadAll(replayed)
	assert.Equal(t, "body", string(bytesRead))
	require.NoError(t, replayed.Close())

	Install(request, []byte("new"))
	bytesRead, _ = io.ReadAll(request.Body)
	assert.Equal(t, "new", string(bytesRead))
	Install(nil, nil)
}

func TestSnapshotConsumedAndStaleBodies(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "http://example.com", bytes.NewReader([]byte("original")))
	_, _ = io.ReadAll(request.Body)
	body, err := Snapshot(request)
	require.NoError(t, err)
	assert.Equal(t, "original", string(body))

	request, _ = http.NewRequest(http.MethodPost, "http://example.com", bytes.NewBufferString("stale"))
	request.Body = io.NopCloser(bytes.NewBufferString("fresh"))
	body, err = Snapshot(request)
	require.NoError(t, err)
	assert.Equal(t, "fresh", string(body))

	request, _ = http.NewRequest(http.MethodPost, "http://example.com", bytes.NewBufferString("stale"))
	request.Body = io.NopCloser(bytes.NewBufferString("other"))
	_, _ = io.ReadAll(request.Body)
	body, err = Snapshot(request)
	require.NoError(t, err)
	assert.Empty(t, body)
}

func TestSnapshotNilEmptyAndFailures(t *testing.T) {
	body, err := Snapshot(nil)
	require.NoError(t, err)
	assert.Nil(t, body)
	request := &http.Request{Body: http.NoBody, GetBody: func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewBufferString("stale")), nil
	}}
	body, err = Snapshot(request)
	require.NoError(t, err)
	assert.Empty(t, body)
	replayed, _ := request.GetBody()
	replayedBytes, _ := io.ReadAll(replayed)
	assert.Equal(t, "stale", string(replayedBytes), "http.NoBody must not mutate an unrelated GetBody function")

	request = &http.Request{GetBody: func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewBufferString("from get body")), nil
	}}
	body, err = Snapshot(request)
	require.NoError(t, err)
	assert.Equal(t, "from get body", string(body))
	installed, readErr := io.ReadAll(request.Body)
	require.NoError(t, readErr)
	assert.Equal(t, "from get body", string(installed))

	request = &http.Request{}
	body, err = Snapshot(request)
	require.NoError(t, err)
	assert.Nil(t, body)
	assert.Nil(t, request.Body)

	request = &http.Request{GetBody: func() (io.ReadCloser, error) { return nil, errors.New("get body failed") }}
	_, err = Snapshot(request)
	assert.ErrorContains(t, err, "get body failed")
	request = &http.Request{GetBody: func() (io.ReadCloser, error) { return nil, nil }}
	body, err = Snapshot(request)
	require.NoError(t, err)
	assert.Nil(t, body)
	request = &http.Request{GetBody: func() (io.ReadCloser, error) {
		return &errorBody{readErr: errors.New("get body read failed")}, nil
	}}
	_, err = Snapshot(request)
	assert.ErrorContains(t, err, "get body read failed")

	request = &http.Request{Body: &errorBody{readErr: errors.New("read failed")}}
	_, err = Snapshot(request)
	assert.ErrorContains(t, err, "read failed")
	request = &http.Request{Body: &failingReplayable{}}
	body, err = Snapshot(request)
	require.NoError(t, err)
	assert.Empty(t, body)

	var nilBody *errorBody
	assert.Nil(t, underlyingReader(nilBody))
	plain := &errorBody{}
	assert.Same(t, plain, underlyingReader(plain))
}

func TestWithFreshBody(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "http://example.com", bytes.NewBufferString("payload"))
	var reads []string
	for range 2 {
		err := WithFreshBody(request, func() error {
			body, readErr := io.ReadAll(request.Body)
			reads = append(reads, string(body))
			return readErr
		})
		require.NoError(t, err)
	}
	assert.Equal(t, []string{"payload", "payload"}, reads)
	restored, _ := io.ReadAll(request.Body)
	assert.Equal(t, "payload", string(restored))

	sentinel := errors.New("callback failed")
	err := WithFreshBody(request, func() error { return sentinel })
	assert.ErrorIs(t, err, sentinel)
	err = WithFreshBody(request, func() error {
		request.Body = nil
		return nil
	})
	require.NoError(t, err)
	err = WithFreshBody(request, func() error {
		request.GetBody = nil
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, request.GetBody)
	replayed, replayErr := request.GetBody()
	require.NoError(t, replayErr)
	replayedBody, readErr := io.ReadAll(replayed)
	require.NoError(t, readErr)
	assert.Equal(t, "payload", string(replayedBody))
	assert.NoError(t, WithFreshBody(nil, func() error { return nil }))
	assert.NoError(t, WithFreshBody(&http.Request{Body: http.NoBody}, func() error { return nil }))
}

func TestWithFreshBodyFailures(t *testing.T) {
	request := &http.Request{Body: &errorBody{readErr: errors.New("snapshot failed")}}
	err := WithFreshBody(request, func() error { return nil })
	assert.ErrorContains(t, err, "snapshot failed")

	request = &http.Request{Body: io.NopCloser(bytes.NewReader(nil)), GetBody: func() (io.ReadCloser, error) {
		return nil, errors.New("fresh failed")
	}}
	err = WithFreshBody(request, func() error { return nil })
	assert.ErrorContains(t, err, "fresh failed")

	newRestoreFailureRequest := func() *http.Request {
		calls := 0
		return &http.Request{Body: io.NopCloser(bytes.NewReader(nil)), GetBody: func() (io.ReadCloser, error) {
			calls++
			if calls == 1 {
				return io.NopCloser(bytes.NewBufferString("body")), nil
			}
			return nil, errors.New("restore failed")
		}}
	}
	request = newRestoreFailureRequest()
	err = WithFreshBody(request, func() error { return nil })
	assert.ErrorContains(t, err, "restore failed")

	request = newRestoreFailureRequest()
	callbackFailure := errors.New("callback also failed")
	err = WithFreshBody(request, func() error { return callbackFailure })
	assert.ErrorContains(t, err, "restore failed")
	assert.ErrorIs(t, err, callbackFailure)
}

type errorBody struct {
	readErr error
}

func (e *errorBody) Read([]byte) (int, error) {
	if e.readErr != nil {
		return 0, e.readErr
	}
	return 0, io.EOF
}

func (*errorBody) Close() error { return nil }

type failingReplayable struct{ errorBody }

func (*failingReplayable) ReadAt([]byte, int64) (int, error) { return 0, errors.New("read-at failed") }
func (*failingReplayable) Size() int64                       { return 4 }
