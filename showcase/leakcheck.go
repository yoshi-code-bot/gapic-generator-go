// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Contains functions to check leaked goroutines.
//
// Call "defer leakcheck.Check(t)" at the beginning of tests.
//
// This is very closely based on grpc-go's leakcheck.

package showcase

import (
	"runtime"
	"sort"
	"strings"
	"time"
)

var goroutinesToIgnore = []string{
	"net/http/transport.go",
	"net/http/h2_bundle.go",
	"go.opencensus.io/stats/view.init.0",
	"testing.Main(",
	"testing.tRunner(",
	"testing.(*M).",
	"runtime.goexit",
	"created by runtime.gc",
	"created by runtime/trace.Start",
	"interestingGoroutines",
	"runtime.MHeap_Scavenger",
	"signal.signal_recv",
	"sigterm.handler",
	"runtime_mcall",
	"(*loggingT).flushDaemon",
	"goroutine in C code",
	"internal/transport/http2_client.go",
}

// registerIgnoreGoroutine appends s into the ignore goroutine list. The
// goroutines whose stack trace contains s will not be identified as leaked
// goroutines. Not thread-safe, only call this function in init().
func registerIgnoreGoroutine(s string) {
	goroutinesToIgnore = append(goroutinesToIgnore, s)
}

func ignore(g string) bool {
	sl := strings.SplitN(g, "\n", 2)
	if len(sl) != 2 {
		return true
	}
	stack := strings.TrimSpace(sl[1])
	if strings.HasPrefix(stack, "testing.RunTests") {
		return true
	}

	if stack == "" {
		return true
	}

	for _, s := range goroutinesToIgnore {
		if strings.Contains(stack, s) {
			return true
		}
	}

	return false
}

// interestingGoroutines returns all goroutines we care about for the purpose of
// leak checking. It excludes testing or runtime ones.
func interestingGoroutines() (gs []string) {
	buf := make([]byte, 2<<20)
	buf = buf[:runtime.Stack(buf, true)]
	for _, g := range strings.Split(string(buf), "\n\n") {
		if !ignore(g) {
			gs = append(gs, g)
		}
	}
	sort.Strings(gs)
	return
}

// Errorfer is the interface that wraps the Errorf method. It's a subset of
// testing.TB to make it easy to use Check.
type Errorfer interface {
	Errorf(format string, args ...interface{})
}

func checkDeadline(efer Errorfer, timeout time.Duration) {
	// Loop, waiting for goroutines to shut down.
	// Wait up to timeout, but finish as quickly as possible.
	deadline := time.Now().Add(timeout)
	var leaked []string
	for time.Now().Before(deadline) {
		if leaked = interestingGoroutines(); len(leaked) == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, g := range leaked {
		efer.Errorf("Leaked goroutine: %v", g)
	}
}

// check looks at the currently-running goroutines and checks if there are any
// interesting (created by gRPC or the GAPIC) goroutines leaked. It waits up to
// 5 seconds in the error cases.
func check(efer Errorfer) {
	checkDeadline(efer, 5*time.Second)
}
