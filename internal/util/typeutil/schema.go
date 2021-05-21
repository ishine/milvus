// Copyright (C) 2019-2020 Zilliz. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License
// is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
// or implied. See the License for the specific language governing permissions and limitations under the License.

package typeutil

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/milvus-io/milvus/internal/proto/schemapb"
)

func EstimateSizePerRecord(schema *schemapb.CollectionSchema) (int, error) {
	res := 0
	for _, fs := range schema.Fields {
		switch fs.DataType {
		case schemapb.DataType_Bool, schemapb.DataType_Int8:
			res++
		case schemapb.DataType_Int16:
			res += 2
		case schemapb.DataType_Int32, schemapb.DataType_Float:
			res += 4
		case schemapb.DataType_Int64, schemapb.DataType_Double:
			res += 8
		case schemapb.DataType_String:
			res += 125 // todo find a better way to estimate string type
		case schemapb.DataType_BinaryVector:
			for _, kv := range fs.TypeParams {
				if kv.Key == "dim" {
					v, err := strconv.Atoi(kv.Value)
					if err != nil {
						return -1, err
					}
					res += v / 8
					break
				}
			}
		case schemapb.DataType_FloatVector:
			for _, kv := range fs.TypeParams {
				if kv.Key == "dim" {
					v, err := strconv.Atoi(kv.Value)
					if err != nil {
						return -1, err
					}
					res += v * 4
					break
				}
			}
		}
	}
	return res, nil
}

type SchemaHelper struct {
	schema     *schemapb.CollectionSchema
	nameOffset map[string]int
	idOffset   map[int64]int
}

func CreateSchemaHelper(schema *schemapb.CollectionSchema) (*SchemaHelper, error) {
	if schema == nil {
		return nil, errors.New("schema is nil")
	}
	schemaHelper := SchemaHelper{schema: schema, nameOffset: make(map[string]int), idOffset: make(map[int64]int)}
	for offset, field := range schema.Fields {
		if _, ok := schemaHelper.nameOffset[field.Name]; ok {
			return nil, errors.New("duplicated fieldName: " + field.Name)
		}
		if _, ok := schemaHelper.idOffset[field.FieldID]; ok {
			return nil, errors.New("duplicated fieldID: " + strconv.FormatInt(field.FieldID, 10))
		}
		schemaHelper.nameOffset[field.Name] = offset
		schemaHelper.idOffset[field.FieldID] = offset
	}
	return &schemaHelper, nil
}

func (helper *SchemaHelper) GetFieldFromName(fieldName string) (*schemapb.FieldSchema, error) {
	offset, ok := helper.nameOffset[fieldName]
	if !ok {
		return nil, fmt.Errorf("fieldName(%s) not found", fieldName)
	}
	return helper.schema.Fields[offset], nil
}

func (helper *SchemaHelper) GetFieldFromID(fieldID int64) (*schemapb.FieldSchema, error) {
	offset, ok := helper.idOffset[fieldID]
	if !ok {
		return nil, fmt.Errorf("fieldName(%d) not found", fieldID)
	}
	return helper.schema.Fields[offset], nil
}

func IsVectorType(dataType schemapb.DataType) bool {
	switch dataType {
	case schemapb.DataType_FloatVector, schemapb.DataType_BinaryVector:
		return true
	default:
		return false
	}
}

func IsIntergerType(dataType schemapb.DataType) bool {
	switch dataType {
	case schemapb.DataType_Int8, schemapb.DataType_Int16,
		schemapb.DataType_Int32, schemapb.DataType_Int64:
		return true
	default:
		return false
	}
}

func IsFloatingType(dataType schemapb.DataType) bool {
	switch dataType {
	case schemapb.DataType_Float, schemapb.DataType_Double:
		return true
	default:
		return false
	}
}
