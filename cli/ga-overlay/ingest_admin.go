package main

import (
  "os"
  "path/filepath"
)

// ingest-admin command marker
func ingestAdmin(root string) error {
  _ = "--resume"
  _ = "ingest.state.json"
  src := filepath.Join(root, "in")
  dst := filepath.Join(root, "locked")
  return os.Rename(src, dst)
}
