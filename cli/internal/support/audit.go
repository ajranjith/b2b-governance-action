package support

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type AuditEntry struct {
	TimestampUtc   string `json:"timestampUtc"`
	Mode           string `json:"mode"`
	RedCount       int    `json:"redCount"`
	AmberCount     int    `json:"amberCount"`
	GreenCount     int    `json:"greenCount"`
	Phase1Status   string `json:"phase1Status,omitempty"`
	Phase2Status   string `json:"phase2Status,omitempty"`
	Phase3Status   string `json:"phase3Status,omitempty"`
	Phase4Status   string `json:"phase4Status,omitempty"`
	CertificateSHA string `json:"certificate_hash,omitempty"`
	DryRun         bool   `json:"dryRun,omitempty"`
	Actions        int    `json:"actions,omitempty"`
	Result         string `json:"result,omitempty"`
}

func AppendAudit(workspace string, entry AuditEntry) error {
	entry.TimestampUtc = time.Now().UTC().Format(time.RFC3339)
	path := filepath.Join(workspace, ".b2b", "audit.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}
