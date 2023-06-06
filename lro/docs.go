// Copyright 2022 The Alis Build Platform. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package lro provides a client for managing long-running operations (LROs) using Google Cloud Bigtable. LROs are a
pattern in resource-driven design where an operation takes an extended period of time to complete. This can be due to
a number of factors, such as the size of the operation, the amount of data involved, or the availability of resources.

The lro package provides a number of features for managing LROs, including:
  - Creating LROs: The CreateOperation method creates a new LRO and stores it in Bigtable.
  - Getting LROs: The GetOperation method gets an LRO from Bigtable.
  - Updating LROs: The SetSuccessful and SetFailed methods update the status of an LRO in Bigtable.
  - Waiting for LROs to finish: The WaitOperation method returns the LRO when it is done or when a specified timeout is
    reached.

// More details on LROs are available at: https://google.aip.dev/151
*/
package lro //import "go.alis.build/lro"
