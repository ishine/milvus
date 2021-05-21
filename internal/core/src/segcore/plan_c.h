// Copyright (C) 2019-2020 Zilliz. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License
// is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
// or implied. See the License for the specific language governing permissions and limitations under the License

#ifdef __cplusplus
extern "C" {
#endif

#include <stdbool.h>
#include <stdint.h>
#include "segcore/collection_c.h"
#include "common/type_c.h"

typedef void* CPlan;
typedef void* CPlaceholderGroup;

CStatus
CreatePlan(CCollection col, const char* dsl, CPlan* res_plan);

// Note: serialized_expr_plan is of binary format
CStatus
CreatePlanByExpr(CCollection col, const char* serialized_expr_plan, int64_t size, CPlan* res_plan);

CStatus
ParsePlaceholderGroup(CPlan plan,
                      void* placeholder_group_blob,
                      int64_t blob_size,
                      CPlaceholderGroup* res_placeholder_group);

int64_t
GetNumOfQueries(CPlaceholderGroup placeholder_group);

int64_t
GetTopK(CPlan plan);

const char*
GetMetricType(CPlan plan);

void
DeletePlan(CPlan plan);

void
DeletePlaceholderGroup(CPlaceholderGroup placeholder_group);

#ifdef __cplusplus
}
#endif
