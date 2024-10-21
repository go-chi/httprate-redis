package httprateredis_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	httprateredis "github.com/go-chi/httprate-redis"
)

// Test local in-memory counter fallback, which gets activated in case Redis is not available.
func TestLocalFallback(t *testing.T) {
	redis, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	redisPort, _ := strconv.Atoi(redis.Port())

	var onErrorCalled bool
	var onFallbackCalled bool

	limitCounter := httprateredis.NewCounter(&httprateredis.Config{
		Host:             redis.Host(),
		Port:             uint16(redisPort),
		MaxIdle:          0,
		MaxActive:        1,
		ClientName:       "httprateredis_test",
		PrefixKey:        fmt.Sprintf("httprate:test:%v", rand.Int31n(100000)), // Unique Redis key for each test
		FallbackTimeout:  200 * time.Millisecond,
		OnError:          func(err error) { onErrorCalled = true },
		OnFallbackChange: func(fallbackActivated bool) { onFallbackCalled = true },
	})

	limitCounter.Config(1000, time.Minute)

	currentWindow := time.Now().UTC().Truncate(time.Minute)
	previousWindow := currentWindow.Add(-time.Minute)

	if limitCounter.IsFallbackActivated() {
		t.Error("fallback should not be activated at the beginning")
	}
	if onErrorCalled {
		t.Error("onError() should not be called at the beginning")
	}
	if onFallbackCalled {
		t.Error("onFallback() should not be called before we simulate redis failure")
	}

	err = limitCounter.IncrementBy("key:fallback", currentWindow, 1)
	if err != nil {
		t.Error(err)
	}

	_, _, err = limitCounter.Get("key:fallback", currentWindow, previousWindow)
	if err != nil {
		t.Error(err)
	}

	if limitCounter.IsFallbackActivated() {
		t.Error("fallback should not be activated before we simulate redis failure")
	}
	if onErrorCalled {
		t.Error("onError() should not be called before we simulate redis failure")
	}
	if onFallbackCalled {
		t.Error("onFallback() should not be called before we simulate redis failure")
	}

	redis.Close()

	err = limitCounter.IncrementBy("key:fallback", currentWindow, 1)
	if err != nil {
		t.Error(err)
	}

	_, _, err = limitCounter.Get("key:fallback", currentWindow, previousWindow)
	if err != nil {
		t.Error(err)
	}

	if !limitCounter.IsFallbackActivated() {
		t.Error("fallback should be activated after we simulate redis failure")
	}
	if !onErrorCalled {
		t.Error("onError() should be called after we simulate redis failure")
	}
	if !onFallbackCalled {
		t.Error("onFallback() should be called after we simulate redis failure")
	}

}
