// Copyright 2026 The Casdoor Authors. All Rights Reserved.
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

//go:build !skipCi

package object

import "testing"

// A negative codeResendTimeout must short-circuit before the record lookup.
//
// ormer is nil in this test binary, so reaching the database at all would panic.
// That is exactly the property being asserted: the disable path does no lookup,
// which is what makes it safe for deployments where remoteAddr is not a usable
// identity (upstream SNAT collapsing every caller onto a few pool addresses).
func TestIsAllowSendDisabledByNegativeTimeout(t *testing.T) {
	application := &Application{CodeResendTimeout: -1}

	if err := IsAllowSend(nil, "116.136.189.11", VerifyTypePhone, application); err != nil {
		t.Fatalf("negative codeResendTimeout should disable the throttle, got: %v", err)
	}
}

// Any negative value disables it, not just -1 — callers should not have to guess
// a magic number.
func TestIsAllowSendDisabledByAnyNegativeTimeout(t *testing.T) {
	for _, timeout := range []int{-1, -60, -9999} {
		application := &Application{CodeResendTimeout: timeout}

		if err := IsAllowSend(nil, "116.136.189.11", VerifyTypeEmail, application); err != nil {
			t.Fatalf("codeResendTimeout=%d should disable the throttle, got: %v", timeout, err)
		}
	}
}
