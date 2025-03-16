/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package engine

import (
	"fmt"
	"strings"
)

type ChartValues map[string]any

// Table gets a table (YAML subsection) from a Values object.
//
// The table is returned as a Values.
//
// Compound table names may be specified with dots:
//
//	foo.bar
//
// The above will be evaluated as "The table bar inside the table
// foo".
//
// An ErrNoTable is returned if the table does not exist.
func (v ChartValues) Table(name string) (ChartValues, error) {
	table := v
	var err error

	for _, n := range parsePath(name) {
		if table, err = tableLookup(table, n); err != nil {
			break
		}
	}
	return table, err
}

func parsePath(key string) []string { return strings.Split(key, ".") }

func tableLookup(v ChartValues, simple string) (ChartValues, error) {
	v2, ok := v[simple]
	if !ok {
		return v, ErrNoTable{simple}
	}
	if vv, ok := v2.(map[string]interface{}); ok {
		return vv, nil
	}

	// This catches a case where a value is of type Values, but doesn't (for some
	// reason) match the map[string]interface{}. This has been observed in the
	// wild, and might be a result of a nil map of type Values.
	if vv, ok := v2.(ChartValues); ok {
		return vv, nil
	}

	return ChartValues{}, ErrNoTable{simple}
}

// ErrNoTable indicates that a chart does not have a matching table.
type ErrNoTable struct {
	Key string
}

func (e ErrNoTable) Error() string { return fmt.Sprintf("%q is not a table", e.Key) }
