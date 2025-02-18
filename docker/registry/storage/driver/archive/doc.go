// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// This package contains an implementation of the registry.StorageDriver from the
// distribution project that uses an archive as the storage backend. This is
// used in mindthegap serve|import|push commands as a read-only storage driver
// negating the need to extract the archive to a directory and thus saving disk
// space and time.
package archive
