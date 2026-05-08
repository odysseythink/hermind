// storage/sqlite/interface_test.go
package sqlite

import "github.com/odysseythink/hermind/storage"

// Compile-time assertion that *Store satisfies storage.Storage.
// If any method is missing or has the wrong signature, this fails to compile.
var _ storage.Storage = (*Store)(nil)
