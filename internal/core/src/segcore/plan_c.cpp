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

#include "segcore/plan_c.h"
#include "query/Plan.h"
#include "segcore/Collection.h"

CStatus
CreatePlan(CCollection c_col, const char* dsl, CPlan* res_plan) {
    auto col = (milvus::segcore::Collection*)c_col;

    try {
        auto res = milvus::query::CreatePlan(*col->get_schema(), dsl);

        auto status = CStatus();
        status.error_code = Success;
        status.error_msg = "";
        auto plan = (CPlan)res.release();
        *res_plan = plan;
        return status;
    } catch (milvus::SegcoreError& e) {
        auto status = CStatus();
        status.error_code = e.get_error_code();
        status.error_msg = strdup(e.what());
        *res_plan = nullptr;
        return status;
    } catch (std::exception& e) {
        auto status = CStatus();
        status.error_code = UnexpectedError;
        status.error_msg = strdup(e.what());
        *res_plan = nullptr;
        return status;
    }
}

// Note: serialized_expr_plan is of binary format
CStatus
CreatePlanByExpr(CCollection c_col, const char* serialized_expr_plan, int64_t size, CPlan* res_plan) {
    auto col = (milvus::segcore::Collection*)c_col;

    try {
        auto res = milvus::query::CreatePlanByExpr(*col->get_schema(), serialized_expr_plan, size);

        auto status = CStatus();
        status.error_code = Success;
        status.error_msg = "";
        auto plan = (CPlan)res.release();
        *res_plan = plan;
        return status;
    } catch (milvus::SegcoreError& e) {
        auto status = CStatus();
        status.error_code = e.get_error_code();
        status.error_msg = strdup(e.what());
        *res_plan = nullptr;
        return status;
    } catch (std::exception& e) {
        auto status = CStatus();
        status.error_code = UnexpectedError;
        status.error_msg = strdup(e.what());
        *res_plan = nullptr;
        return status;
    }
}

CStatus
ParsePlaceholderGroup(CPlan c_plan,
                      void* placeholder_group_blob,
                      int64_t blob_size,
                      CPlaceholderGroup* res_placeholder_group) {
    std::string blob_string((char*)placeholder_group_blob, (char*)placeholder_group_blob + blob_size);
    auto plan = (milvus::query::Plan*)c_plan;

    try {
        auto res = milvus::query::ParsePlaceholderGroup(plan, blob_string);

        auto status = CStatus();
        status.error_code = Success;
        status.error_msg = "";
        auto group = (CPlaceholderGroup)res.release();
        *res_placeholder_group = group;
        return status;
    } catch (std::exception& e) {
        auto status = CStatus();
        status.error_code = UnexpectedError;
        status.error_msg = strdup(e.what());
        *res_placeholder_group = nullptr;
        return status;
    }
}

int64_t
GetNumOfQueries(CPlaceholderGroup placeholder_group) {
    auto res = milvus::query::GetNumOfQueries((milvus::query::PlaceholderGroup*)placeholder_group);

    return res;
}

int64_t
GetTopK(CPlan plan) {
    auto res = milvus::query::GetTopK((milvus::query::Plan*)plan);

    return res;
}

const char*
GetMetricType(CPlan plan) {
    auto query_plan = static_cast<milvus::query::Plan*>(plan);
    auto metric_str = milvus::MetricTypeToName(query_plan->plan_node_->query_info_.metric_type_);
    return strdup(metric_str.c_str());
}

void
DeletePlan(CPlan cPlan) {
    auto plan = (milvus::query::Plan*)cPlan;
    delete plan;
    // std::cout << "delete plan" << std::endl;
}

void
DeletePlaceholderGroup(CPlaceholderGroup cPlaceholder_group) {
    auto placeHolder_group = (milvus::query::PlaceholderGroup*)cPlaceholder_group;
    delete placeHolder_group;
    // std::cout << "delete placeholder" << std::endl;
}
