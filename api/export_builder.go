package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"velarix/core"
	"velarix/store"

	"github.com/jung-kurt/gofpdf"
)

func buildSessionExport(sessionID string, orgID string, engine *core.Engine, history []store.JournalEntry, orgUsage map[string]uint64, journalChainHead string, format string) (contentType string, filename string, data []byte, err error) {
	var records [][]string
	for _, entry := range history {
		factID := entry.FactID
		if factID == "" && entry.Fact != nil {
			factID = entry.Fact.ID
		}

		confidence := "1.0"
		if entry.Fact != nil {
			confidence = fmt.Sprintf("%.2f", entry.Fact.ManualStatus)
		}

		records = append(records, []string{
			time.UnixMilli(entry.Timestamp).Format(time.RFC3339),
			string(entry.Type),
			sessionID,
			orgID,
			entry.ActorID,
			factID,
			confidence,
			fmt.Sprintf("%.2f", engine.GetStatus(factID)),
		})
	}

	h := sha256.New()
	for _, row := range records {
		h.Write([]byte(strings.Join(row, ",")))
	}
	hashSum := hex.EncodeToString(h.Sum(nil))

	switch format {
	case "csv", "":
		buf := &bytes.Buffer{}
		writer := csv.NewWriter(buf)
		_ = writer.Write([]string{"VERIFICATION_HASH", hashSum})
		if journalChainHead != "" {
			_ = writer.Write([]string{"JOURNAL_CHAIN_HEAD", journalChainHead})
		}
		_ = writer.Write([]string{"timestamp", "event_type", "session_id", "org_id", "actor_id", "fact_id", "confidence", "current_status"})
		_ = writer.WriteAll(records)
		writer.Flush()
		return "text/csv", fmt.Sprintf("velarix_audit_%s.csv", sessionID), buf.Bytes(), nil
	case "pdf":
		usage := orgUsage
		if usage == nil {
			usage = map[string]uint64{}
		}
		pdf := gofpdf.New("P", "mm", "A4", "")
		pdf.AddPage()
		pdf.SetFont("Courier", "B", 8)
		pdf.Cell(0, 10, "Verification Hash: "+hashSum)
		if journalChainHead != "" {
			pdf.Ln(4)
			pdf.Cell(0, 10, "Journal Chain Head: "+journalChainHead)
		}
		pdf.Ln(10)
		pdf.SetFont("Arial", "B", 16)
		pdf.Cell(0, 10, "Velarix Verification Export")
		pdf.Ln(12)
		pdf.SetFont("Arial", "", 10)
		pdf.Cell(0, 10, fmt.Sprintf("Total Facts: %d | API Requests: %d", usage["facts_asserted"], usage["api_requests"]))
		pdf.Ln(12)
		for _, row := range records {
			pdf.Cell(0, 5, fmt.Sprintf("[%s] %s | Actor: %s | Fact: %s | Status: %s", row[0], row[1], row[4], row[5], row[7]))
			pdf.Ln(4)
			if pdf.GetY() > 270 {
				pdf.AddPage()
			}
		}
		out := &bytes.Buffer{}
		if err := pdf.Output(out); err != nil {
			return "", "", nil, err
		}
		return "application/pdf", fmt.Sprintf("velarix_audit_%s.pdf", sessionID), out.Bytes(), nil
	default:
		return "", "", nil, fmt.Errorf("unsupported export format: %s", format)
	}
}
