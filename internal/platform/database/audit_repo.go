package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// AgentAuditLog represents a single row from the agent_audit_log table.
type AgentAuditLog struct {
	ID         string          `json:"id"`
	AgentType  string          `json:"agent_type"`
	Action     string          `json:"action"`
	InputData  json.RawMessage `json:"input_data"`
	OutputData json.RawMessage `json:"output_data"`
	Confidence *float64        `json:"confidence"`
	NodeID     *string         `json:"node_id"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ConfidencePct devuelve la confianza como porcentaje (0–100), o -1 si es NULL.
func (a AgentAuditLog) ConfidencePct() int {
	if a.Confidence == nil {
		return -1
	}
	return int(*a.Confidence * 100)
}

// InputDataStr devuelve el JSON de input formateado, o "—" si es nulo.
func (a AgentAuditLog) InputDataStr() string {
	if len(a.InputData) == 0 || string(a.InputData) == "null" {
		return "—"
	}
	var pretty interface{}
	if err := json.Unmarshal(a.InputData, &pretty); err != nil {
		return string(a.InputData)
	}
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return string(a.InputData)
	}
	return string(out)
}

// OutputDataStr devuelve el JSON de output formateado, o "—" si es nulo.
func (a AgentAuditLog) OutputDataStr() string {
	if len(a.OutputData) == 0 || string(a.OutputData) == "null" {
		return "—"
	}
	var pretty interface{}
	if err := json.Unmarshal(a.OutputData, &pretty); err != nil {
		return string(a.OutputData)
	}
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return string(a.OutputData)
	}
	return string(out)
}

// ListAuditLogs recupera los últimos `limit` registros del log de auditoría,
// ordenados por fecha de creación descendente.
func ListAuditLogs(db *sql.DB, limit int) ([]AgentAuditLog, error) {
	query := `
		SELECT id, agent_type, action,
		       COALESCE(input_data, 'null'::jsonb),
		       COALESCE(output_data, 'null'::jsonb),
		       confidence,
		       node_id::text,
		       created_at
		FROM agent_audit_log
		ORDER BY created_at DESC
		LIMIT $1`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []AgentAuditLog
	for rows.Next() {
		var entry AgentAuditLog
		var inputRaw, outputRaw []byte
		if err := rows.Scan(
			&entry.ID,
			&entry.AgentType,
			&entry.Action,
			&inputRaw,
			&outputRaw,
			&entry.Confidence,
			&entry.NodeID,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan audit log row: %w", err)
		}
		entry.InputData = json.RawMessage(inputRaw)
		entry.OutputData = json.RawMessage(outputRaw)
		logs = append(logs, entry)
	}
	return logs, rows.Err()
}
