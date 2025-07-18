/*
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package fhirclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// ResolveRef can be used to resolve references in a resource being read.
// The path is the path to the reference, and the target is the resource to resolve the reference into.
// E.g., to ServiceRequest's subject, the path would be "subject" and the target could e.g., be a Patient or Group.
// It returns an error when the path does not contain a reference, when the reference cannot be resolved,
// or when the resolved reference cannot be unmarshaled into the target.
func ResolveRef(path string, target any) PostParseOption {
	return func(client Client, resource any) error {
		err := resolveReference(client, path, resource, target)
		if err != nil {
			return fmt.Errorf("resolve reference: %w", err)
		}
		return nil
	}
}

func resolveReference(client Client, path string, resource any, result any) error {
	// Sanity checks
	resultPtrType := reflect.TypeOf(result)
	if resultPtrType.Kind() != reflect.Ptr {
		return errors.New("result must be a pointer")
	}
	resultType := resultPtrType.Elem()

	// TODO: Support lists, absolute URLs
	asMap, err := toMap(resource)
	if err != nil {
		return err
	}
	refRaw, ok := asMap[path]
	if !ok {
		// TODO: Set to nil instead
		return fmt.Errorf("path not found: %s", path)
	}
	if resultType.Kind() == reflect.Slice {
		// Need to result list of references
		refs, ok := refRaw.([]interface{})
		if !ok {
			return fmt.Errorf("not a list of FHIR References at path: %s", path)
		}
		for _, ref := range refs {
			sliceEntry := reflect.New(resultType.Elem())
			err := doResolve(client, path, ref, sliceEntry.Interface())
			if err != nil {
				return err
			}
			// Add to result array
			resultValue := reflect.ValueOf(result).Elem()
			resultValue.Set(reflect.Append(resultValue, sliceEntry.Elem()))
		}
		return nil
	}
	return doResolve(client, path, refRaw, result)
}

func doResolve(client Client, path string, refRaw interface{}, result any) error {
	refMap, ok := refRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("not a FHIR Reference at path: %s", path)
	}
	if _, ok := refMap["identifier"]; ok {
		return fmt.Errorf("unexpected FHIR Reference.identifier (not supported) at path: %s", path)
	}
	ref, ok := refMap["reference"].(string)
	if !ok {
		return fmt.Errorf("FHIR Reference.reference missing/invalid at path: %s", path)
	}
	return client.Read(ref, result)
}

type ReferenceResolver struct {
	client Client
}

func isSlice(v interface{}) bool {
	return reflect.TypeOf(v).Kind() == reflect.Slice
}

func toMap(v any) (map[string]interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	err = json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}
