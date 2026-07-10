// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

// Package requeststate owns replayable request-body snapshots shared by validation stages.
package requeststate

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"reflect"
)

type replayableBody interface {
	io.ReaderAt
	Size() int64
}

type snapshotBody struct {
	*bytes.Reader
	bytes []byte
}

func (b *snapshotBody) Close() error { return nil }

// Snapshot reads a request body once and installs replay support while leaving it readable.
func Snapshot(request *http.Request) ([]byte, error) {
	if request == nil {
		return nil, nil
	}
	if request.Body == nil {
		if request.GetBody == nil {
			return nil, nil
		}
		replay, err := request.GetBody()
		if err != nil {
			return nil, err
		}
		if replay == nil {
			return nil, nil
		}
		body, readErr := io.ReadAll(replay)
		_ = replay.Close()
		if readErr != nil {
			return nil, readErr
		}
		Install(request, body)
		return body, nil
	}
	if request.Body == http.NoBody {
		return nil, nil
	}
	if snapshot, ok := request.Body.(*snapshotBody); ok {
		return append([]byte(nil), snapshot.bytes...), nil
	}
	var prior []byte
	if replayable, ok := underlyingReader(request.Body).(replayableBody); ok && replayable.Size() > 0 {
		var err error
		prior, err = io.ReadAll(io.NewSectionReader(replayable, 0, replayable.Size()))
		if err != nil {
			prior = nil
		}
	}
	body, err := io.ReadAll(request.Body)
	_ = request.Body.Close()
	if err != nil {
		return nil, err
	}
	if len(body) == 0 && len(prior) > 0 && request.GetBody != nil {
		replay, replayErr := request.GetBody()
		if replayErr == nil && replay != nil {
			replayed, readErr := io.ReadAll(replay)
			_ = replay.Close()
			if readErr == nil && bytes.Equal(replayed, prior) {
				body = replayed
			}
		}
	}
	Install(request, body)
	return body, nil
}

func underlyingReader(body io.ReadCloser) io.Reader {
	value := reflect.ValueOf(body)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if value.Kind() == reflect.Struct {
		field := value.FieldByName("Reader")
		if field.IsValid() && field.CanInterface() {
			if reader, ok := field.Interface().(io.Reader); ok {
				return reader
			}
		}
	}
	return body
}

// Install replaces a request body with an immutable replayable snapshot.
func Install(request *http.Request, body []byte) {
	if request == nil {
		return
	}
	snapshot := append([]byte(nil), body...)
	request.Body = &snapshotBody{Reader: bytes.NewReader(snapshot), bytes: snapshot}
	request.ContentLength = int64(len(snapshot))
	request.GetBody = func() (io.ReadCloser, error) {
		return &snapshotBody{Reader: bytes.NewReader(snapshot), bytes: snapshot}, nil
	}
}

// WithFreshBody gives a callback an independent reader and restores a fresh reader afterward.
func WithFreshBody(request *http.Request, callback func() error) error {
	if request == nil || request.Body == nil || request.Body == http.NoBody {
		return callback()
	}
	if request.GetBody == nil {
		if _, err := Snapshot(request); err != nil {
			return err
		}
	}
	getBody := request.GetBody
	body, err := getBody()
	if err != nil {
		return err
	}
	request.Body = body
	callbackErr := callback()
	if request.Body != nil {
		_ = request.Body.Close()
	}
	restored, restoreErr := getBody()
	request.GetBody = getBody
	if restoreErr != nil {
		return errors.Join(callbackErr, restoreErr)
	}
	request.Body = restored
	return callbackErr
}
