package manage

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/minicago/gooj/sql_service"
)

// UserCSVImportRow describes one user import row.
type UserCSVImportRow struct {
	Username  string
	GroupName string
	Password  string
}

// ParseUserCSVImport parses a CSV payload with headers: username,group,password.
func ParseUserCSVImport(data []byte) ([]UserCSVImportRow, error) {
	// Strip UTF-8 BOM if present
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	r := csv.NewReader(bytes.NewReader(data))
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1

	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, errors.New("csv must include header and at least one user row")
	}

	columns := make(map[string]int)
	for idx, header := range records[0] {
		columns[strings.ToLower(strings.TrimSpace(header))] = idx
	}
	for _, key := range []string{"username", "group", "password"} {
		if _, ok := columns[key]; !ok {
			return nil, fmt.Errorf("missing required csv column: %s", key)
		}
	}

	rows := make([]UserCSVImportRow, 0, len(records)-1)
	for rowIdx := 1; rowIdx < len(records); rowIdx++ {
		record := records[rowIdx]
		if len(record) == 0 || strings.TrimSpace(strings.Join(record, "")) == "" {
			continue
		}
		if len(record) <= columns["username"] || len(record) <= columns["group"] || len(record) <= columns["password"] {
			return nil, fmt.Errorf("row %d is malformed", rowIdx+1)
		}
		user := UserCSVImportRow{
			Username:  strings.TrimSpace(record[columns["username"]]),
			GroupName: strings.TrimSpace(record[columns["group"]]),
			Password:  strings.TrimSpace(record[columns["password"]]),
		}
		if user.Username == "" || user.GroupName == "" || user.Password == "" {
			return nil, fmt.Errorf("row %d has empty required field", rowIdx+1)
		}
		rows = append(rows, user)
	}
	if len(rows) == 0 {
		return nil, errors.New("csv has no user rows")
	}
	return rows, nil
}

// ImportUsersCSVHandler imports users from a submitted CSV file.
func ImportUsersCSVHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("csv")
	if err != nil {
		http.Error(w, "csv file required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read csv", http.StatusBadRequest)
		return
	}

	rows, err := ParseUserCSVImport(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	creator := CurrentUsername(r)
	if creator == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	imported := 0
	var lastErr error
	for _, row := range rows {
		if err := sql_service.CreateUserWithGroup(row.Username, row.Password, row.GroupName, creator); err != nil {
			lastErr = err
			break
		}
		imported++
	}
	if imported == 0 {
		http.Error(w, fmt.Sprintf("failed to import any users: %v", lastErr), http.StatusInternalServerError)
		return
	}
	if imported != len(rows) {
		http.Error(w, fmt.Sprintf("import partially failed after %d rows: %v", imported, lastErr), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "imported": imported})
}
