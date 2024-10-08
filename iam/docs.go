// Copyright 2023 The Alis Build Platform. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package iam is a simple Authorization package based on the
Google IAM (Identity and Access Management) Policy framework
involves defining and managing access controls for resources.

This package does handles the identification / authn part
of the IAM framework, as well as the Authorisation / Authz
side of the framework. It authorises whether a particular
**Principal** (user or service account) is able to perform a
particular **Permission** (Get, Update, List, etc.) on a
particular **Resource**. A resource is defined in the context
of a Resource Driven development framework as defined at
[RDD](https://google.com) inline with the
[API Improvement Proposals](https://aip.dev)
*/
package iam // import "go.alis.build/iam"
