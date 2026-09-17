package handlers

import (
	"errors"
	"strings"

	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

var hostSortFields = map[string]string{"id": "ID", "name": "Name", "status": "Status", "ip_address": "IPAddress", "cpu_cores": "CPUCores", "memory_mb": "MemoryMB", "created_at": "CreatedAt", "updated_at": "UpdatedAt"}
var vmSortFields = map[string]string{"id": "ID", "name": "Name", "namespace": "Namespace", "status": "Status", "vcpus": "VCPUs", "memory_mb": "MemoryMB", "host_node": "HostNode", "ip_address": "IPAddress", "created_at": "CreatedAt", "updated_at": "UpdatedAt"}

// Only known model fields become quoted SQL identifiers; client input is never SQL.
func validatedSort(field, direction string, allowed map[string]string) (clause.OrderByColumn, error) {
	if field == "" {
		field = "name"
	}
	desc := strings.HasPrefix(field, "-")
	if desc {
		field = strings.TrimPrefix(field, "-")
	}
	modelField, ok := allowed[field]
	if !ok {
		return clause.OrderByColumn{}, errors.New("unsupported sort field")
	}
	if direction != "" {
		switch strings.ToLower(direction) {
		case "asc":
			if desc {
				return clause.OrderByColumn{}, errors.New("conflicting sort direction")
			}
		case "desc":
			desc = true
		default:
			return clause.OrderByColumn{}, errors.New("unsupported sort direction")
		}
	}
	return clause.OrderByColumn{Column: clause.Column{Name: (schema.NamingStrategy{}).ColumnName("", modelField)}, Desc: desc}, nil
}
