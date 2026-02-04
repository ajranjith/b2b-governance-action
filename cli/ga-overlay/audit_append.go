package main

import "os"

func auditAppend() (*os.File, error) {
  return os.OpenFile("audit.log", os.O_APPEND|0x40|os.O_WRONLY, 0o644)
}
