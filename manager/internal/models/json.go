package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Scan implements sql.Scanner for JSONMap
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONMap)
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to unmarshal JSONB value: %T %v", value, value)
	}
	if len(bytes) == 0 {
		*j = make(JSONMap)
		return nil
	}
	// Accept both objects {} and arrays []
	var obj JSONMap
	if err := json.Unmarshal(bytes, &obj); err == nil {
		*j = obj
		return nil
	}
	var arr []interface{}
	if err := json.Unmarshal(bytes, &arr); err == nil {
		*j = JSONMap{"_arr": arr}
		return nil
	}
	return fmt.Errorf("failed to unmarshal JSONB value: %s", string(bytes))
}

// Value implements driver.Valuer for JSONMap
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(j)
}
