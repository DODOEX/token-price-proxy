package schema

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONMap is a custom type for handling map[string]string JSON fields in GORM
type JSONMap map[string]string

// Value implements the driver.Valuer interface
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

// Scan implements the sql.Scanner interface
func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(map[string]string)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("unsupported Scan, storing driver.Value type %T into type *JSONMap", value)
	}

	return json.Unmarshal(bytes, m)
}
