package main

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueueSizeHandlerSuccess(t *testing.T) {
	r := require.New(t)
	reader := &fakeQueueCountReader{
		current: 123,
		err:     nil,
	}

	handler := newQueueSizeHandler(reader)
	_, echoCtx, rec := newTestCtx("GET", "/queue")
	err := handler(echoCtx)
	r.NoError(err)
	r.Equal(200, rec.Code, "response code")
	respMap := map[string]int{}
	decodeErr := json.NewDecoder(rec.Body).Decode(&respMap)
	r.NoError(decodeErr)
	r.Equalf(1, len(respMap), "response JSON length was not 1")
	sizeVal, ok := respMap["sample.com"]
	r.Truef(ok, "'sample.com' entry not available in return JSON")
	r.Equalf(reader.current, sizeVal, "returned JSON queue size was wrong")
	reader.err = errors.New("test error")
	r.Error(handler(echoCtx))
}

func TestQueueSizeHandlerFail(t *testing.T) {
	r := require.New(t)
	reader := &fakeQueueCountReader{
		current: 0,
		err:     errors.New("test error"),
	}

	handler := newQueueSizeHandler(reader)
	_, echoCtx, rec := newTestCtx("GET", "/queue")
	err := handler(echoCtx)
	r.Error(err)
	r.Equal(500, rec.Code, "response code")
}
